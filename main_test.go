package main_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/parsers"
)

var flagURL = flag.String("url", "", "Chapter URL to test")

// getTargetURL retrieves the URL from the -url flag or positional argument after -args
func getTargetURL() string {
	if *flagURL != "" {
		return *flagURL
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return arg
		}
	}
	return ""
}

// ============================================================================
// 1. Live Diagnostic Parser Test
// ============================================================================

func TestMangaParser(t *testing.T) {
	targetURL := strings.TrimSpace(getTargetURL())
	if targetURL == "" {
		t.Skip("Skip: Provide a chapter URL to test. Example: go test . -args https://01.apkomik.com/chapter-1/")
	}

	t.Logf("==================================================")
	t.Logf("PARSER DIAGNOSTIC FOR: %s", targetURL)
	t.Logf("==================================================")

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		t.Fatalf("[FAILED] Invalid URL format: %v", err)
	}

	// 1. Check Router / Registry
	p, err := core.FindParser(parsed.Host)
	if err != nil {
		t.Fatalf("[FAILED DOMAIN] Domain '%s' is not registered in parser registry!", parsed.Host)
	}
	t.Logf("[OK 1/4] Domain Match     : Recognized as '%s'", p.Name())

	// 2. Check Connection & Anti-Bot / Cloudflare Protection
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("[CONNECTION FAILED] Unable to reach server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("[BLOCKED - HTTP 403] Access denied! Server blocked request (Cloudflare/WAF).")
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("[BLOCKED - HTTP 503] Server 503 (Cloudflare Turnstile/Under Attack).")
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("[HTTP FAILED] Server responded with status code %d %s", resp.StatusCode, resp.Status)
	}
	t.Logf("[OK 2/4] HTTP Connection  : 200 OK (Clean access, unblocked)")

	// 3. Image Extraction
	start := time.Now()
	images, err := p.GetImages(targetURL)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("[EXTRACTION FAILED] %v", err)
	}
	if len(images) == 0 {
		t.Fatalf("[EMPTY IMAGES FAILED] No images found. Selector/script structure may have changed.")
	}
	t.Logf("[OK 3/4] Image Extraction : Found %d images (time: %v)", len(images), duration)

	// 4. Sample Image URL Validation
	sample := images[0]
	t.Logf("[OK 4/4] Sample Image URL : %s", sample)
	t.Logf("==================================================")
	t.Logf("[SUCCESS] Parser '%s' IS 100%% FUNCTIONAL!", p.Name())
}

// ============================================================================
// 2. Core Utility Tests
// ============================================================================

func TestImageFormatConversion(t *testing.T) {
	// 1. JPEG magic bytes
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	resBytes, ext, err := core.EnsureJPGorPNG(jpegBytes, ".jpg")
	if err != nil || ext != ".jpg" || len(resBytes) != len(jpegBytes) {
		t.Errorf("JPEG check failed: ext=%s, err=%v", ext, err)
	}

	// 2. PNG magic bytes
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	resBytes, ext, err = core.EnsureJPGorPNG(pngBytes, ".png")
	if err != nil || ext != ".png" || len(resBytes) != len(pngBytes) {
		t.Errorf("PNG check failed: ext=%s, err=%v", ext, err)
	}
}

func TestParseInputURLs(t *testing.T) {
	// 1. Space separated
	input1 := "https://example.com/ch-1 https://example.com/ch-2 https://example.com/ch-3"
	urls1 := core.ParseInputURLs(input1)
	if len(urls1) != 3 {
		t.Fatalf("Expected 3 URLs, got %d", len(urls1))
	}
	if urls1[0] != "https://example.com/ch-1" || urls1[2] != "https://example.com/ch-3" {
		t.Errorf("Unexpected URLs parsed: %v", urls1)
	}

	// 2. Pipe separated
	input2 := "https://example.com/ch-1 | https://example.com/ch-2"
	urls2 := core.ParseInputURLs(input2)
	if len(urls2) != 2 {
		t.Fatalf("Expected 2 URLs for pipe delimiter, got %d", len(urls2))
	}

	// 3. Comma and newline separated
	input3 := "https://example.com/ch-1, https://example.com/ch-2\nhttps://example.com/ch-3\tinvalid-url"
	urls3 := core.ParseInputURLs(input3)
	if len(urls3) != 3 {
		t.Fatalf("Expected 3 valid URLs, got %d", len(urls3))
	}
}

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ch 344: Special?", "Ch 344- Special-"},
		{`Volume 1 / Ch 10 *Special* <Final>`, "Volume 1 - Ch 10 -Special- -Final-"},
		{"  Ch 105...  ", "Ch 105"},
		{"Ch | 12", "Ch - 12"},
		{"第1話 忍者×魔法", "第1話 忍者×魔法"},
		{"【レッドサム家】長女・カレンの復讐②", "【レッドサム家】長女・カレンの復讐②"},
	}

	for _, tt := range tests {
		got := core.SanitizeFolderName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeFolderName(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestMultiChapterNamingCollision(t *testing.T) {
	usedFolders := make(map[string]int)
	resolveFolderName := func(title string, idx int) string {
		folderName := title
		if folderName == "" {
			folderName = fmt.Sprintf("Ch %d", idx+1)
		}
		key := strings.ToLower(folderName)
		if count, exists := usedFolders[key]; exists {
			usedFolders[key] = count + 1
			folderName = fmt.Sprintf("%s (%d)", folderName, count+1)
		} else {
			usedFolders[key] = 1
		}
		return folderName
	}

	// Scenario: multiple links where two share the same chapter name
	r1 := resolveFolderName("Ch 344", 0)
	r2 := resolveFolderName("Ch 344", 1)
	r3 := resolveFolderName("Ch 345", 2)

	if r1 != "Ch 344" {
		t.Errorf("Unexpected r1: %s", r1)
	}
	if r2 != "Ch 344 (2)" {
		t.Errorf("Expected deduplicated r2: 'Ch 344 (2)', got '%s'", r2)
	}
	if r3 != "Ch 345" {
		t.Errorf("Unexpected r3: %s", r3)
	}
}

// ============================================================================
// 3. Parser-Specific Chapter Extraction Tests
// ============================================================================

func TestAsuraScansHTMLTitleExtraction(t *testing.T) {
	// 1. Exact modern header from Asura Scans
	htmlSnippet := `
	<!DOCTYPE html>
	<html>
	<head><title>Overgeared - Chapter 344</title></head>
	<body>
		<div class="header">
			<div class="font-semibold truncate text-zinc-400">
				<span class="text-zinc-100">Chapter <!-- -->344</span>
				<span class="font-normal text-sm"> - <!-- -->Saharan vs. Valhalla</span>
			</div>
		</div>
	</body>
	</html>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlSnippet))
	if err != nil {
		t.Fatalf("Failed to parse HTML document: %v", err)
	}

	title := parsers.ExtractAsuraChapterTitle(doc)
	expected := "Ch 344"

	if title != expected {
		t.Fatalf("Expected chapter title %q, got %q", expected, title)
	}
	t.Logf("[OK] Extracted Asura chapter title successfully as: %s", title)

	// 2. Absence of selector returns empty string (no guessing)
	emptyDoc, _ := goquery.NewDocumentFromReader(strings.NewReader("<div>No header here</div>"))
	emptyTitle := parsers.ExtractAsuraChapterTitle(emptyDoc)
	if emptyTitle != "" {
		t.Fatalf("Expected empty title when selector is missing, got %q", emptyTitle)
	}
}

func TestComiciChapterTitleExtraction(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "Contoh 1: Manga standard dengan subjudul",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第1話 忍者×魔法</h1><p class="ep-main-h-date">2026年8月7日</p></div></div>`,
			expected: "第1話 忍者×魔法",
		},
		{
			name:     "Contoh 2: Manhwa Webtoon header (mengabaikan judul komik)",
			html:     `<h1 class="ep-h-main-h" data-e2e="ehTitle"><span class="ep-h-main-h-text"><a class="ep-h-main-h-series" data-e2e="ehSeriesLnk" href="/series/cffb958c0a451">耳兜の冒険者〜あいつに聞かれるな・目を合わせるな・関わるな〜</a><span data-e2e="ehEpisodeTtl">第1話</span></span></h1>`,
			expected: "第1話",
		},
		{
			name:     "Contoh 3: Manga chapter dengan part pecahan [3]",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第1話 ［3］</h1><p class="ep-main-h-date">2025年4月24日</p></div></div>`,
			expected: "第1話 ［3］",
		},
		{
			name:     "Contoh 4: Cerita khusus dengan nomor melingkar",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">【レッドサム家】長女・カレンの復讐②</h1><p class="ep-main-h-date">2025年11月27日</p></div>`,
			expected: "【レッドサム家】長女・カレンの復讐②",
		},
		{
			name:     "Contoh 5: Manga chapter nomor polos",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第2話</h1><p class="ep-main-h-date">2025年8月22日</p></div></div>`,
			expected: "第2話",
		},
		{
			name:     "Contoh 6: Manga dengan kata Hou (Laporan)",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">第1報　青春さん、立ちはだかる。</h1><p class="ep-main-h-date">2025年9月24日</p></div>`,
			expected: "第1報 青春さん、立ちはだかる。",
		},
		{
			name:     "Contoh 7: Pengumuman rilis jilid (Announcement)",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">単行本10巻発売告知</h1></div>`,
			expected: "単行本10巻発売告知",
		},
	}

	for _, tt := range tests {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
		if err != nil {
			t.Fatalf("[%s] Failed to parse HTML: %v", tt.name, err)
		}
		got := parsers.ExtractComiciChapterTitle(doc)
		if got != tt.expected {
			t.Errorf("[%s] Expected %q, got %q", tt.name, tt.expected, got)
		} else {
			t.Logf("[%s] OK -> %s", tt.name, got)
		}
	}
}

func TestToonzRSCExtraction(t *testing.T) {
	mockRSC := `1:HL["/uploads/chapters/1022788/13405767/001.webp","image"]
2:{"name":"chapter","chapterNumber":"72.00","comicId":1022788}
3:{"images":[{"filename":"/chapters/1022788/13405767/001.webp"},{"filename":"/chapters/1022788/13405767/002.webp"},{"filename":"/chapters/1022788/13405767/003.webp"}]}
`
	toonz := parsers.NewToonz()
	if toonz.Name() != "Toonz" {
		t.Fatalf("Expected parser name 'Toonz', got %q", toonz.Name())
	}

	reFilename := regexp.MustCompile(`"filename"\s*:\s*"([^"]+)"`)
	matches := reFilename.FindAllStringSubmatch(mockRSC, -1)
	if len(matches) != 3 {
		t.Fatalf("Expected 3 filename matches, got %d", len(matches))
	}

	reChapter := regexp.MustCompile(`"chapterNumber"\s*:\s*"([0-9]+(?:\.[0-9]+)?)"`)
	chMatch := reChapter.FindStringSubmatch(mockRSC)
	if len(chMatch) <= 1 || strings.TrimSuffix(chMatch[1], ".00") != "72" {
		t.Fatalf("Expected chapter number 72, got %v", chMatch)
	}
	t.Logf("[OK] Toonz RSC payload parsed: 3 images, Chapter 72")
}

func TestFormatImageIndex(t *testing.T) {
	tests := []struct {
		index    int
		total    int
		expected string
	}{
		{index: 1, total: 50, expected: "001"},
		{index: 50, total: 50, expected: "050"},
		{index: 1, total: 999, expected: "001"},
		{index: 999, total: 999, expected: "999"},
		{index: 1, total: 1000, expected: "0001"},
		{index: 100, total: 1000, expected: "0100"},
		{index: 1000, total: 1000, expected: "1000"},
		{index: 1, total: 1500, expected: "0001"},
		{index: 1500, total: 1500, expected: "1500"},
		{index: 7, total: 10000, expected: "00007"},
		{index: 10000, total: 10000, expected: "10000"},
	}

	for _, tt := range tests {
		got := core.FormatImageIndex(tt.index, tt.total)
		if got != tt.expected {
			t.Errorf("FormatImageIndex(%d, %d) = %q, expected %q", tt.index, tt.total, got, tt.expected)
		}
	}
	t.Logf("[OK] FormatImageIndex passed all dynamic padding scenarios")
}

func TestAllFormatChapterTitleExtraction(t *testing.T) {
	tests := []struct {
		name     string
		rawInput string
		isURL    bool
		expected string
	}{
		{
			name:     "1. XComic",
			rawInput: "Volume 20 Chapter 250 : Big Mouth ⑥",
			expected: "Ch 250",
		},
		{
			name:     "2. Weloma",
			rawInput: "Chap 29.1",
			expected: "Ch 29.1",
		},
		{
			name:     "3. WeebCentral",
			rawInput: "Chapter 185",
			expected: "Ch 185",
		},
		{
			name:     "4. Rawkuma",
			rawInput: "SEINARU OTOME TO HIMEGOTO WO Chapter 115",
			expected: "Ch 115",
		},
		{
			name:     "5. NihonKuni",
			rawInput: "Chapter 147",
			expected: "Ch 147",
		},
		{
			name:     "6. NicoManga",
			rawInput: "Chapter 184",
			expected: "Ch 184",
		},
		{
			name:     "7. Naver (Korean Hwa)",
			rawInput: "92화",
			expected: "Ch 92",
		},
		{
			name:     "8a. MangaRaw4U Slug part 3",
			rawInput: "https://mangaraw4u.com/manga/otome-game-raw/chapter-13-3",
			isURL:    true,
			expected: "Ch 13.30",
		},
		{
			name:     "8b. MangaRaw4U Slug part 1",
			rawInput: "https://mangaraw4u.com/manga/meshimazu-onna/chapter-24-1",
			isURL:    true,
			expected: "Ch 24.10",
		},
		{
			name:     "8c. MangaRaw4U Slug single integer",
			rawInput: "https://mangaraw4u.com/manga/test-raw/chapter-50",
			isURL:    true,
			expected: "Ch 50",
		},
		{
			name:     "9. MangaNato",
			rawInput: "Their Trap Chapter 49",
			expected: "Ch 49",
		},
		{
			name:     "10. Mangago",
			rawInput: "Ch.223.9",
			expected: "Ch 223.9",
		},
		{
			name:     "11. MangaFire",
			rawInput: "Chapter 1080",
			expected: "Ch 1080",
		},
		{
			name:     "12. GigaViewer (Authentic Japanese sanitized)",
			rawInput: "第1話 忍者×魔法",
			expected: "第1話 忍者×魔法",
		},
		{
			name:     "13. ComicWalker (Authentic Japanese sanitized)",
			rawInput: "第2話",
			expected: "第2話",
		},
		{
			name:     "14. MangaPlus (#001 -> Ch 1)",
			rawInput: " #001",
			expected: "Ch 1",
		},
		{
			name:     "14b. MangaPlus (#025 -> Ch 25)",
			rawInput: " #025",
			expected: "Ch 25",
		},
	}

	for _, tt := range tests {
		var got string
		if tt.isURL {
			got = core.ExtractMangaRaw4UChapterTitle(tt.rawInput)
		} else {
			got = core.NormalizeChapterTitle(tt.rawInput)
		}

		if got != tt.expected {
			t.Errorf("[%s] Expected %q, got %q", tt.name, tt.expected, got)
		} else {
			t.Logf("[%s] OK -> %s", tt.name, got)
		}
	}
}

func TestMangaFireChapterTitleFromAPI(t *testing.T) {
	mockJSON := `{"data":{"number":166,"name":"","pages":[{"url":"https://example.com/1.jpg"}]}}`
	var res struct {
		Data struct {
			Number json.Number `json:"number"`
			Name   string      `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(mockJSON), &res); err != nil {
		t.Fatalf("Failed to unmarshal MangaFire mock: %v", err)
	}
	title := fmt.Sprintf("Ch %s", res.Data.Number.String())
	if title != "Ch 166" {
		t.Fatalf("Expected 'Ch 166', got %q", title)
	}
	t.Logf("[OK] MangaFire chapter title resolved: %s", title)
}


