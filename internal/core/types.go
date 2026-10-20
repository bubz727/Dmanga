package core

// Chapter represents a manga chapter with its title and image URLs
type Chapter struct {
	Title  string
	Images []string
}

// Parser is the primary interface implemented by every website parser
type Parser interface {
	Name() string
	Domains() []string
	GetImages(chapterURL string) ([]string, error)
}

// ChapterParser is an optional interface implemented by parsers that extract both chapter title and image URLs
type ChapterParser interface {
	GetChapter(chapterURL string) (*Chapter, error)
}

// ImageProcessor allows a parser to process or descramble raw image bytes before saving them to disk
type ImageProcessor interface {
	ProcessImage(rawBytes []byte, imageURL string) ([]byte, error)
}
