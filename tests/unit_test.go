package tests

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/cli"
	"dmanga/internal/downloader"
	"dmanga/internal/image"
	"dmanga/internal/image/descramble"
	"dmanga/internal/parser"
	"dmanga/internal/utils"
)

func TestImageFormatConversion(t *testing.T) {
	// 1. JPEG magic bytes
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	resBytes, ext, err := image.EnsureJPGorPNG(jpegBytes, ".jpg")
	if err != nil || ext != ".jpg" || len(resBytes) != len(jpegBytes) {
		t.Errorf("JPEG check failed: ext=%s, err=%v", ext, err)
	}

	// 2. PNG magic bytes
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	resBytes, ext, err = image.EnsureJPGorPNG(pngBytes, ".png")
	if err != nil || ext != ".png" || len(resBytes) != len(pngBytes) {
		t.Errorf("PNG check failed: ext=%s, err=%v", ext, err)
	}
}

func TestParseInputURLs(t *testing.T) {
	// 1. Space separated
	input1 := "https://example.com/ch-1 https://example.com/ch-2 https://example.com/ch-3"
	urls1 := cli.ParseInputURLs(input1)
	if len(urls1) != 3 {
		t.Fatalf("Expected 3 URLs, got %d", len(urls1))
	}
	if urls1[0] != "https://example.com/ch-1" || urls1[2] != "https://example.com/ch-3" {
		t.Errorf("Unexpected URLs parsed: %v", urls1)
	}

	// 2. Pipe separated
	input2 := "https://example.com/ch-1 | https://example.com/ch-2"
	urls2 := cli.ParseInputURLs(input2)
	if len(urls2) != 2 {
		t.Fatalf("Expected 2 URLs for pipe delimiter, got %d", len(urls2))
	}

	// 3. Comma and newline separated
	input3 := "https://example.com/ch-1, https://example.com/ch-2\nhttps://example.com/ch-3\tinvalid-url"
	urls3 := cli.ParseInputURLs(input3)
	if len(urls3) != 3 {
		t.Fatalf("Expected 3 valid URLs, got %d", len(urls3))
	}

	// 4. Input with -f flag
	input4 := "-f https://weebcentral.com/chapters/01M5DR493EHYY89H1S8B8GY0EG"
	urls4 := cli.ParseInputURLs(input4)
	if len(urls4) != 1 || urls4[0] != "https://weebcentral.com/chapters/01M5DR493EHYY89H1S8B8GY0EG" {
		t.Fatalf("Expected 1 URL without flag, got %v", urls4)
	}
}

func TestHasFolderFlag(t *testing.T) {
	if !cli.HasFolderFlag("-f", "https://example.com/ch-1") {
		t.Errorf("Expected HasFolderFlag to be true for '-f'")
	}
	if !cli.HasFolderFlag("https://example.com/ch-1", "-f") {
		t.Errorf("Expected HasFolderFlag to be true for trailing '-f'")
	}
	if !cli.HasFolderFlag("--folder", "https://example.com/ch-1") {
		t.Errorf("Expected HasFolderFlag to be true for '--folder'")
	}
	if !cli.HasFolderFlag("-f https://example.com/ch-1") {
		t.Errorf("Expected HasFolderFlag to be true for combined string")
	}
	if cli.HasFolderFlag("https://example.com/ch-1") {
		t.Errorf("Expected HasFolderFlag to be false without flag")
	}
	if cli.HasFolderFlag("https://example.com/ch-f/1") {
		t.Errorf("Expected HasFolderFlag to be false for URL containing 'ch-f'")
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
		got := utils.SanitizeFolderName(tt.input)
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

func TestAsuraScansHTMLTitleExtraction(t *testing.T) {
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

	title := parser.ExtractAsuraChapterTitle(doc)
	expected := "Ch 344"

	if title != expected {
		t.Fatalf("Expected chapter title %q, got %q", expected, title)
	}
	t.Logf("[OK] Extracted Asura chapter title successfully as: %s", title)

	emptyDoc, _ := goquery.NewDocumentFromReader(strings.NewReader("<div>No header here</div>"))
	emptyTitle := parser.ExtractAsuraChapterTitle(emptyDoc)
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
			name:     "Example 1: Standard manga with subtitle",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第1話 忍者×魔法</h1><p class="ep-main-h-date">2026年8月7日</p></div></div>`,
			expected: "第1話 忍者×魔法",
		},
		{
			name:     "Example 2: Webtoon header",
			html:     `<h1 class="ep-h-main-h" data-e2e="ehTitle"><span class="ep-h-main-h-text"><a class="ep-h-main-h-series" data-e2e="ehSeriesLnk" href="/series/cffb958c0a451">耳兜の冒険者〜あいつに聞かれるな・目を合わせるな・関わるな〜</a><span data-e2e="ehEpisodeTtl">第1話</span></span></h1>`,
			expected: "第1話",
		},
		{
			name:     "Example 3: Chapter with part [3]",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第1話 ［3］</h1><p class="ep-main-h-date">2025年4月24日</p></div></div>`,
			expected: "第1話 ［3］",
		},
		{
			name:     "Example 4: Special chapter",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">【レッドサム家】長女・カレンの復讐②</h1><p class="ep-main-h-date">2025年11月27日</p></div>`,
			expected: "【レッドサム家】長女・カレンの復讐②",
		},
		{
			name:     "Example 5: Plain number chapter",
			html:     `<div class="ep-main-h"><div class="ep-main-h-main"><h1 class="ep-main-h-h">第2話</h1><p class="ep-main-h-date">2025年8月22日</p></div></div>`,
			expected: "第2話",
		},
		{
			name:     "Example 6: Report chapter",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">第1報　青春さん、立ちはだかる。</h1><p class="ep-main-h-date">2025年9月24日</p></div>`,
			expected: "第1報 青春さん、立ちはだかる。",
		},
		{
			name:     "Example 7: Tankoubon announcement",
			html:     `<div class="ep-main-h-main"><h1 class="ep-main-h-h">単行本10巻発売告知</h1></div>`,
			expected: "単行本10巻発売告知",
		},
	}

	for _, tt := range tests {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
		if err != nil {
			t.Fatalf("[%s] Failed to parse HTML: %v", tt.name, err)
		}
		got := parser.ExtractComiciChapterTitle(doc)
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
	toonz := parser.NewToonz()
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
		got := utils.FormatImageIndex(tt.index, tt.total)
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
			got = utils.ExtractMangaRaw4UChapterTitle(tt.rawInput)
		} else {
			got = utils.NormalizeChapterTitle(tt.rawInput)
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

func TestMangaBallChapterIDExtraction(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedID  string
		expectError bool
	}{
		{
			name:        "1. Standard /chapter-detail/{id}",
			url:         "https://mangaball.com/chapter-detail/6acc608ca91bdf88681cd5c0",
			expectedID:  "6acc608ca91bdf88681cd5c0",
			expectError: false,
		},
		{
			name:        "2. URL with query parameter chapter=77",
			url:         "https://mangaball.com/chapter-detail/6acc608ca91bdf88681cd5c0?chapter=77",
			expectedID:  "6acc608ca91bdf88681cd5c0",
			expectError: false,
		},
		{
			name:        "3. Short /chapter/{id}",
			url:         "https://mangaball.com/chapter/6acc608ca91bdf88681cd5c0",
			expectedID:  "6acc608ca91bdf88681cd5c0",
			expectError: false,
		},
		{
			name:        "4. Query parameter chapter_id={id}",
			url:         "https://mangaball.com/chapter-detail?chapter_id=6acc608ca91bdf88681cd5c0",
			expectedID:  "6acc608ca91bdf88681cd5c0",
			expectError: false,
		},
		{
			name:        "5. Slug-id format",
			url:         "https://mangaball.com/read/manga-slug-6acc608ca91bdf88681cd5c0",
			expectedID:  "6acc608ca91bdf88681cd5c0",
			expectError: false,
		},
		{
			name:        "6. Title detail URL should reject with error",
			url:         "https://mangaball.com/title-detail/6acc608ca91bdf88681cd5b0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		got, err := parser.ExtractMangaBallChapterID(tt.url)
		if tt.expectError {
			if err == nil {
				t.Errorf("[%s] Expected error for URL %s, but got none", tt.name, tt.url)
			} else {
				t.Logf("[%s] OK -> got expected error: %v", tt.name, err)
			}
		} else {
			if err != nil {
				t.Errorf("[%s] Unexpected error: %v", tt.name, err)
			} else if got != tt.expectedID {
				t.Errorf("[%s] Expected ID %q, got %q", tt.name, tt.expectedID, got)
			} else {
				t.Logf("[%s] OK -> ID: %s", tt.name, got)
			}
		}
	}
}

func TestMangaBallChapterTitleFromAPI(t *testing.T) {
	// 1. When chapter_number is provided
	mock1 := `{"code":200,"data":{"chapter":{"chapter_number":77,"number":77,"name":"77","pages":["https://example.com/1.jpg"]}}}`
	var res1 parser.MangaBallResponse
	if err := json.Unmarshal([]byte(mock1), &res1); err != nil {
		t.Fatalf("Failed to parse mock1: %v", err)
	}
	title1 := parser.FormatMangaBallChapterTitle(&res1, "https://mangaball.com/chapter-detail/6acc608ca91bdf88681cd5c0")
	if title1 != "Ch 77" {
		t.Errorf("Expected 'Ch 77', got %q", title1)
	} else {
		t.Logf("[OK] MangaBall integer chapter resolved: %s", title1)
	}

	// 2. When number is float (e.g. 13.5)
	mock2 := `{"code":200,"data":{"chapter":{"number":13.5,"name":"13.5","pages":["https://example.com/1.jpg"]}}}`
	var res2 parser.MangaBallResponse
	if err := json.Unmarshal([]byte(mock2), &res2); err != nil {
		t.Fatalf("Failed to parse mock2: %v", err)
	}
	title2 := parser.FormatMangaBallChapterTitle(&res2, "https://mangaball.com/chapter-detail/6acc608ca91bdf88681cd5c0")
	if title2 != "Ch 13.5" {
		t.Errorf("Expected 'Ch 13.5', got %q", title2)
	} else {
		t.Logf("[OK] MangaBall float chapter resolved: %s", title2)
	}

	// 3. When raw text name is Chapter 42
	mock3 := `{"code":200,"data":{"chapter":{"name":"Chapter 42","pages":["https://example.com/1.jpg"]}}}`
	var res3 parser.MangaBallResponse
	if err := json.Unmarshal([]byte(mock3), &res3); err != nil {
		t.Fatalf("Failed to parse mock3: %v", err)
	}
	title3 := parser.FormatMangaBallChapterTitle(&res3, "https://mangaball.com/chapter-detail/6acc608ca91bdf88681cd5c0")
	if title3 != "Ch 42" {
		t.Errorf("Expected 'Ch 42', got %q", title3)
	} else {
		t.Logf("[OK] MangaBall name string resolved: %s", title3)
	}
}

func TestNicoMangaChapterTitleExtraction(t *testing.T) {
	tests := []struct {
		name       string
		html       string
		rawHTML    string
		chapterURL string
		expected   string
	}{
		{
			name: "1. NicoManga standard breadcrumb snippet from user",
			html: `<div class="jsx-4f85a76b1374b37f breadcrumb-navigation"><span class="jsx-4f85a76b1374b37f breadcrumb-item"><a href="https://nicomanga.com/" class="jsx-4f85a76b1374b37f"><i class="jsx-4f85a76b1374b37f fa fa-home"></i> Home</a></span><span class="jsx-4f85a76b1374b37f breadcrumb-separator">/</span><span class="jsx-4f85a76b1374b37f breadcrumb-item"><a href="https://nicomanga.com/manga-list.html" class="jsx-4f85a76b1374b37f">List manga</a></span><span class="jsx-4f85a76b1374b37f breadcrumb-separator">/</span><span class="jsx-4f85a76b1374b37f breadcrumb-item"><a href="https://nicomanga.com/manga835/14bc51bc.html" class="jsx-4f85a76b1374b37f manga-name">ISEKAI DE SLOW LIFE WO (GANBOU) - MANGA RAW</a></span><span class="jsx-4f85a76b1374b37f breadcrumb-separator">/</span><span class="jsx-4f85a76b1374b37f breadcrumb-item" style="color: rgb(255, 255, 255);">Chapter 69.1</span></div>`,
			expected: "Ch 69.1",
		},
		{
			name: "2. Trailing elements with breadcrumb-item class in DOM",
			html: `<div class="jsx-4f85a76b1374b37f breadcrumb-navigation"><span class="breadcrumb-item">Chapter 45</span></div><div class="footer"><span class="breadcrumb-item">Next Chapter</span></div>`,
			expected: "Ch 45",
		},
		{
			name: "3. Escaped Next.js streaming RSC chunk in rawHTML",
			rawHTML: `self.__next_f.push([1,"...<div class=\"jsx-4f85a76b1374b37f breadcrumb-navigation\"><span class=\"jsx-4f85a76b1374b37f breadcrumb-item\" style=\"color: rgb(255, 255, 255);\">Chapter 69.1<\/span><\/div>..."])`,
			expected: "Ch 69.1",
		},
		{
			name: "4. Next.js React RSC JSON chunk in rawHTML",
			rawHTML: `{"id":1,"children":"Chapter 69.1"}`,
			expected: "Ch 69.1",
		},
		{
			name: "5. Title tag fallback",
			html: `<html><head><title>ISEKAI DE SLOW LIFE WO Chapter 69.1 - NicoManga</title></head><body></body></html>`,
			expected: "Ch 69.1",
		},
		{
			name:       "6. Chapter URL fallback",
			chapterURL: "https://nicomanga.com/manga835/14bc51bc/chapter-69.1.html",
			expected:   "Ch 69.1",
		},
	}

	for _, tt := range tests {
		var doc *goquery.Document
		if tt.html != "" {
			var err error
			doc, err = goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("[%s] Failed to parse HTML: %v", tt.name, err)
			}
		}

		got := parser.ExtractNicoMangaChapterTitle(doc, tt.rawHTML, tt.chapterURL)
		if got != tt.expected {
			t.Errorf("[%s] Expected %q, got %q", tt.name, tt.expected, got)
		} else {
			t.Logf("[%s] OK -> %s", tt.name, got)
		}
	}
}

func TestProgressBarAndMetrics(t *testing.T) {
	// 1. Test FormatBytes
	if s := downloader.FormatBytes(500); s != "500 B" {
		t.Errorf("Expected '500 B', got %q", s)
	}
	if s := downloader.FormatBytes(1024); s != "1.0 KB" {
		t.Errorf("Expected '1.0 KB', got %q", s)
	}
	if s := downloader.FormatBytes(1572864); s != "1.5 MB" {
		t.Errorf("Expected '1.5 MB', got %q", s)
	}

	// 2. Test FormatSpeed
	if s := downloader.FormatSpeed(0); s != "0 B/s" {
		t.Errorf("Expected '0 B/s', got %q", s)
	}
	if s := downloader.FormatSpeed(5242880); s != "5.0 MB/s" {
		t.Errorf("Expected '5.0 MB/s', got %q", s)
	}

	// 3. Test RenderProgressBar
	bar0 := downloader.RenderProgressBar(0, 10, 10)
	if bar0 != "[          ]" {
		t.Errorf("Expected '[          ]', got %q", bar0)
	}
	barHalf := downloader.RenderProgressBar(5, 10, 10)
	if barHalf != "[====>     ]" {
		t.Errorf("Expected '[====>     ]', got %q", barHalf)
	}
	barFull := downloader.RenderProgressBar(10, 10, 10)
	if barFull != "[==========]" {
		t.Errorf("Expected '[==========]', got %q", barFull)
	}
}

func TestDecryptMangaPlus(t *testing.T) {
	keyHex := "aabbccdd"
	// Create original plaintext
	orig := []byte("Hello MangaPlus!")
	key := []byte{0xaa, 0xbb, 0xcc, 0xdd}

	// Encrypt with XOR
	enc := make([]byte, len(orig))
	for i := 0; i < len(orig); i++ {
		enc[i] = orig[i] ^ key[i%len(key)]
	}

	// Decrypt using DecryptMangaPlus
	urlWithKey := "https://jumpg-assets.tokyo-cdn.com/page.jpg#encryptionKey=" + keyHex
	dec, err := descramble.DecryptMangaPlus(enc, urlWithKey)
	if err != nil {
		t.Fatalf("DecryptMangaPlus failed: %v", err)
	}
	if string(dec) != string(orig) {
		t.Fatalf("Expected %q, got %q", orig, dec)
	}

	// URL without key should return unmodified bytes
	urlWithoutKey := "https://jumpg-assets.tokyo-cdn.com/page.jpg"
	unmod, err := descramble.DecryptMangaPlus(orig, urlWithoutKey)
	if err != nil || string(unmod) != string(orig) {
		t.Fatalf("Expected unmodified bytes when no key is present")
	}
}
