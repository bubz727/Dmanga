package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var (
	reWebtoonSlugEpisode = regexp.MustCompile(`(?i)(?:episode|ep|ch|chapter)-(\d+)/viewer`)
	reWebtoonTrailingNum = regexp.MustCompile(`-(\d+)/viewer`)
)

type Webtoon struct {
	client *http.Client
}

func NewWebtoon() *Webtoon {
	return &Webtoon{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (w *Webtoon) Name() string {
	return "Webtoon"
}

func (w *Webtoon) Domains() []string {
	return []string{
		"webtoons.com",
		"www.webtoons.com",
		"m.webtoons.com",
	}
}

func (w *Webtoon) GetImages(chapterURL string) ([]string, error) {
	chap, err := w.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (w *Webtoon) GetChapter(chapterURL string) (*core.Chapter, error) {
	chapterURL = w.ensureEpisodeNo(chapterURL)

	doc, _, err := fetcher.FetchHTML(chapterURL, map[string]string{
		"Referer": "https://www.webtoons.com/",
		"Cookie":  "needCCPA=false; needGDPR=false; needCOPPA=false; pagAgeGate=true",
	})
	if err != nil {
		return nil, err
	}

	// 1. Extract Chapter Title
	rawTitle := strings.TrimSpace(doc.Find("h1.subj_episode, .subj_episode").First().Text())
	if rawTitle == "" {
		if t := doc.Find("head title").Text(); t != "" {
			if idx := strings.Index(t, "|"); idx != -1 {
				rawTitle = strings.TrimSpace(t[:idx])
			} else {
				rawTitle = strings.TrimSpace(t)
			}
		}
	}
	if rawTitle == "" {
		rawTitle = strings.TrimSpace(doc.Find("head meta[property='og:title']").AttrOr("content", ""))
	}
	chapterTitle := utils.NormalizeChapterTitle(rawTitle)

	// 2. Extract Images
	var images []string
	seen := make(map[string]bool)

	doc.Find("#_imageList img, .viewer_lst img, div.viewer_img img, img._images").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("data-url")
		if !exists || strings.TrimSpace(src) == "" {
			src, exists = s.Attr("src")
		}
		src = strings.TrimSpace(src)
		if src == "" || strings.Contains(src, "banner") || strings.Contains(src, "logo") || strings.Contains(src, "bg_transparency") {
			return
		}

		// Remove `type` query param (e.g. ?type=q90) for full quality
		if parsed, err := url.Parse(src); err == nil {
			q := parsed.Query()
			if q.Has("type") {
				q.Del("type")
				parsed.RawQuery = q.Encode()
				src = parsed.String()
			}
		}

		if !seen[src] {
			seen[src] = true
			images = append(images, src)
		}
	})

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

// ensureEpisodeNo recovers episode_no when unquoted '&episode_no=...' is stripped by Windows CMD
func (w *Webtoon) ensureEpisodeNo(chapterURL string) string {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return chapterURL
	}

	q := parsed.Query()
	if q.Get("episode_no") != "" {
		return chapterURL
	}

	// 1. Try matching against Webtoon mobile episode API if title_no is present
	titleNo := q.Get("title_no")
	if titleNo != "" {
		pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		lang := "en"
		kind := "webtoon"
		if len(pathParts) >= 2 {
			lang = pathParts[0]
			if pathParts[1] == "canvas" || pathParts[1] == "challenge" {
				kind = "canvas"
			}
		}
		apiURL := fmt.Sprintf("https://m.webtoons.com/api/v1/%s/%s/episodes?pageSize=99999", kind, titleNo)
		if kind == "canvas" {
			apiURL += "&readingLanguageCode=" + lang
		}
		if req, err := http.NewRequest("GET", apiURL, nil); err == nil {
			fetcher.ApplyDefaultBrowserHeaders(req, "m.webtoons.com")
			req.Header.Set("Referer", "https://m.webtoons.com/")
			if resp, err := w.client.Do(req); err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					if body, err := io.ReadAll(resp.Body); err == nil {
						var apiResp struct {
							Result struct {
								EpisodeList []struct {
									ViewerLink string `json:"viewerLink"`
									EpisodeNo  int    `json:"episodeNo"`
								} `json:"episodeList"`
							} `json:"result"`
						}
						if json.Unmarshal(body, &apiResp) == nil {
							for _, ep := range apiResp.Result.EpisodeList {
								if strings.Contains(ep.ViewerLink, parsed.Path) && ep.EpisodeNo > 0 {
									q.Set("episode_no", strconv.Itoa(ep.EpisodeNo))
									parsed.RawQuery = q.Encode()
									return parsed.String()
								}
							}
						}
					}
				}
			}
		}
	}

	// 2. Fallback: extract episode number from the URL path (e.g. /episode-1/viewer)
	if m := reWebtoonSlugEpisode.FindStringSubmatch(parsed.Path); len(m) > 1 {
		q.Set("episode_no", m[1])
		parsed.RawQuery = q.Encode()
		return parsed.String()
	}
	if m := reWebtoonTrailingNum.FindStringSubmatch(parsed.Path); len(m) > 1 {
		q.Set("episode_no", m[1])
		parsed.RawQuery = q.Encode()
		return parsed.String()
	}

	return chapterURL
}

func init() {
	Register(NewWebtoon())
}
