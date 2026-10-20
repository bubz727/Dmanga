package downloader

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteImageFile writes the processed image byte slice to disk with the target baseName and extension
func WriteImageFile(targetDir, baseName, defaultExt string, bodyBytes []byte) (string, error) {
	fileName := fmt.Sprintf("%s%s", baseName, defaultExt)
	destPath := filepath.Join(targetDir, fileName)

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := out.Write(bodyBytes); err != nil {
		return "", err
	}

	return fileName, nil
}
