package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"manga-downloader/themes"
)

type ComiciParser struct{}

func NewComiciParser() themes.Parser {
	return &ComiciParser{}
}

func init() {
	Register(NewComiciParser())
}

func (p *ComiciParser) Name() string {
	return "Comici"
}

func (p *ComiciParser) Domains() []string {
	return []string{
		"rimacomiplus.jp",
		"younganimal.com",
		"takecomic.jp",
		"kimicomi.com",
		"youngchampion.jp",
		"mangalt.jp",
		"mangaspa.nikkan-spa.jp",
		"piacomic.jp",
		"comic-room-base.com",
		"manga-zegra.com",
		"comic-growl.com",
		"asacomi.jp",
		"comicpash.jp",
		"comic.j-nbooks.jp",
		"hayacomic.jp",
		"championcross.jp",
		"kansai.mag-garden.co.jp",
		"comicride.jp",
		"carula.jp",
		"comic-medu.com",
		"comics.manga-bang.com",
		"bigcomics.jp",
		"studio.booklista.co.jp",
		"hanayume.com",
		"bibibi-comic.com",
	}
}

type comiciEpisodeResponse struct {
	Result []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		PageCount int    `json:"page_count"`
	} `json:"result"`
}

type comiciContentsResponse struct {
	TotalPages int `json:"totalPages"`
	Result     []struct {
		Sort     int    `json:"sort"`
		ImageURL string `json:"imageUrl"`
		Scramble string `json:"scramble"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	} `json:"result"`
}

func (p *ComiciParser) GetImages(chapterURL string) ([]string, error) {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("URL tidak valid: %w", err)
	}

	doc, rawHTML, err := themes.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsed.Host),
	})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil HTML: %w", err)
	}

	viewerID := ""
	apiDomain := ""
	seriesHash := ""

	// 1. Ekstraksi dari DOM #comici-viewer
	viewerEl := doc.Find("#comici-viewer")
	if viewerEl.Length() > 0 {
		if id, ok := viewerEl.Attr("comici-viewer-id"); ok && id != "" {
			viewerID = id
		} else if id, ok := viewerEl.Attr("data-comici-viewer-id"); ok && id != "" {
			viewerID = id
		}
		if dom, ok := viewerEl.Attr("data-api-domain"); ok && dom != "" {
			apiDomain = dom
		}
	}

	// 2. Ekstraksi #xHeader
	if hash, ok := doc.Find("#xHeader").Attr("data-series-hash"); ok && hash != "" {
		seriesHash = hash
	}

	// 3. Fallback regex jika dirender via JS
	if viewerID == "" {
		reViewer := regexp.MustCompile(`(?:comici-viewer-id|data-comici-viewer-id)=["']([^"']+)["']`)
		if m := reViewer.FindStringSubmatch(rawHTML); len(m) > 1 {
			viewerID = m[1]
		}
	}
	if viewerID == "" {
		// Ambil episode ID dari path URL (misal: /episodes/58ec1e11d6d08)
		rePathID := regexp.MustCompile(`/episodes/([a-zA-Z0-9]+)`)
		if m := rePathID.FindStringSubmatch(parsed.Path); len(m) > 1 {
			viewerID = m[1]
		}
	}
	if seriesHash == "" {
		reHash := regexp.MustCompile(`data-series-hash=["']([^"']+)["']`)
		if m := reHash.FindStringSubmatch(rawHTML); len(m) > 1 {
			seriesHash = m[1]
		}
	}
	if apiDomain == "" {
		reDomain := regexp.MustCompile(`data-api-domain=["']([^"']+)["']`)
		if m := reDomain.FindStringSubmatch(rawHTML); len(m) > 1 {
			apiDomain = m[1]
		}
	}

	if viewerID == "" {
		return nil, fmt.Errorf("tidak dapat menemukan viewer-id untuk Comici pada halaman ini")
	}

	// Tentukan startURL API
	startURL := parsed.Host
	if apiDomain != "" {
		if strings.HasPrefix(apiDomain, "/") {
			startURL = parsed.Host + apiDomain
		} else if apiDomain == parsed.Host {
			startURL = parsed.Host
		} else {
			startURL = fmt.Sprintf("%s%s", parsed.Host, apiDomain)
		}
	}

	client := &http.Client{Timeout: 20 * time.Second}

	// Tentukan total halaman
	totalPages := 0
	if seriesHash != "" {
		checkURL := fmt.Sprintf("https://%s/book/contentsInfo?user-id=&comici-viewer-id=%s&page-from=0&page-to=1", startURL, viewerID)
		req, _ := http.NewRequest("GET", checkURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Referer", chapterURL)

		if resp, err := client.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var data comiciContentsResponse
			if body, err := io.ReadAll(resp.Body); err == nil {
				_ = json.Unmarshal(body, &data)
				totalPages = data.TotalPages
			}
		}
	}

	if totalPages <= 0 {
		epURL := fmt.Sprintf("https://%s/book/episodeInfo?comici-viewer-id=%s", startURL, viewerID)
		req, _ := http.NewRequest("GET", epURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Referer", chapterURL)

		if resp, err := client.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var data comiciEpisodeResponse
			if body, err := io.ReadAll(resp.Body); err == nil {
				_ = json.Unmarshal(body, &data)
				for _, item := range data.Result {
					if item.ID == viewerID || totalPages == 0 {
						totalPages = item.PageCount
					}
				}
			}
		}
	}

	if totalPages <= 0 {
		totalPages = 200
	}

	// Ambil semua halaman
	fetchURL := fmt.Sprintf("https://%s/book/contentsInfo?user-id=0&comici-viewer-id=%s&page-from=0&page-to=%d", startURL, viewerID, totalPages)
	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request halaman: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", chapterURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil contentsInfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contentsInfo HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respons contentsInfo: %w", err)
	}

	var contents comiciContentsResponse
	if err := json.Unmarshal(body, &contents); err != nil {
		return nil, fmt.Errorf("gagal parse JSON contentsInfo: %w", err)
	}

	var imageURLs []string
	for _, page := range contents.Result {
		if page.ImageURL == "" {
			continue
		}
		imgURL := page.ImageURL
		if page.Scramble != "" {
			imgURL = fmt.Sprintf("%s#scramble=%s", page.ImageURL, url.QueryEscape(page.Scramble))
		}
		imageURLs = append(imageURLs, imgURL)
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("tidak ditemukan gambar pada respons Comici")
	}

	return imageURLs, nil
}

// ProcessImage mendescramble puzzle potongan gambar Comici sebelum disimpan ke file
func (p *ComiciParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return themes.ProcessImageDescramble(rawBytes, imageURL)
}
