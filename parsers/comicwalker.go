package parsers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"manga-downloader/core"
)

var (
	cwEpisodeRegex = regexp.MustCompile(`/episodes/([A-Za-z0-9_-]+)`)
	cwDetailRegex  = regexp.MustCompile(`/detail/([A-Za-z0-9_-]+)`)
)

type ComicWalker struct{}

func NewComicWalker() *ComicWalker {
	return &ComicWalker{}
}

func (c *ComicWalker) Name() string {
	return "Comic-Walker"
}

func (c *ComicWalker) Domains() []string {
	return []string{
		"comic-walker.com",
		"www.comic-walker.com",
	}
}

func (c *ComicWalker) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}

	episodeID := parsedURL.Query().Get("episodeId")
	if episodeID == "" {
		// 1. Check if URL contains /episodes/{episodeCode}
		if epMatches := cwEpisodeRegex.FindStringSubmatch(parsedURL.Path); len(epMatches) > 1 {
			episodeCode := epMatches[1]
			epID, err := c.fetchEpisodeIDByCode(client, episodeCode)
			if err == nil && epID != "" {
				episodeID = epID
			}
		}

		// 2. Check if URL is a detail/series page: /detail/{workCode}
		if episodeID == "" {
			if detailMatches := cwDetailRegex.FindStringSubmatch(parsedURL.Path); len(detailMatches) > 1 {
				workCode := detailMatches[1]
				epType := strings.ToLower(parsedURL.Query().Get("episodeType"))
				epID, err := c.fetchEpisodeIDByWork(client, workCode, epType)
				if err == nil && epID != "" {
					episodeID = epID
				}
			}
		}
	}

	if episodeID == "" {
		return nil, fmt.Errorf("could not resolve episode ID from URL: %s", chapterURL)
	}

	// 3. Fetch Viewer Manifest
	viewerURL := fmt.Sprintf("https://comic-walker.com/api/contents/viewer?episodeId=%s&imageSizeType=width%%3A1284", url.QueryEscape(episodeID))
	req, err := http.NewRequest("GET", viewerURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://comic-walker.com/")
	core.ApplySessionHeaders(req, "comic-walker.com")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("viewer API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, core.HTTPStatusError(resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var viewerResp struct {
		Manuscripts []struct {
			DrmMode     string `json:"drmMode"`
			DrmHash     string `json:"drmHash"`
			DrmImageUrl string `json:"drmImageUrl"`
		} `json:"manuscripts"`
	}

	if err := json.Unmarshal(bodyBytes, &viewerResp); err != nil {
		return nil, core.JSONParseError("viewer", err)
	}

	if len(viewerResp.Manuscripts) == 0 {
		return nil, core.ErrNoImagesFound
	}

	var images []string
	for _, m := range viewerResp.Manuscripts {
		imgURL := m.DrmImageUrl
		if imgURL == "" {
			continue
		}

		if strings.EqualFold(m.DrmMode, "xor") && m.DrmHash != "" {
			imgURL = fmt.Sprintf("%s#drmHash=%s", imgURL, m.DrmHash)
		}
		images = append(images, imgURL)
	}

	return core.EnsureImages(images)
}

// ProcessImage implements core.ImageProcessor to decrypt XOR-scrambled WebP images
func (c *ComicWalker) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	idx := strings.Index(imageURL, "#drmHash=")
	if idx == -1 {
		return rawBytes, nil
	}

	drmHash := imageURL[idx+len("#drmHash="):]
	if amp := strings.Index(drmHash, "&"); amp != -1 {
		drmHash = drmHash[:amp]
	}

	if len(drmHash) < 16 {
		return rawBytes, nil
	}

	// First 16 hex chars = 8 bytes key
	keyHex := drmHash[:16]
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) == 0 {
		return rawBytes, nil
	}

	keyLen := len(key)
	dec := make([]byte, len(rawBytes))
	for i := 0; i < len(rawBytes); i++ {
		dec[i] = rawBytes[i] ^ key[i%keyLen]
	}

	return dec, nil
}

func (c *ComicWalker) fetchEpisodeIDByCode(client *http.Client, episodeCode string) (string, error) {
	apiURL := fmt.Sprintf("https://comic-walker.com/api/contents/details/episode?episodeCode=%s&workCode=0&episodeType=first", url.QueryEscape(episodeCode))
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://comic-walker.com/")
	core.ApplySessionHeaders(req, "comic-walker.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var data struct {
		Episode struct {
			ID string `json:"id"`
		} `json:"episode"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	return data.Episode.ID, nil
}

func (c *ComicWalker) fetchEpisodeIDByWork(client *http.Client, workCode, epType string) (string, error) {
	apiURL := fmt.Sprintf("https://comic-walker.com/api/contents/details/work?workCode=%s", url.QueryEscape(workCode))
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://comic-walker.com/")
	core.ApplySessionHeaders(req, "comic-walker.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var data struct {
		LatestEpisodeID string `json:"latestEpisodeId"`
		FirstEpisodes   struct {
			Result []struct {
				ID string `json:"id"`
			} `json:"result"`
		} `json:"firstEpisodes"`
		LatestEpisodes struct {
			Result []struct {
				ID string `json:"id"`
			} `json:"result"`
		} `json:"latestEpisodes"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	if epType == "latest" {
		if data.LatestEpisodeID != "" {
			return data.LatestEpisodeID, nil
		}
		if len(data.LatestEpisodes.Result) > 0 {
			return data.LatestEpisodes.Result[0].ID, nil
		}
	}

	// Default: first episode
	if len(data.FirstEpisodes.Result) > 0 {
		return data.FirstEpisodes.Result[0].ID, nil
	}
	if len(data.LatestEpisodes.Result) > 0 {
		return data.LatestEpisodes.Result[0].ID, nil
	}

	return "", fmt.Errorf("no episodes found for work code: %s", workCode)
}

func init() {
	core.Register(NewComicWalker())
}
