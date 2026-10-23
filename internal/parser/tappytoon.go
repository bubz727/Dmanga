package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var (
	tappyChapterIDRegex = regexp.MustCompile(`/(?:chapters|book/[^/]+)/(\d+)`)
	tappyNextDataRegex  = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">([^<]+)</script>`)
)

type Tappytoon struct{}

func NewTappytoon() *Tappytoon {
	return &Tappytoon{}
}

func (t *Tappytoon) Name() string {
	return "Tappytoon"
}

func (t *Tappytoon) Domains() []string {
	return []string{
		"tappytoon.com",
		"www.tappytoon.com",
	}
}

func (t *Tappytoon) GetImages(chapterURL string) ([]string, error) {
	chap, err := t.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (t *Tappytoon) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	// 1. Extract chapter ID
	chapterID := ""
	if m := tappyChapterIDRegex.FindStringSubmatch(parsedURL.Path); len(m) > 1 {
		chapterID = m[1]
	} else {
		parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
		if len(parts) > 0 {
			chapterID = parts[len(parts)-1]
		}
	}

	if chapterID == "" {
		return nil, core.ChapterIDNotFoundError(chapterURL)
	}

	// 2. Fetch chapter page HTML to extract __NEXT_DATA__
	doc, rawHTML, err := fetcher.FetchHTML(chapterURL, nil)
	if err != nil {
		return nil, err
	}

	baseURL := "https://api-global.tappytoon.com"
	reqHeaders := make(map[string]string)
	chapterTitle := ""

	// Extract title from HTML
	if doc != nil {
		rawTitle := strings.TrimSpace(doc.Find("head title").Text())
		rawTitle = strings.TrimPrefix(rawTitle, "Read ")
		if idx := strings.Index(rawTitle, "-"); idx != -1 {
			rawTitle = strings.TrimSpace(rawTitle[:idx])
		}
		if idx := strings.Index(rawTitle, "|"); idx != -1 {
			rawTitle = strings.TrimSpace(rawTitle[:idx])
		}
		chapterTitle = utils.NormalizeChapterTitle(rawTitle)
	}

	isSuperHigh := false
	hasSuperHighInfo := false

	// Extract axios headers, chapter title, and comic quality info from __NEXT_DATA__
	if m := tappyNextDataRegex.FindStringSubmatch(rawHTML); len(m) > 1 {
		var nextData map[string]interface{}
		if err := json.Unmarshal([]byte(m[1]), &nextData); err == nil {
			if props, ok := nextData["props"].(map[string]interface{}); ok {
				if initSt, ok := props["initialState"].(map[string]interface{}); ok {
					if axios, ok := initSt["axios"].(map[string]interface{}); ok {
						if b, ok := axios["baseURL"].(string); ok && b != "" {
							baseURL = b
						}
						if h, ok := axios["headers"].(map[string]interface{}); ok {
							for k, v := range h {
								if s, ok := v.(string); ok {
									reqHeaders[k] = s
								}
							}
						}
					}
					if entities, ok := initSt["entities"].(map[string]interface{}); ok {
						if chapters, ok := entities["chapters"].(map[string]interface{}); ok {
							if chObj, ok := chapters[chapterID].(map[string]interface{}); ok {
								if t, ok := chObj["title"].(string); ok && strings.TrimSpace(t) != "" {
									chapterTitle = utils.NormalizeChapterTitle(strings.TrimSpace(t))
								}
							}
						}
						if comics, ok := entities["comics"].(map[string]interface{}); ok {
							for _, v := range comics {
								if comicObj, ok := v.(map[string]interface{}); ok {
									if sh, ok := comicObj["isSuperHighQualitySupported"].(bool); ok {
										isSuperHigh = sh
										hasSuperHighInfo = true
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 3. Fetch manifest API for image files
	var variants []string
	if hasSuperHighInfo {
		if isSuperHigh {
			variants = []string{"super_high", "high"}
		} else {
			variants = []string{"high", "super_high"}
		}
	} else {
		variants = []string{"super_high", "high"}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var images []string

	for _, variant := range variants {
		if len(images) > 0 {
			break
		}
		manifestURL := fmt.Sprintf("%s/content-delivery/contents/manifest?chapterId=%s&canAcceptSignedCookie=true&variant=%s", baseURL, chapterID, variant)
		req, err := http.NewRequest("GET", manifestURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Origin", "https://www.tappytoon.com")
		req.Header.Set("Referer", "https://www.tappytoon.com/")
		for k, v := range reqHeaders {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Save any signed cookies if returned via Set-Cookie
			cookieMap := make(map[string]string)
			for _, c := range resp.Cookies() {
				cookieMap[c.Name] = c.Value
			}
			if len(cookieMap) > 0 {
				fetcher.SaveSession(parsedURL.Host, "", cookieMap)
			}

			var manifestResp struct {
				Files []struct {
					URL string `json:"url"`
				} `json:"files"`
			}
			if err := json.Unmarshal(bodyBytes, &manifestResp); err == nil {
				for _, f := range manifestResp.Files {
					u := strings.TrimSpace(f.URL)
					if u != "" {
						images = append(images, u)
						if len(cookieMap) > 0 {
							if pu, err := url.Parse(u); err == nil {
								fetcher.SaveSession(pu.Host, "", cookieMap)
							}
						}
					}
				}
			}
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	// Fallback to /contents API if manifest is empty
	if len(images) == 0 {
		contentAPI := fmt.Sprintf("%s/content-delivery/contents?chapterId=%s&variant=original&locale=en", baseURL, chapterID)
		req2, err := http.NewRequest("GET", contentAPI, nil)
		if err == nil {
			req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			for k, v := range reqHeaders {
				req2.Header.Set(k, v)
			}
			resp2, err := client.Do(req2)
			if err == nil && resp2.StatusCode == http.StatusOK {
				defer resp2.Body.Close()
				bBytes, _ := io.ReadAll(resp2.Body)
				var contentResp struct {
					Files []struct {
						URL string `json:"url"`
					} `json:"files"`
					Images []string `json:"images"`
				}
				if err := json.Unmarshal(bBytes, &contentResp); err == nil {
					for _, f := range contentResp.Files {
						if f.URL != "" {
							images = append(images, f.URL)
						}
					}
					images = append(images, contentResp.Images...)
				}
			} else if resp2 != nil {
				resp2.Body.Close()
			}
		}
	}

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func init() {
	Register(NewTappytoon())
}
