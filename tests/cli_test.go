package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dmanga/internal/cli"
)

// --- SemVer Tests ---

func TestSemVerComparison(t *testing.T) {
	tests := []struct {
		name           string
		current        string
		remote         string
		expectedCmp    int  // cli.CompareVersions(current, remote)
		expectedUpdate bool // cli.IsUpdateAvailable(current, remote)
	}{
		{
			name:           "1. Standard patch upgrade",
			current:        "v1.0.0",
			remote:         "v1.0.1",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "2. Crucial test: v1.9.0 vs v1.10.0 (lexicographical string fail check)",
			current:        "v1.9.0",
			remote:         "v1.10.0",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "3. Same version should have no update",
			current:        "v1.2.0",
			remote:         "v1.2.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
		{
			name:           "4. Current is newer than remote (e.g. local beta)",
			current:        "v2.0.0",
			remote:         "v1.9.0",
			expectedCmp:    1,
			expectedUpdate: false,
		},
		{
			name:           "5. Prerelease identifier: rc.2 vs rc.10",
			current:        "v1.0.0-rc.2",
			remote:         "v1.0.0-rc.10",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "6. Stable release is newer than prerelease",
			current:        "v1.0.0-rc.1",
			remote:         "v1.0.0",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "7. Alpha is older than Beta",
			current:        "v1.0.0-alpha",
			remote:         "v1.0.0-beta",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "8. Without leading 'v' prefix normalization",
			current:        "1.0.0",
			remote:         "v1.0.1",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "9. Dev build should not trigger standard update",
			current:        "dev",
			remote:         "v1.0.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
		{
			name:           "10. Invalid version strings handled safely",
			current:        "invalid-tag",
			remote:         "v1.0.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmp := cli.CompareVersions(tt.current, tt.remote)
			if gotCmp != tt.expectedCmp {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.current, tt.remote, gotCmp, tt.expectedCmp)
			}

			gotUpdate := cli.IsUpdateAvailable(tt.current, tt.remote)
			if gotUpdate != tt.expectedUpdate {
				t.Errorf("IsUpdateAvailable(%q, %q) = %v, expected %v", tt.current, tt.remote, gotUpdate, tt.expectedUpdate)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "v1.0.0"},
		{"v1.0.0", "v1.0.0"},
		{"V2.1.0", "V2.1.0"},
		{"   1.5.0  ", "v1.5.0"},
		{"", ""},
	}

	for _, tt := range tests {
		got := cli.NormalizeVersion(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeVersion(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetVersion(t *testing.T) {
	orig := cli.Version
	defer func() { cli.Version = orig }()

	cli.Version = "dev"
	if got := cli.GetVersion(); got != "dev" {
		t.Errorf("Expected 'dev', got %q", got)
	}

	cli.Version = "1.0.0"
	if got := cli.GetVersion(); got != "v1.0.0" {
		t.Errorf("Expected 'v1.0.0', got %q", got)
	}

	cli.Version = "v2.3.4"
	if got := cli.GetVersion(); got != "v2.3.4" {
		t.Errorf("Expected 'v2.3.4', got %q", got)
	}

	cli.Version = "  "
	if got := cli.GetVersion(); got != "dev" {
		t.Errorf("Expected 'dev', got %q", got)
	}
}

// --- Online Checker Tests ---

func TestReleaseCacheSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dmanga-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cacheFile := filepath.Join(tempDir, "release_cache.json")

	now := time.Now().Truncate(time.Second)
	testCache := &cli.ReleaseCache{
		CheckedAt:     now,
		LatestVersion: "v1.2.0",
		ReleaseURL:    "https://github.com/bubz727/Dmanga/releases/tag/v1.2.0",
	}

	if err := cli.SaveReleaseCache(cacheFile, testCache); err != nil {
		t.Fatalf("SaveReleaseCache failed: %v", err)
	}

	loaded, err := cli.LoadReleaseCache(cacheFile)
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

	tag, url, err := cli.FetchLatestRelease(server.URL, 2*time.Second)
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

	_, _, err = cli.FetchLatestRelease(errServer.URL, 2*time.Second)
	if err == nil {
		t.Errorf("Expected error for HTTP 403, got nil")
	}

	// 3. Timeout scenario
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer slowServer.Close()

	_, _, err = cli.FetchLatestRelease(slowServer.URL, 50*time.Millisecond)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}

// --- Self-Updater Tests ---

func TestGetPlatformAssetNames(t *testing.T) {
	binaryName, checksums, err := cli.GetPlatformAssetNames()
	if err != nil {
		t.Fatalf("GetPlatformAssetNames returned error: %v", err)
	}

	if checksums != "SHA256SUMS.txt" {
		t.Errorf("Expected checksums name 'SHA256SUMS.txt', got %q", checksums)
	}

	switch runtime.GOOS {
	case "windows":
		if binaryName != "Dmanga_windows_amd64.exe" && binaryName != "Dmanga_windows_arm64.exe" {
			t.Errorf("Unexpected Windows binary name: %q", binaryName)
		}
	case "linux":
		if binaryName != "Dmanga_linux_amd64" && binaryName != "Dmanga_linux_arm64" {
			t.Errorf("Unexpected Linux binary name: %q", binaryName)
		}
	}
}

func TestParseExpectedChecksum(t *testing.T) {
	mockChecksums := `
# Checksums for Dmanga v1.2.0
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  Dmanga_windows_amd64.exe
a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0  Dmanga_linux_amd64
9876543210fedcba0987654321fedcba0987654321fedcba0987654321fedcba  Dmanga_windows_arm64.exe
`

	// 1. Found scenario
	hash, err := cli.ParseExpectedChecksum(mockChecksums, "Dmanga_windows_amd64.exe")
	if err != nil {
		t.Fatalf("ParseExpectedChecksum failed: %v", err)
	}
	if hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Unexpected hash: %s", hash)
	}

	// 2. Case-insensitive lookup
	hashLinux, err := cli.ParseExpectedChecksum(mockChecksums, "dmanga_linux_amd64")
	if err != nil {
		t.Fatalf("ParseExpectedChecksum failed for lower case: %v", err)
	}
	if hashLinux != "a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0" {
		t.Errorf("Unexpected hash for linux: %s", hashLinux)
	}

	// 3. Not found scenario
	_, err = cli.ParseExpectedChecksum(mockChecksums, "nonexistent.exe")
	if err == nil {
		t.Errorf("Expected error for nonexistent file, got nil")
	}
}

func TestDownloadWithChecksumValidation(t *testing.T) {
	content := []byte("Hello Dmanga Binary Payload Content!")
	hasher := sha256.New()
	hasher.Write(content)
	correctHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "dmanga-update-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetFile := filepath.Join(tempDir, "test_binary.bin")

	// 1. Correct hash should succeed
	if err := cli.DownloadWithChecksum(server.URL, targetFile, correctHash); err != nil {
		t.Fatalf("Expected download to succeed with correct hash: %v", err)
	}

	// 2. Wrong hash should fail
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := cli.DownloadWithChecksum(server.URL, targetFile, wrongHash); err == nil {
		t.Fatalf("Expected error for mismatched checksum, got nil")
	}
}
