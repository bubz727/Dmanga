package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dmanga/internal/downloader"
)

func TestWriteImageFileAtomic(t *testing.T) {
	dir := t.TempDir()
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x01, 0x02}

	name, err := downloader.WriteImageFile(dir, "001", ".jpg", data)
	if err != nil {
		t.Fatalf("WriteImageFile failed: %v", err)
	}
	if name != "001.jpg" {
		t.Errorf("Expected file name '001.jpg', got %q", name)
	}

	got, err := os.ReadFile(filepath.Join(dir, "001.jpg"))
	if err != nil || len(got) != len(data) {
		t.Fatalf("Final file missing or wrong size: err=%v len=%d", err, len(got))
	}
	if _, err := os.Stat(filepath.Join(dir, "001.jpg.part")); !os.IsNotExist(err) {
		t.Errorf("Temporary .part file should not remain after write")
	}

	// Overwriting an existing file must also succeed
	if _, err := downloader.WriteImageFile(dir, "001", ".jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0}); err != nil {
		t.Errorf("Overwrite failed: %v", err)
	}
}

func TestDownloadFileSuccessAndHTTPError(t *testing.T) {
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, make([]byte, 2048)...)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.jpg" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(jpegBytes)
	}))
	defer server.Close()

	dir := t.TempDir()

	// 1. Success: file is saved, size reported, no .part left behind
	name, size, err := downloader.DownloadFile(server.Client(), server.URL+"/ok.jpg", dir, "001", ".jpg", "example.com", nil)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	if name != "001.jpg" || size != int64(len(jpegBytes)) {
		t.Errorf("Unexpected result: name=%q size=%d (expected 001.jpg, %d)", name, size, len(jpegBytes))
	}
	if _, err := os.Stat(filepath.Join(dir, "001.jpg.part")); !os.IsNotExist(err) {
		t.Errorf("Temporary .part file should not remain")
	}

	// 2. HTTP 404: error returned and no file written
	_, _, err = downloader.DownloadFile(server.Client(), server.URL+"/missing.jpg", dir, "002", ".jpg", "example.com", nil)
	if err == nil {
		t.Errorf("Expected error for HTTP 404, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "002.jpg")); !os.IsNotExist(statErr) {
		t.Errorf("No file should be written on HTTP error")
	}
}
