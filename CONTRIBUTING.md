# Contributing Guide

Thank you for your interest in contributing to **CLI Manga Downloader**! This document outlines code standards, architectural patterns, the process for adding new website parsers, and testing guidelines.

---

## Architecture Overview

The project uses a clean modular design where `core/` houses the primary framework engine, `themes/` provides reusable CMS templates, and `parsers/` contains site-specific extractors:

```text
├── core/                 # Core framework engine, registry, networking, & image pipeline
│   ├── base.go           # Core Parser & ImageProcessor interfaces, FetchHTML, GetFileExtension
│   ├── registry.go       # Central parser registry: Register, FindParser, GetAllParsers
│   ├── descramble.go     # Image tile descrambler & JPEG/PNG format normalizer
│   ├── downloader.go     # Concurrent worker pool and file writing pipeline
│   ├── diagnostic.go     # Anti-bot and parser verification suite
│   ├── cli.go            # CLI arguments, help formatting, and parser listing
│   ├── cloudflare.go     # Cloudflare Turnstile hybrid browser solver
│   └── errors.go         # Standard project error types
├── themes/               # Reusable CMS base templates
│   ├── mangathemesia.go  # WordPress MangaThemesia CMS base template
│   └── natsu.go          # Natsu reader base template
├── parsers/              # Individual site parsers (one file per platform/site)
│   ├── asurascan.go      # Custom Next.js / Astro props parser
│   ├── mangafire.go      # Custom XOR VRF API signer parser
│   ├── comici.go         # Comici platforms parser (descrambler enabled)
│   └── ...               # Additional site parsers
├── main.go               # Minimal CLI entry point and argument router
└── main_test.go          # Test suites and diagnostic runner
```

---

## Adding a New Parser

To add support for a new manga website, create a single new file in the `parsers/` directory (e.g., `parsers/kiryuu.go`).

IMPORTANT: Parsers must register themselves automatically into the core registry using Go's `init()` function with `core.Register(...)`.

---

### Scenario 1: Websites Using Standard CMS Themes

If the website uses a recognized CMS theme (such as MangaThemesia or Natsu), simply embed the corresponding base theme struct:

```go
package parsers

import (
	"manga-downloader/core"
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
	core.Register(NewKiryuu())
}
```

---

### Scenario 2: Websites with Custom Architecture or APIs

If the website uses a custom frontend (e.g., Next.js streaming, custom APIs, GraphQL, or custom HTML layouts), implement the `core.Parser` interface directly:

```go
package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
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

	doc, _, err := core.FetchHTML(chapterURL, map[string]string{
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

		imgURL = core.CleanImageURL(imgURL, parsedURL.Host)
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
	core.Register(NewExampleCustom())
}
```

---

### Scenario 3: Websites with Scrambled/Puzzle Images

If the website uses an image scrambling technique (like Comici or GigaViewer), implement `core.ImageProcessor`:

```go
func (p *MyParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
    return core.ProcessImageDescramble(rawBytes, imageURL)
}
```

---

## Testing Your Parser

Before submitting your changes, run automated tests:

### 1. Compile Check
Verify that your changes compile without warnings:
```powershell
go build -ldflags="-s -w" -o Dmanga.exe .
```

### 2. Diagnostic Mode
Test your parser live against a real chapter URL:
```powershell
.\Dmanga.exe -t https://example.com/manga/chapter-1
```

### 3. Automated Test Suite
Run the project unit test suite:
```powershell
go test ./...
```
