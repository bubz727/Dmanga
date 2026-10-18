package parsers

import (
	"manga-downloader/core"
	"manga-downloader/themes"
)

type Rawkuma struct {
	themes.Natsu
}

func NewRawkuma() *Rawkuma {
	return &Rawkuma{
		Natsu: themes.NewNatsu(),
	}
}

func (r *Rawkuma) Name() string {
	return "Rawkuma (Natsu)"
}

func (r *Rawkuma) Domains() []string {
	return []string{
		"rawkuma.net",
	}
}

func (r *Rawkuma) GetImages(chapterURL string) ([]string, error) {
	return r.ExtractImages(chapterURL)
}

func init() {
	core.Register(NewRawkuma())
}
