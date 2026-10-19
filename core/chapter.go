package core

// FetchChapter extracts chapter data (images and title).
// If the parser implements ChapterParser, it delegates to GetChapter.
// Otherwise, it calls GetImages and leaves Title empty.
func FetchChapter(p Parser, chapterURL string) (*Chapter, error) {
	if cp, ok := p.(ChapterParser); ok {
		chap, err := cp.GetChapter(chapterURL)
		if err != nil {
			return nil, err
		}
		if chap != nil {
			chap.Title = SanitizeFolderName(chap.Title)
			if len(chap.Images) == 0 {
				images, err := p.GetImages(chapterURL)
				if err != nil {
					return nil, err
				}
				chap.Images = images
			}
			return chap, nil
		}
	}

	images, err := p.GetImages(chapterURL)
	if err != nil {
		return nil, err
	}

	return &Chapter{
		Title:  "",
		Images: images,
	}, nil
}
