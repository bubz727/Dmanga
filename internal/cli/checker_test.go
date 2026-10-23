package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseCacheSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dmanga-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cacheFile := filepath.Join(tempDir, "release_cache.json")

	now := time.Now().Truncate(time.Second)
	testCache := &ReleaseCache{
		CheckedAt:     now,
		LatestVersion: "v1.2.0",
		ReleaseURL:    "https://github.com/bubz727/Dmanga/releases/tag/v1.2.0",
	}

	if err := SaveReleaseCache(cacheFile, testCache); err != nil {
		t.Fatalf("SaveReleaseCache failed: %v", err)
	}

	loaded, err := LoadReleaseCache(cacheFile)
	if err != nil {
		t.Fatalf("LoadReleaseCache failed: %v", err)
	}

	if loaded.LatestVersion != "v1.2.0" {
		t.Errorf("Expected LatestVersion 'v1.2.0', got %q", loaded.LatestVersion)
	}
	if loaded.ReleaseURL != testCache.ReleaseURL {
		t.Errorf("Expected ReleaseURL %q, got %q", testCache.ReleaseURL, loaded.ReleaseURL)
	}
}

func TestFetchLatestReleaseMock(t *testing.T) {
	// 1. Success scenario
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("Missing expected Accept header")
		}
		resp := map[string]string{
			"tag_name": "v1.5.0",
			"html_url": "https://github.com/bubz727/Dmanga/releases/tag/v1.5.0",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tag, url, err := FetchLatestRelease(server.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("FetchLatestRelease failed: %v", err)
	}
	if tag != "v1.5.0" {
		t.Errorf("Expected tag 'v1.5.0', got %q", tag)
	}
	if url != "https://github.com/bubz727/Dmanga/releases/tag/v1.5.0" {
		t.Errorf("Unexpected URL: %s", url)
	}

	// 2. HTTP Error scenario
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Rate limit exceeded", http.StatusForbidden)
	}))
	defer errServer.Close()

	_, _, err = FetchLatestRelease(errServer.URL, 2*time.Second)
	if err == nil {
		t.Errorf("Expected error for HTTP 403, got nil")
	}

	// 3. Timeout scenario
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer slowServer.Close()

	_, _, err = FetchLatestRelease(slowServer.URL, 50*time.Millisecond)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}
