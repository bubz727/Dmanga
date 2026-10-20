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

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

type MangaBall struct{}

func NewMangaBall() *MangaBall {
	return &MangaBall{}
}

func (m *MangaBall) Name() string {
	return "MangaBall"
}

func (m *MangaBall) Domains() []string {
	return []string{
		"mangaball.com",
		"mangaball.net",
	}
}

var (
	mangaBallObjectIDRegex    = regexp.MustCompile(`^[a-fA-F0-9]{24}$`)
	mangaBallAnyObjectIDRegex = regexp.MustCompile(`[a-fA-F0-9]{24}`)
)

// ExtractMangaBallChapterID extracts the 24-hex MongoDB ObjectId from a MangaBall URL
func ExtractMangaBallChapterID(chapterURL string) (string, error) {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return "", err
	}

	cleanPath := strings.Trim(parsed.Path, "/")
	parts := strings.Split(cleanPath, "/")

	// If the user pasted a title/manga details page instead of a chapter
	for _, p := range parts {
		if p == "title-detail" || p == "title" {
			return "", fmt.Errorf("URL is a title/series page, please provide a chapter URL (e.g. https://mangaball.com/chapter-detail/<id>): %s", chapterURL)
		}
	}

	// 1. Check query parameters: chapter_id or id
	if id := parsed.Query().Get("chapter_id"); mangaBallObjectIDRegex.MatchString(id) {
		return id, nil
	}
	if id := parsed.Query().Get("id"); mangaBallObjectIDRegex.MatchString(id) {
		return id, nil
	}

	// 2. Check path segments directly following "chapter" or "chapter-detail"
	for i, p := range parts {
		if (p == "chapter" || p == "chapter-detail") && i+1 < len(parts) {
			seg := parts[i+1]
			if mangaBallObjectIDRegex.MatchString(seg) {
				return seg, nil
			}
			if id := mangaBallAnyObjectIDRegex.FindString(seg); id != "" {
				return id, nil
			}
		}
	}

	// 3. Fallback: Search from the end of path segments for any 24-hex ObjectId (e.g. slug-id)
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if mangaBallObjectIDRegex.MatchString(seg) {
			return seg, nil
		}
		if id := mangaBallAnyObjectIDRegex.FindString(seg); id != "" {
			return id, nil
		}
	}

	return "", core.ChapterIDNotFoundError(chapterURL)
}

type MangaBallResponse struct {
	Code int `json:"code"`
	Data struct {
		Chapter struct {
			Number        json.Number `json:"number"`
			ChapterNumber json.Number `json:"chapter_number"`
			Name          string      `json:"name"`
			ChapterName   string      `json:"chapter_name"`
			Title         string      `json:"title"`
			TitleName     string      `json:"title_name"`
			Pages         []string    `json:"pages"`
		} `json:"chapter"`
	} `json:"data"`
}

// FormatMangaBallChapterTitle formats chapter title into "Ch <number>"
func FormatMangaBallChapterTitle(res *MangaBallResponse, chapterURL string) string {
	numStr := ""
	if res != nil {
		if s := res.Data.Chapter.ChapterNumber.String(); s != "" {
			numStr = s
		} else if s := res.Data.Chapter.Number.String(); s != "" {
			numStr = s
		}
	}

	if numStr != "" {
		return fmt.Sprintf("Ch %s", numStr)
	}

	if res != nil {
		rawTitle := strings.TrimSpace(res.Data.Chapter.ChapterName)
		if rawTitle == "" {
			rawTitle = strings.TrimSpace(res.Data.Chapter.Name)
		}
		if rawTitle == "" {
			rawTitle = strings.TrimSpace(res.Data.Chapter.Title)
		}

		if rawTitle != "" {
			if num := utils.ExtractChapterNumber(rawTitle); num != "" {
				return fmt.Sprintf("Ch %s", num)
			}
			if _, err := strconv.ParseFloat(rawTitle, 64); err == nil {
				return fmt.Sprintf("Ch %s", rawTitle)
			}
			return utils.NormalizeChapterTitle(rawTitle)
		}
	}

	if num := utils.ExtractChapterNumber(chapterURL); num != "" {
		return fmt.Sprintf("Ch %s", num)
	}

	return ""
}

// GetChapter extracts both the chapter title and image URLs
func (m *MangaBall) GetChapter(chapterURL string) (*core.Chapter, error) {
	chapterID, err := ExtractMangaBallChapterID(chapterURL)
	if err != nil {
		return nil, err
	}

	baseDomain := "mangaball.com"
	apiURL := fmt.Sprintf("https://%s/api/v1/chapter-detail?chapter_id=%s", baseDomain, chapterID)
	client := &http.Client{Timeout: 30 * time.Second}

	var bodyBytes []byte
	maxAttempts := 2

	for attempt := 0; attempt < maxAttempts; attempt++ {
		fetcher.CheckRateLimitCooldown()

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
		if sess := fetcher.GetSession(baseDomain); sess != nil && sess.UserAgent != "" {
			ua = sess.UserAgent
		}

		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", baseDomain))
		req.Header.Set("Origin", fmt.Sprintf("https://%s", baseDomain))
		fetcher.ApplySessionHeaders(req, baseDomain)

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxAttempts-1 {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, err
		}

		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt < maxAttempts-1 {
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, err
		}

		// Cloudflare Challenge check
		if fetcher.IsCloudflareChallenge(resp.StatusCode, string(b), resp.Header) {
			if attempt == 0 && !fetcher.HasAttemptedSolve(baseDomain) {
				fmt.Printf("[!] MangaBall API blocked by Cloudflare (HTTP %d). Solving challenge...\n", resp.StatusCode)
				_, solveErr := fetcher.SolveCloudflare(chapterURL)
				if solveErr != nil {
					// Fallback to solving on base domain if chapterURL failed
					_, solveErr = fetcher.SolveCloudflare(fmt.Sprintf("https://%s/", baseDomain))
					if solveErr != nil {
						return nil, fmt.Errorf("Cloudflare challenge bypass failed for MangaBall: %w", solveErr)
					}
				}
				continue
			}
			return nil, fmt.Errorf("Cloudflare challenge blocked access to MangaBall (HTTP %d)", resp.StatusCode)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, core.HTTPStatusError(resp.StatusCode, resp.Status)
		}

		bodyBytes = b
		break
	}

	var res MangaBallResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, core.JSONParseError("MangaBall", err)
	}

	chapterTitle := FormatMangaBallChapterTitle(&res, chapterURL)

	var images []string
	for _, raw := range res.Data.Chapter.Pages {
		clean := utils.CleanImageURL(raw, baseDomain)
		if clean != "" {
			images = append(images, clean)
		}
	}

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (m *MangaBall) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func init() {
	Register(NewMangaBall())
}
