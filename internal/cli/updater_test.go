package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetPlatformAssetNames(t *testing.T) {
	binaryName, checksums, err := GetPlatformAssetNames()
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
	hash, err := ParseExpectedChecksum(mockChecksums, "Dmanga_windows_amd64.exe")
	if err != nil {
		t.Fatalf("ParseExpectedChecksum failed: %v", err)
	}
	if hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Unexpected hash: %s", hash)
	}

	// 2. Case-insensitive lookup
	hashLinux, err := ParseExpectedChecksum(mockChecksums, "dmanga_linux_amd64")
	if err != nil {
		t.Fatalf("ParseExpectedChecksum failed for lower case: %v", err)
	}
	if hashLinux != "a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0" {
		t.Errorf("Unexpected hash for linux: %s", hashLinux)
	}

	// 3. Not found scenario
	_, err = ParseExpectedChecksum(mockChecksums, "nonexistent.exe")
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
	if err := DownloadWithChecksum(server.URL, targetFile, correctHash); err != nil {
		t.Fatalf("Expected download to succeed with correct hash: %v", err)
	}

	// 2. Wrong hash should fail
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := DownloadWithChecksum(server.URL, targetFile, wrongHash); err == nil {
		t.Fatalf("Expected error for mismatched checksum, got nil")
	}
}
