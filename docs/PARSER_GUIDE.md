# Parser Implementation Guide

This guide provides step-by-step instructions for adding support for a new manga website to Dmanga.

---

## 1. Defining a Parser

Every parser lives in the `internal/parser/` directory (e.g., `internal/parser/mysite.go`) and must implement the `core.Parser` interface:

```go
package parser

import (
    "dmanga/internal/core"
)

type MySite struct{}

func NewMySite() *MySite {
    return &MySite{}
}

func (m *MySite) Name() string {
    return "MySite"
}

func (m *MySite) Domains() []string {
    return []string{"mysite.com", "www.mysite.com"}
}

func (m *MySite) GetImages(chapterURL string) ([]string, error) {
    // Logic for extracting image URLs
}

func init() {
    Register(NewMySite())
}
```

---

## 2. Optional Capabilities

### A. Automatic Chapter Title & Subfolder Naming
If the target website exposes explicit chapter numbers or titles (e.g., "Chapter 42" or "第1話"), implement `core.ChapterParser`:

```go
func (m *MySite) GetChapter(chapterURL string) (*core.Chapter, error) {
    // Return &core.Chapter{ Title: "Ch 42", Images: images }, nil
}
```

### B. Scrambled Image De-scrambling
If the website serves scrambled image tiles (such as Comici), implement `core.ImageProcessor`:

```go
func (m *MySite) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
    // De-scramble the image bytes and return clean JPEG/PNG bytes
}
```

---

## 3. Automatic Registration

Parsers register themselves automatically into `internal/parser/registry.go` inside the Go `init()` function:

```go
func init() {
    Register(NewMySite())
}
```

---

## 4. Testing Your New Parser

Run diagnostic mode to verify the parser works without downloading image files:

```powershell
go run ./cmd/dmanga -t https://mysite.com/chapter-1
```
