package downloader

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteImageFile writes the processed image byte slice to disk with the target baseName and extension.
// Data is written to a temporary ".part" file first and then renamed, so an interrupted run never
// leaves a truncated image under its final name.
func WriteImageFile(targetDir, baseName, defaultExt string, bodyBytes []byte) (string, error) {
	fileName := fmt.Sprintf("%s%s", baseName, defaultExt)
	destPath := filepath.Join(targetDir, fileName)
	partPath := destPath + ".part"

	if err := os.WriteFile(partPath, bodyBytes, 0644); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}

	if err := os.Rename(partPath, destPath); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}

	return fileName, nil
}
