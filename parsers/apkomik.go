package parsers

import (
	"manga-downloader/themes"
)

// ApKomik parser berbasis tema MangaThemesia
type ApKomik struct {
	themes.MangaThemesia
}

func NewApKomik() *ApKomik {
	return &ApKomik{
		MangaThemesia: themes.NewMangaThemesia(),
	}
}

func (a *ApKomik) Name() string {
	return "ApKomik (MangaThemesia)"
}

func (a *ApKomik) Domains() []string {
	return []string{
		"01.apkomik.com",
		"apkomik.com",
	}
}

func (a *ApKomik) GetImages(chapterURL string) ([]string, error) {
	return a.ExtractImages(chapterURL)
}
