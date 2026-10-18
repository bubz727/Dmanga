# Contributing Guide

Thank you for your interest in contributing to **CLI Manga Downloader**! This document outlines code standards, architectural patterns, the process for adding new website parsers, and testing guidelines.

---

## Architecture Overview

The project uses a modular design separating base themes from site-specific parsers:

```text
├── themes/               # Reusable CMS base templates & descrambling engine
│   ├── base.go           # Core Parser interface & HTTP helpers
│   ├── descramble.go     # Tile descrambler & image format normalization
│   ├── madara.go         # WordPress Madara CMS base template
│   ├── mangathemesia.go  # WordPress MangaThemesia CMS base template
│   └── natsu.go          # Natsu reader base template
├── parsers/              # Individual site parsers (one file per platform/site)
│   ├── asurascan.go      # Custom Next.js / Astro props parser
│   ├── mangafire.go      # Custom XOR VRF API signer parser
│   ├── registry.go       # Central parser registry (DO NOT EDIT)
│   └── ...               # Additional site parsers
├── main.go               # CLI entry point, worker pool runner, and CLI flags
└── main_test.go          # Test suites and diagnostic runner
```

---

## Adding a New Parser

To add support for a new manga website, create a single new file in the `parsers/` directory (e.g., `parsers/kiryuu.go`).

IMPORTANT: Never modify `parsers/registry.go`. Parsers must register themselves automatically using Go's `init()` function.

---

### Scenario 1: Websites Using Standard CMS Themes

If the website uses a recognized CMS theme (such as MangaThemesia, Madara, or Natsu), simply embed the corresponding base theme struct:

```go
package parsers

import (
	"manga-downloader/themes"
)

type Kiryuu struct {
	themes.MangaThemesia
}

func NewKiryuu() *Kiryuu {
	return &Kiryuu{
		MangaThemesia: themes.NewMangaThemesia(),
	}
}

func (k *Kiryuu) Name() string {
	return "Kiryuu"
}

func (k *Kiryuu) Domains() []string {
	return []string{
		"kiryuu01.com",
		"kiryuu.id",
		"kiryuu.org",
	}
}

func (k *Kiryuu) GetImages(chapterURL string) ([]string, error) {
	return k.ExtractImages(chapterURL)
}

func init() {
	Register(NewKiryuu())
}
```

---

### Scenario 2: Websites with Custom Architecture or APIs

If the website uses a custom frontend (e.g., Next.js streaming, custom APIs, GraphQL, or custom HTML layouts), implement the `themes.Parser` interface directly:

```go
package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/themes"
)

type ExampleCustom struct{}

func NewExampleCustom() *ExampleCustom {
	return &ExampleCustom{}
}

func (e *ExampleCustom) Name() string {
	return "Example Custom"
}

func (e *ExampleCustom) Domains() []string {
	return []string{"examplecustom.com"}
}

func (e *ExampleCustom) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	doc, _, err := themes.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	})
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	doc.Find("div.chapter-content img").Each(func(i int, s *goquery.Selection) {
		imgURL, exists := s.Attr("src")
		if !exists || strings.TrimSpace(imgURL) == "" {
			imgURL, _ = s.Attr("data-src")
		}

		imgURL = themes.CleanImageURL(imgURL, parsedURL.Host)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	if len(images) == 0 {
		return nil, fmt.Errorf("no images found for this chapter")
	}

	return images, nil
}

func init() {
	Register(NewExampleCustom())
}
```

---

### Scenario 3: Websites with Scrambled Canvas Tiles

If the target website delivers scrambled/tiled images (such as GigaViewer or Comici), implement the optional `themes.ImageProcessor` interface:

```go
func (p *CustomParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return themes.ProcessImageDescramble(rawBytes, imageURL)
}
```

The downloader will automatically route the downloaded byte stream through your `ProcessImage` hook before saving files to disk.

---

## Testing Guidelines

Every new parser must be tested before submitting a pull request.

### 1. Diagnostic Test Mode
Run the diagnostic test against a live chapter URL to verify domain matching, HTTP connectivity, and image extraction without downloading images:

```powershell
go run . -t "https://example.com/chapter-1"
```

Expected output:
```text
[OK 1/4] Domain Match      : Example Custom
[OK 2/4] HTTP Connection   : 200 OK (Unblocked)
[OK 3/4] Image Extraction  : Found X images
[OK 4/4] Image Sample      : https://.../001.jpg
[SUCCESS] Parser 'Example Custom' is fully functional!
```

### 2. Run Test Suite
Run the test suite to ensure all unit tests pass:

```powershell
go test -v .
```

Verify specific URLs via test arguments:

```powershell
go test -v . -args "https://example.com/chapter-1"
```

---

## Submission Checklist

Before submitting a Pull Request, verify the following:

- The new parser is located inside `parsers/` with a clear, lowercase filename (e.g., `parsers/sitename.go`).
- The parser registers itself via `func init() { Register(NewParser()) }`.
- `parsers/registry.go` has not been modified.
- Live diagnostic test (`go run . -t <url>`) executes successfully.
- All unit tests pass with `go test ./...`.
- No binary files (`*.exe`), temporary files, or downloaded image files are committed.
