# Panduan Kontribusi (Contributing Guide)

Terima kasih atas ketertarikanmu untuk berkontribusi pada proyek **CLI Manga Downloader**! Panduan ini menjelaskan standar penulisan kode, alur penambahan parser situs komik baru, dan cara melakukan pengujian (testing).

---

## 📁 Struktur Arsitektur

Proyek ini menggunakan arsitektur modular berbasis **Themes** dan **Parsers**:

```text
├── themes/               # Kumpulan base template CMS (MangaThemesia, Madara, dll)
│   ├── base.go           # Interface Parser & fungsi helper HTTP
│   └── mangathemesia.go  # Logika parser standar tema MangaThemesia
├── parsers/              # Satu file untuk satu situs manga
│   ├── apkomik.go        # Contoh parser situs ApKomik
│   └── registry.go       # Sistem auto-registration (JANGAN DIEDIT)
├── main.go               # Entry point CLI
├── main_test.go          # Pengujian dan diagnosa parser
└── CONTRIBUTING.md       # Panduan ini
```

---

## 🚀 Tata Cara Menambahkan Parser Baru

Untuk menambahkan situs komik baru, kamu **hanya perlu membuat 1 file baru** di dalam folder `parsers/` (contoh: `parsers/kiryuu.go`). 

> [!IMPORTANT]
> **JANGAN PERNAH mengedit `parsers/registry.go`**. Parser kamu akan otomatis terdaftar menggunakan fungsi `init()`.

### Skenario 1: Situs Menggunakan Tema Standar (Contoh: MangaThemesia)
Jika situs menggunakan tema WordPress MangaThemesia (seperti Kiryuu, WestManga, Komikcast), kodenya sangat singkat karena cukup mewarisi (*embed*) `themes.MangaThemesia`:

```go
package parsers

import (
	"manga-downloader/themes"
)

// 1. Definisikan struct dengan meng-embed base theme
type Kiryuu struct {
	themes.MangaThemesia
}

// 2. Constructor
func NewKiryuu() *Kiryuu {
	return &Kiryuu{
		MangaThemesia: themes.NewMangaThemesia(),
	}
}

// 3. Nama resmi parser
func (k *Kiryuu) Name() string {
	return "Kiryuu (MangaThemesia)"
}

// 4. Daftar domain yang didukung (termasuk domain alternatif/mirror)
func (k *Kiryuu) Domains() []string {
	return []string{
		"kiryuu01.com",
		"kiryuu.id",
		"kiryuu.org",
	}
}

// 5. Logika ekstraksi (cukup panggil ExtractImages bawaan theme)
func (k *Kiryuu) GetImages(chapterURL string) ([]string, error) {
	return k.ExtractImages(chapterURL)
}

// 6. WAJIB: Daftarkan parser secara otomatis ke sistem
func init() {
	Register(NewKiryuu())
}
```

---

### Skenario 2: Situs Memiliki Struktur Kustom (Bukan Tema Standar)
Jika situs tidak menggunakan tema CMS standar (misal: Komiku), tulis logika parsing HTML sendiri:

```go
package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/themes"
)

type ContohKustom struct{}

func NewContohKustom() *ContohKustom {
	return &ContohKustom{}
}

func (c *ContohKustom) Name() string {
	return "Contoh Kustom"
}

func (c *ContohKustom) Domains() []string {
	return []string{"contohsitus.com"}
}

func (c *ContohKustom) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	// 1. Fetch HTML dengan helper bawaan
	doc, _, err := themes.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	})
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	// 2. Cari gambar dengan selector CSS situs tersebut
	doc.Find("div.chapter-content img").Each(func(i int, s *goquery.Selection) {
		imgURL, exists := s.Attr("src")
		if !exists || strings.TrimSpace(imgURL) == "" {
			imgURL, _ = s.Attr("data-src")
		}

		// Bersihkan query resize & URL relatif
		imgURL = themes.CleanImageURL(imgURL, parsedURL.Host)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan")
	}

	return images, nil
}

// WAJIB: Auto-register
func init() {
	Register(NewContohKustom())
}
```

---

## 🧪 Perintah Testing yang Wajib Dijalankan

Sebelum mengirimkan kontribusi atau commit, **kamu wajib menguji parser baru tersebut**.

### 1. Test Diagnosa URL (Direkomendasikan)
Gunakan salah satu link chapter asli dari situs yang baru kamu tambahkan untuk memastikan parser mendeteksi gambar dan tidak diblokir:

```powershell
go run . -t "https://situs-target.com/chapter-1/"
```
*atau via `go test`:*
```powershell
go test -v . -args "https://situs-target.com/chapter-1/"
```

**Hasil yang diharapkan:**
```text
[✓ 1/4] Domain Match      : Dikenali sebagai 'Nama Parser'
[✓ 2/4] Koneksi HTTP       : 200 OK (Akses aman, tidak terblokir)
[✓ 3/4] Ekstraksi Gambar  : Ditemukan X gambar
[✓ 4/4] Sampel Gambar     : https://.../001.webp
[SUKSES] Parser 'Nama Parser' BERFUNGSI SEMPURNA!
```

### 2. Test Seluruh Unit Test (Integrity Check)
Jalankan perintah ini untuk memastikan tidak ada syntax error atau tabrakan domain:
```powershell
go test ./...
```
Pastikan menghasilkan status **`ok`** / **`PASS`**.

---

## ✅ Checklist Sebelum Mengajukan Pull Request

- [ ] File parser ditaruh di folder `parsers/` dengan nama yang jelas (contoh: `parsers/kiryuu.go`).
- [ ] Terdapat blok `func init() { Register(NewNamaParser()) }` di bagian bawah file.
- [ ] Tidak mengubah atau mengedit file `parsers/registry.go`.
- [ ] Telah diuji dengan `go run . -t <url_chapter>` dan menghasilkan status **SUKSES**.
- [ ] Perintah `go test ./...` lulus tanpa error.
- [ ] Tidak menyertakan file binary (`*.exe`) atau gambar komik hasil download ke dalam commit Git.
