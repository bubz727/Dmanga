# Contributing to Dmanga

Thank you for your interest in contributing to **Dmanga**! This document outlines the project architecture, code standards, guidelines for implementing new parsers, and testing instructions.

---

## Architecture Overview

Dmanga follows a modular architecture where `core/` contains the networking, concurrency, image processing, and registry engine, `themes/` provides reusable base reader engines, and `parsers/` holds isolated website extractors:

```text
Dmanga/
├── cmd/
│   └── dmanga/
│       └── main.go           # Minimal CLI entry point & argument router
├── internal/
│   ├── core/                 # Core interfaces (Parser, ChapterParser, ImageProcessor) & Chapter struct
│   ├── parser/               # 19 site parsers & single central parser registry
│   │   └── themes/           # Reusable CMS base templates (MangaThemesia, Natsu)
│   ├── fetcher/              # HTTP client, CDP Cloudflare Turnstile solver, cookies & rate limiting
│   ├── downloader/           # Concurrent worker pool and disk writer
│   ├── image/                # Format normalizer & tile de-scrambling (Comici, Comic-Walker)
│   ├── cli/                  # CLI arguments, help banner, diagnostic suite & table formatter
│   └── utils/                # Pure helper functions (URL sanitizer, path naming, file extensions)
├── tests/                    # Integration tests & offline test fixtures
├── docs/                     # Technical documentation & contributing guides
└── .cf_cookies.json          # Cached Cloudflare session cookies per domain
```

---

## Core Interfaces

All parsers interact with the engine through interfaces defined in `core/types.go`:

### 1. `Parser` (Required)
Every site parser must implement `core.Parser`:
```go
type Parser interface {
    Name() string
    Domains() []string
    GetImages(chapterURL string) ([]string, error)
}
```

### 2. `ChapterParser` (Optional)
If a site provides explicit chapter titles (e.g. `Chapter 42` or `第1話`), implement `core.ChapterParser` to allow Dmanga to create neatly named subfolders during multi-chapter downloads:
```go
type ChapterParser interface {
    GetChapter(chapterURL string) (*Chapter, error)
}
```

### 3. `ImageProcessor` (Optional)
If a site serves scrambled, puzzle, or encrypted images, implement `core.ImageProcessor` to descramble image bytes before they are written to disk:
```go
type ImageProcessor interface {
    ProcessImage(rawBytes []byte, imageURL string) ([]byte, error)
}
```

---

## Adding a New Parser

To add support for a new manga website, create a single new file in `parsers/` (e.g., `parsers/kiryuu.go`).

> [!IMPORTANT]
> Parsers must register themselves automatically using Go's `init()` function with `core.Register(...)`.

---

### Scenario 1: Websites Using Standard CMS Themes

If the website uses a recognized CMS theme (such as MangaThemesia or Natsu), embed the corresponding base theme struct:

```go
package parsers

import (
	"dmanga/internal/core"
	"dmanga/internal/parser/themes"
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

If the website uses a custom frontend (e.g., Next.js, custom REST APIs, or specific HTML layouts), implement `core.Parser` directly:

```go
package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
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

### Scenario 3: Websites with Scrambled Images

If the site serves scrambled image tiles (such as Comici), implement `core.ImageProcessor`:

```go
func (p *MyParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
    return core.ProcessImageDescramble(rawBytes, imageURL)
}
```

---

## Testing Your Parser

Before submitting changes, ensure everything builds and tests pass:

### 1. Compile Check
Verify that the binary compiles cleanly:
```powershell
go build -ldflags="-s -w" -o Dmanga.exe .
```

### 2. Diagnostic Mode
Test domain routing, anti-bot protection, and image extraction without downloading images:
```powershell
.\Dmanga.exe -t https://example.com/manga/chapter-1
```

### 3. Automated Test Suite
Run the unit test suite:
```powershell
go test -v .
```
