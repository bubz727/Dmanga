# CLI Manga Downloader (Go)

Aplikasi downloader komik/manga CLI yang super ringan, cepat, dan to-the-point.

## Fitur Utama
- **To-The-Point:** Buka CMD di folder mana saja, jalankan `downloadManga`, masukkan URL chapter, beres.
- **Auto Detect Domain:** Otomatis mengenali situs manga berdasarkan URL.
- **Anti Recompile (`rules.json`):** Jika situs ganti CSS selector atau ganti domain, kamu hanya perlu mengedit file `rules.json` tanpa perlu compile ulang kodenya.
- **Download Paralel:** Menggunakan Goroutines (5 worker bersamaan) sehingga download puluhan gambar selesai dalam hitungan detik.
- **Format Rapi:** Otomatis memberi nama berurutan 3 digit (`001.jpg`, `002.jpg`, `003.jpg`, dst).

---

## Struktur Proyek

```text
C:\Bubz\CLI-Manga-Downloader\
├── go.mod             # Definisi dependensi Go
├── rules.json         # Konfigurasi selector & domain tiap situs (bisa diedit di Notepad)
├── main.go            # Kode utama (Router domain, HTML extractor, paralel downloader)
└── README.md          # Petunjuk penggunaan
```

---

## Cara Build & Pasang di Windows

1. **Install Go (jika belum ada):**
   ```powershell
   winget install GoLang.Go
   ```
2. **Download dependensi & Compile:**
   ```powershell
   go mod tidy
   go build -o downloadManga.exe
   ```
3. **Pasang ke Windows PATH (Agar bisa dipanggil dari folder mana saja):**
   - Buat satu folder tetap, misalnya `C:\Tools\`
   - Pindahkan `downloadManga.exe` dan `rules.json` ke folder `C:\Tools\`
   - Tambahkan `C:\Tools` ke `System Environment Variables (PATH)` Windows.

---

## Cara Penggunaan

Cukup buka CMD/Terminal di folder tempat kamu ingin menyimpan gambar:

```cmd
C:\Komik\Solo Leveling\Chapter 01> downloadManga
```
Lalu masukkan URL chapter saat diminta:
```text
Masukkan URL Chapter: https://komiku.id/ch/one-piece-chapter-1100/
```
Atau langsung jadikan argumen:
```cmd
downloadManga https://komiku.id/ch/one-piece-chapter-1100/
```
Gambar akan langsung ter-download ke folder tersebut dengan nama `001.jpg`, `002.jpg`, dst.
