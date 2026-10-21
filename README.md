# Dmanga

A lightweight, high-performance command-line manga, comic, and webtoon downloader written in Go.

**Dmanga** is primarily focused on **Japanese RAW manga** and major digital platforms featuring expansive catalogs and high-resolution image quality (including official publisher platforms, GigaViewer, Comici, and dedicated raw archives).

---

## Supported Platforms

Dmanga currently supports **80 domains** across **19 specialized parsers**:

| Parser | Domains Supported | Architecture / Notes |
| :--- | :--- | :--- |
| **GigaViewer** | 25 domains (`comic-days.com`, `shonenjumpplus.com`, `tonarinoyj.jp`, `sunday-webry.com`, `kuragebunch.com`, `comic-earthstar.com`, `comic-gardo.com`, `comic-zenon.com`, etc.) | Official publisher reader, authentic Japanese chapter naming |
| **Comici** | 25 domains (`younganimal.com`, `championcross.jp`, `bigcomics.jp`, `hanayume.com`, `youngchampion.jp`, `comicpash.jp`, `comicride.jp`, etc.) | 4x4 image tile de-scrambler enabled |
| **Comic-Walker** | `comic-walker.com`, `www.comic-walker.com` | Official Kadokawa platform, API metadata parsing |
| **MANGA Plus** | `mangaplus.shueisha.co.jp` | Official Shueisha platform, high-resolution streams |
| **Naver Comic** | `comic.naver.com`, `m.comic.naver.com` | Korean webtoon reader with Korean Hwa extraction |
| **MangaFire** | `mangafire.to` | Custom XOR / VRF API signer |
| **MangaDex** | `mangadex.org` | MangaDex v5 REST API integration |
| **Mangago** | `mangago.me` | AES decryption & HTML unpacker |
| **Manganato** | `manganato.gg`, `www.manganato.gg` | Standard scan reader |
| **Asura Scans** | `asurascans.com` | Astro / Next.js props parser |
| **Toonz & Toonx** | `toonz.to`, `toonx.to` | Next.js RSC (React Server Components) payload parser |
| **Weeb Central** | `weebcentral.com` | HTMX reader with full-chapter image extraction |
| **Rawkuma** | `rawkuma.net` | Natsu theme reader engine |
| **MangaBall** | `mangaball.com`, `mangaball.net` | Next.js API & custom chapter detail parsing |
| **MangaRaw4U** | `mangaraw4u.com`, `www.mangaraw4u.com` | Multi-format chapter slug parser |
| **NicoManga** | `nicomanga.com`, `nicomanga.net`, and `www.*` | RAW archive reader |
| **Nihonkuni** | `nihonkuni.com`, `www.nihonkuni.com` | RAW archive reader |
| **Weloma** | `weloma.net`, `weloma.art`, `www.weloma.net` | RAW archive reader |
| **XComic** | `xcomic.me`, `xcomic.net` | RAW archive reader |

Run `Dmanga -l` to view all supported platforms in your terminal.

---

## Installation & Build

### 1. Prerequisites
- [Go 1.22+](https://go.dev/dl/) installed on your system.

### 2. Build Binary
Compile an optimized, standalone executable:

```powershell
go build -ldflags="-s -w" -o Dmanga.exe ./cmd/dmanga
```

Alternatively, you can compile directly using the provided Makefile:
```powershell
make build
```

### 3. Add to System PATH (Optional)
To run `Dmanga` from any directory in Command Prompt or PowerShell:
1. Place `Dmanga.exe` into a permanent directory (e.g., `C:\Tools\`).
2. Add `C:\Tools` to your system `PATH` environment variable.

---

## Usage

### Single Chapter Download
Navigate to your desired directory and run:

```cmd
Dmanga https://mangafire.to/title/ro8ro-all-class-awakening-god-slayer/chapter/9479428
```

Images are downloaded directly into the current directory as `001.jpg`, `002.jpg`, etc.

### Single Chapter in Subfolder (`-f`, `--folder`)
To download a single chapter into its own dedicated subfolder (named after the chapter, e.g., `Ch 185` or authentic Japanese title):

```cmd
Dmanga -f https://weebcentral.com/chapters/01M5DR493EHYY89H1S8B8GY0EG
```

### Multi-Link Batch Download
Download multiple chapters simultaneously by separating URLs with spaces or pipe (`|`) characters:

```cmd
Dmanga https://example.com/chapter-1 https://example.com/chapter-2
```

Or using the pipe delimiter:

```cmd
Dmanga "https://example.com/chapter-1 | https://example.com/chapter-2"
```

In multi-link mode, each chapter is automatically saved in its own organized subfolder (e.g., `Ch 1`, `Ch 2`, or authentic Japanese title `第1話`).

### Prompt Mode
Run `Dmanga` without arguments:

```cmd
Dmanga
```

You will be prompted to paste one or multiple URLs.

### Diagnostic Test Mode (`-t`)
Inspect domain routing, HTTP status, anti-bot protection, and extracted image count without downloading any files:

```cmd
Dmanga -t https://comic-days.com/episode/3270296674374344033
```

### List Supported Parsers (`-l`, `--list`, `list`)
Display all registered manga parsers along with their supported domains:

```cmd
Dmanga -l
```

---

## Testing

Run the full offline unit test suite:
```powershell
go test -v ./...
```

Run a live diagnostic check against a specific URL:
```powershell
go test -v ./tests/integration/... -args "https://comic-days.com/episode/3270296674374344033"
```

---

## Documentation & Architecture

- [Architecture Overview](docs/ARCHITECTURE.md): Modular package design and unidirectional dependency graph.
- [Parser Implementation Guide](docs/PARSER_GUIDE.md): Step-by-step instructions for adding new parsers.
- [Contributing Guidelines](docs/CONTRIBUTING.md): Code standards and pull request workflows.

---

## Acknowledgments & Credits

Special thanks to the following open-source projects, extension repositories, and userscripts whose implementations and research helped guide the parsers in this project:

- [keiyoushi/extensions-source](https://github.com/keiyoushi/extensions-source/)
- [InvalidDavid/UMA](https://github.com/InvalidDavid/UMA)
- [Kotatsu-Redo/kotatsu-parsers-redo](https://github.com/Kotatsu-Redo/kotatsu-parsers-redo/)
- [Comic-Walker Manga Downloader](https://greasyfork.org/en/scripts/557576-comic-walker-manga-downloader)
- [Rippper](https://greasyfork.org/en/scripts/573396-rippper)
