package parsers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/themes"
)

// Porting langsung dari VrfSigner di Kotatsu mangafire.kt
const (
	mfTable1B64 = "yINlmUNho8VYJT+ibTIP+9ESiULpVEtMOoD6U6lRE0R/xwXo/Xp9NrUgC4cw/Lmo33vUyjUE40kUoEWIr/fxfNNcq2s79ShQ5NhNrFnJ4hXPwOu/SuXzIbuTQKGFvfm08E9jvCfqAtoDqvQq3dVWPQFmJjgvkISBeXY3BgANR+yVnjGbcxZ47d6kLNfZPIayTq3/YGySb1KuVZodWp/WGNAO5pfMcpaK53Hhs0allBszaMaxuouOwdxbwgxIw6YunSsXjI05Yi0j9j4eHKfSXR8Ifo/Od+8iamRfCXTyvm7NGRGYdcQ0ywcK/u6RXhrbcCm4t2eCtrDgQVecJGkQ+A=="
	mfKey1B64   = "0Ec58JOY3uBzJK9m3zqIOpdlF7UFiax9DmA="
	mfTable2B64 = "IUFltCxD3Oc2cwCgkJffthaOg9cgPUb0LgW6H/VtfcF0kc5F25t+aWj6JH9VOhOaY0rAFdUxlDnl5BLNvwEJvQtP5qcw7vdb/K+chnbwnspSHT8mz5lqwz41TezG0hkO06FTjJZhsyNuFLDpD2ZZxQj/QIRcF90zpmQ7Byu483WsQqUE0C342HL+JXngRB6fRzxRyVTaKu83h7UYTJ0QMt6ixFh6S3F8gqkKwrGTL3jHNBsD45UnifK8+RGtishQV2K3rujLKEkiZxpr2dYcudFW4oFsDKhad3CLBvuyTqsCo4B7mL5IKQ1vXo/MOOvq1I1d8ar9X6Ttu5KF4fZgiA=="
	mfKey2B64   = "AAdjb1iPY8CiDmq9H34tKTBF8a3oDQ=="
	mfTable3B64 = "NQHlu1/wVO5EmkwQymF810qqY2xG1k2obcas4Z9mCsPEIFl9pRIjFxbJ7ybMHbBckT5Ton85E0FOeHezbh/mjlEYpmpnlXOS8dgrqeq2KfxImTh1YK9y0PeMNhzA1OQzSY9brYOJq/l2QnE/hwOeZIhPixVSKIUlDb5vLcH6RWKxkIEMuP0bDwIqQ71AJJaEaMJL7A6YtyIwoRT+L5v4aZzodN/0+3nOGsfblFjgxSfPzVDjNFeNl5P26+kEC/8AHgdrpAbt3hHz3HrRN1Y6e+JHgF7ncFWnoF0y3THL1S71WgWGCa6KtSzTCCG58n68nTyj2T3Sshk7utqCtMi/ZQ=="
	mfKey3B64   = "DELOJgPsVaCcblDtTGMdHzM="
)

type vrfStage struct {
	table []byte
	key   []byte
	iv    int
}

var vrfStages []vrfStage

func init() {
	t1, _ := base64.StdEncoding.DecodeString(mfTable1B64)
	k1, _ := base64.StdEncoding.DecodeString(mfKey1B64)
	t2, _ := base64.StdEncoding.DecodeString(mfTable2B64)
	k2, _ := base64.StdEncoding.DecodeString(mfKey2B64)
	t3, _ := base64.StdEncoding.DecodeString(mfTable3B64)
	k3, _ := base64.StdEncoding.DecodeString(mfKey3B64)

	vrfStages = []vrfStage{
		{table: t1, key: k1, iv: 0x5A},
		{table: t2, key: k2, iv: 0x35},
		{table: t3, key: k3, iv: 0xBA},
	}
}

func signVrf(path string) string {
	data := []byte(path)
	for _, stage := range vrfStages {
		data = encryptVrfStage(data, stage.table, stage.key, stage.iv)
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func encryptVrfStage(data, table, key []byte, iv int) []byte {
	out := make([]byte, len(data))
	prev := iv
	keySize := len(key)
	for i := range data {
		prev = int(table[(int(data[i])^int(key[i%keySize])^prev)&0xFF]) & 0xFF
		out[i] = byte(prev)
	}
	return out
}

type MangaFire struct{}

func NewMangaFire() *MangaFire {
	return &MangaFire{}
}

func (m *MangaFire) Name() string {
	return "MangaFire"
}

func (m *MangaFire) Domains() []string {
	return []string{
		"mangafire.to",
	}
}

var chapterIDRegex = regexp.MustCompile(`(?:chapter_id|current_chapter_id|id)\s*[:=]\s*["']?(\d+)["']?`)

func (m *MangaFire) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	// Ambil chapter ID dari URL atau dari HTML halaman reader
	chapterID := ""
	cleanPath := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(cleanPath, "/")

	// Cek apakah URL langsung berupa /title/.../12345
	if len(parts) >= 3 && parts[0] == "title" {
		chapterID = parts[len(parts)-1]
	}

	// Jika URL berupa /read/... kita fetch halamannya untuk cari data-id
	if chapterID == "" || strings.HasPrefix(cleanPath, "read/") {
		doc, rawHTML, err := themes.FetchHTML(chapterURL, map[string]string{
			"Referer": fmt.Sprintf("https://%s/", baseHost),
		})
		if err == nil {
			// Cari di data-id reader
			doc.Find("#reader, [data-id], .read-container").Each(func(i int, s *goquery.Selection) {
				if id, exists := s.Attr("data-id"); exists && id != "" && chapterID == "" {
					chapterID = id
				}
			})

			// Cari di script data
			if chapterID == "" {
				if match := chapterIDRegex.FindStringSubmatch(rawHTML); len(match) > 1 {
					chapterID = match[1]
				}
			}
		}
	}

	if chapterID == "" {
		// Fallback: gunakan bagian terakhir URL
		chapterID = parts[len(parts)-1]
	}

	// Bangun query ke /api/chapters/{chapterId} dengan tanda tangan VRF
	apiPath := fmt.Sprintf("/chapters/%s", chapterID)
	vrf := signVrf(apiPath)

	apiURL := fmt.Sprintf("https://%s/api%s?vrf=%s", baseHost, apiPath, url.QueryEscape(vrf))

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", baseHost))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type MangaFireResponse struct {
		Data struct {
			Pages []struct {
				URL string `json:"url"`
			} `json:"pages"`
		} `json:"data"`
	}

	var res MangaFireResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("gagal parse JSON MangaFire: %v", err)
	}

	var images []string
	for _, page := range res.Data.Pages {
		clean := themes.CleanImageURL(page.URL, baseHost)
		if clean != "" {
			images = append(images, clean)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan untuk MangaFire")
	}

	return images, nil
}

func init() {
	Register(NewMangaFire())
}
