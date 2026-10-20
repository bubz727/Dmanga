# Dmanga Architecture Overview

Dmanga follows the **Standard Go Project Layout** (`cmd/` and `internal/`) with a strict unidirectional dependency graph to eliminate circular imports.

## Dependency Graph

```text
cmd/dmanga/main.go
  └── internal/cli (app, args, help, list, diagnostic)
        └── internal/downloader (downloader, worker, writer)
              ├── internal/parser (registry, 19 parsers, themes)
              │     ├── internal/core (types, errors, chapter)
              │     ├── internal/fetcher (client, headers, cookies, cloudflare, ratelimit)
              │     ├── internal/image/descramble (comici, comicwalker)
              │     └── internal/utils (url, naming, extension)
              ├── internal/image (processor)
              └── internal/utils (naming, extension)
```

## Core Modules

1. **`cmd/dmanga/`**: Lightweight CLI entry point that receives `os.Args` and delegates to the CLI runner.
2. **`internal/core/`**: Fundamental domain types (`Chapter`), domain error types (`ErrNotFound`, etc.), and chapter fetching pipeline helpers.
3. **`internal/parser/`**: Site-specific parser implementations, base themes (`MangaThemesia`, `Natsu`), and the centralized parser registry.
4. **`internal/fetcher/`**: HTTP networking, browser client hints, session persistence (`.cf_cookies.json`), CDP WebSocket Cloudflare Turnstile bypass, and rate-limiting.
5. **`internal/downloader/`**: Concurrent goroutine worker pool (controlled concurrency) and file writing to disk.
6. **`internal/image/`**: Image format processing, extension resolution, and matrix tile de-scrambling algorithms (Comici 4x4 tiles, Kadokawa Comic-Walker).
7. **`internal/cli/`**: CLI presentation, multi-URL parsing, diagnostic runner (`-t`), and parser table printer (`-l`).
8. **`internal/utils/`**: Pure helper functions for URL sanitization, Windows-safe path cleaning, and image file extensions.
