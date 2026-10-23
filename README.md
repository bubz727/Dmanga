# Dmanga

A lightweight, high-performance command-line manga, comic, and webtoon downloader written in Go.

Primarily focused on **RAW manga** and major digital platforms and high-resolution image quality.

---

## Supported Platforms

| Parser | Domains Supported |
| :--- | :--- |
| **GigaViewer** | 25 domains (`comic-days.com`, `shonenjumpplus.com`, `tonarinoyj.jp`, `sunday-webry.com`, `kuragebunch.com`, `comic-earthstar.com`, `comic-gardo.com`, `comic-zenon.com`, etc.) |
| **Comici** | 25 domains (`younganimal.com`, `championcross.jp`, `bigcomics.jp`, `hanayume.com`, `youngchampion.jp`, `comicpash.jp`, `comicride.jp`, etc.) |
| **Baozimh** | 12 domains (`baozimh.com`, `baozimh.org`, `baozimhw.com`, `twmanga.com`, `webmota.com`, and regional/www subdomains) |
| **BookWalker** | 8 domains (`bookwalker.jp`, `global.bookwalker.jp`, `viewer-trial.bookwalker.jp`, `bookwalker.com`, etc.) |
| **Comic-Walker** | `comic-walker.com`, `www.comic-walker.com` |
| **MANGA Plus** | `mangaplus.shueisha.co.jp` |
| **MangaUp Global** | `global.manga-up.com` |
| **MangaUp Japan** | `manga-up.com`, `www.manga-up.com` |
| **Naver Comic** | `comic.naver.com`, `m.comic.naver.com` |
| **Webtoon** | `webtoons.com`, `m.webtoons.com`, `www.webtoons.com` |
| **Tappytoon** | `tappytoon.com`, `www.tappytoon.com` |
| **MangaFire** | `mangafire.to` |
| **MangaDex** | `mangadex.org` |
| **Mangago** | `mangago.me` |
| **Manganato** | `manganato.gg`, `www.manganato.gg` |
| **Asura Scans** | `asurascans.com` |
| **Toonz & Toonx** | `toonz.to`, `toonx.to` |
| **Weeb Central** | `weebcentral.com` |
| **Rawkuma** | `rawkuma.net` |
| **MangaBall** | `mangaball.com`, `mangaball.net` |
| **MangaRaw4U** | `mangaraw4u.com`, `www.mangaraw4u.com` |
| **NicoManga** | `nicomanga.com`, `nicomanga.net`, and `www.*` |
| **Nihonkuni** | `nihonkuni.com`, `www.nihonkuni.com` |
| **Weloma** | `weloma.net`, `weloma.art`, `www.weloma.net` |
| **XComic** | `xcomic.me`, `xcomic.net` |

Run `Dmanga -l` to view all supported platforms in your terminal.

---

## Quick Installation

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/bubz727/Dmanga/main/install.ps1 | iex
```

#### Linux (curl & bash)
```bash
curl -fsSL https://raw.githubusercontent.com/bubz727/Dmanga/main/install.sh | bash
```

### Build from Source

#### Prerequisites
- [Go 1.22+](https://go.dev/dl/) installed on your system.

#### Compile Binary
Compile an optimized, standalone executable:
```powershell
make build
```
Or directly using `go build`:
```powershell
go build -ldflags="-s -w" -o Dmanga.exe ./cmd/dmanga
```

---

## Updating Dmanga

Update to the latest release automatically with safe rollback and SHA256 checksum verification:
```cmd
Dmanga update
```

Check if a new release is available without updating:
```cmd
Dmanga --check-update
```

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

### Download Speed (`-c`, `--concurrency`)
Control how many images are downloaded in parallel (default `10`, maximum `32`):

```cmd
Dmanga -c 16 https://example.com/chapter-1
```

You can also set it once via the `DMANGA_CONCURRENCY` environment variable. The flag takes priority over the environment variable. If a site starts returning HTTP 429 (rate limit), Dmanga automatically lowers the number of active workers for that chapter.

### Resume and Skip Existing (`--force`)
Images that were already downloaded completely (same file name, non-empty) are skipped, so re-running an interrupted chapter only fetches what is missing. Files are written to a temporary `.part` file first, so a partially written image is never mistaken for a finished one. Use `--force` to re-download everything:

```cmd
Dmanga --force https://example.com/chapter-1
```

Note: when downloading several different chapters into the same folder without `-f`, files such as `001.jpg` from an earlier chapter count as existing. Use `-f` (subfolder per chapter) or `--force` in that case.

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

### Chapter Prefetch (`--prefetch`)
While one chapter is downloading, Dmanga fetches the page and image list of the next chapters in the background, so there is no loading pause between chapters. The default is `2` chapters ahead; use `--prefetch 0` to turn it off, or raise it up to `5`:

```cmd
Dmanga --prefetch 3 https://example.com/chapter-1 https://example.com/chapter-2 https://example.com/chapter-3
```

It can also be set with the `DMANGA_PREFETCH` environment variable (the flag takes priority). The first chapter of each website is always fetched in the foreground, so any Cloudflare verification window appears at the right moment.

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
