# CLI Manga Downloader

A lightweight, high-performance command-line manga and comic downloader written in Go.

This project is primarily focused on **Japanese RAW manga** and major digital platforms featuring expansive catalogs and high-resolution image quality (including official publisher platforms, GigaViewer, Comici, and dedicated raw archives).

---

## Features

- High Concurrency: Downloads images in parallel using worker pools for fast downloads.
- Multi-Link Support: Download multiple chapter URLs in a single command by separating URLs with spaces or pipe (`|`) characters.
- Automatic Format Normalization: Non-standard image formats (such as WebP and GIF) are automatically converted to high-quality JPEG (`.jpg`). Original JPEG and PNG files are preserved without re-encoding.
- Built-in Image Descrambling: Native puzzle-tile descrambler for Japanese platforms running Comici and GigaViewer (such as Shonen Jump+, Tonari no Young Jump, and Comic Days).
- Informative Logging: Detailed progress output displaying original filenames, target filenames, and pixel dimensions (e.g., `sample.webp -> 001.jpg (800x1200)`).
- Zero-Padding Output: Downloads are sequentially formatted (`001.jpg`, `002.jpg`, etc.) directly into the current directory or dedicated chapter folders.
- Cloudflare Turnstile Solver: Integrated hybrid solver opens a lightweight browser window when Cloudflare challenges (403/503) are detected, caches `cf_clearance` cookies locally, and seamlessly resumes downloading.
- Diagnostic Mode: Test parser extraction and check for anti-bot blocks without downloading files.

---

## Supported Platforms

The downloader supports over 60 manga websites across various reader architectures:

- MangaFire (`mangafire.to`)
- Asura Scans (`asurascans.com`)
- Toonz & Toonx (`toonz.to`, `toonx.to`)
- WeebCentral (`weebcentral.com`)
- Rawkuma (`rawkuma.net`)
- MangaDex (`mangadex.org`)
- Manganato (`manganato.gg`, `manganato.com`, `chapmanganato.to`, `natomanga.com`, `readmanganato.com`)
- MANGA Plus by SHUEISHA (`mangaplus.shueisha.co.jp`)
- Naver Webtoon (`comic.naver.com`, `m.comic.naver.com`)
- MangaBall (`mangaball.com`)
- MangaRaw4U (`mangaraw4u.com`)
- Nihonkuni (`nihonkuni.com`)
- NicoManga (`nicomanga.com`, `nicomanga.net`)
- Weloma (`weloma.net`, `weloma.art`)
- XComic (`xcomic.me`)
- Comic-Walker (`comic-walker.com`)
- GigaViewer Platforms (25 domains, including `comic-days.com`, `shonenjumpplus.com`, `tonarinoyj.jp`, `kuragebunch.com`, `sunday-webry.com`)
- Comici Platforms (25 domains, including `younganimal.com`, `championcross.jp`, `bigcomics.jp`, `hanayume.com`)

---

## Project Structure

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
│   ├── mangathemesia.go  # MangaThemesia CMS base template
│   └── natsu.go          # Natsu reader base template
├── parsers/              # Self-registering site parsers
│   ├── comici.go         # Comici reader platforms (25 domains)
│   ├── gigaviewer.go     # GigaViewer reader platforms (25 domains)
│   ├── mangafire.go      # MangaFire VRF API signer & extractor
│   ├── mangadex.go       # MangaDex At-Home API parser
│   ├── mangaplus.go      # Shueisha MangaPlus Protobuf stream parser
│   └── ...               # Additional site parsers
├── main.go               # Minimal CLI entry point and argument router
├── main_test.go          # Unit tests and diagnostic test runner
├── CONTRIBUTING.md       # Developer guide for adding new parsers
└── README.md             # Project documentation
```

---

## Installation & Build

### 1. Prerequisites
- Go 1.22 or later installed on your system.

### 2. Build Binary
Compile an optimized, standalone executable:

```powershell
go build -ldflags="-s -w" -o Dmanga.exe .
```

### 3. Add to System PATH (Optional)
To run the downloader from any folder in Command Prompt or PowerShell:
1. Place `Dmanga.exe` into a permanent directory (e.g., `C:\Tools\`).
2. Add `C:\Tools` to your Windows `PATH` environment variables.

---

## Usage

### Single Chapter Download
Navigate to your desired target folder and run:

```cmd
Dmanga https://mangafire.to/title/ro8ro-all-class-awakening-god-slayer/chapter/9479428
```

Images will be saved directly into the current working directory as `001.jpg`, `002.jpg`, etc.

### Multi-Link Download
You can download multiple chapters simultaneously by separating URLs with spaces or pipe (`|`) characters:

```cmd
Dmanga https://example.com/chapter-1 https://example.com/chapter-2
```

Or using the pipe delimiter:

```cmd
Dmanga "https://example.com/chapter-1 | https://example.com/chapter-2"
```

In multi-link mode, each chapter is saved in its own subfolder (`Chapter 1`, `Chapter 2`, etc.).

### Interactive Prompt Mode
Run `Dmanga` without arguments:

```cmd
Dmanga
```
The program will prompt you to enter one or multiple chapter URLs.

### Diagnostic Test Mode (-t)
Inspect domain routing, HTTP status, anti-bot protection, and image extraction count without downloading any files:

```cmd
Dmanga -t https://comic-days.com/episode/3270296674374344033
```

### List Supported Parsers (-l, --list, list)
Display all registered manga parsers along with their supported domains:

```cmd
Dmanga -l
```

---

## Testing

Run unit tests:
```powershell
go test -v .
```

Run a live diagnostic check against a specific URL:
```powershell
go test -v . -args "https://comic-days.com/episode/3270296674374344033"
```
