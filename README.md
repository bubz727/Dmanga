# CLI Manga Downloader

A lightweight, high-performance command-line manga and comic downloader written in Go.

This project is primarily focused on **Japanese RAW manga** and major digital platforms featuring expansive catalogs and high-resolution image quality (including official publisher platforms, GigaViewer, Comici, and dedicated raw archives).

---

## Supported Platforms

The downloader supports over 60 manga websites across various reader architectures:

- MangaFire (`mangafire.to`)
- Asura Scans (`asurascans.com`)
- Toonz & Toonx (`toonz.to`, `toonx.to`)
- WeebCentral (`weebcentral.com`)
- Rawkuma (`rawkuma.net`)
- MangaDex (`mangadex.org`)
- Manganato (`manganato.gg`, `www.manganato.gg`)
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

---

## Acknowledgments & Credits

Big thanks to the following open-source projects, extension repositories, and userscripts whose implementations and research helped guide the parsers in this project:

- [keiyoushi/extensions-source](https://github.com/keiyoushi/extensions-source/)
- [InvalidDavid/UMA](https://github.com/InvalidDavid/UMA)
- [Kotatsu-Redo/kotatsu-parsers-redo](https://github.com/Kotatsu-Redo/kotatsu-parsers-redo/)
- [Comic-Walker Manga Downloader](https://greasyfork.org/en/scripts/557576-comic-walker-manga-downloader)
- [Rippper](https://greasyfork.org/en/scripts/573396-rippper)
