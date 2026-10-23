package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReleaseAssetInfo holds details about assets in a GitHub Release
type ReleaseAssetInfo struct {
	TagName     string
	BinaryURL   string
	BinaryName  string
	ChecksumURL string
}

// GetPlatformAssetNames returns the expected release binary filename and checksums filename for the current platform
func GetPlatformAssetNames() (binaryName, checksumsName string, err error) {
	checksumsName = "SHA256SUMS.txt"
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "Dmanga_windows_amd64.exe", checksumsName, nil
		case "arm64":
			return "Dmanga_windows_arm64.exe", checksumsName, nil
		default:
			return "", "", fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "Dmanga_linux_amd64", checksumsName, nil
		case "arm64":
			return "Dmanga_linux_arm64", checksumsName, nil
		default:
			return "", "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// FetchReleaseAssets queries the GitHub Releases API and locates asset download URLs
func FetchReleaseAssets(apiURL string, targetBinary string) (*ReleaseAssetInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Dmanga-CLI-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse release metadata: %w", err)
	}

	info := &ReleaseAssetInfo{
		TagName:    strings.TrimSpace(data.TagName),
		BinaryName: targetBinary,
	}

	for _, a := range data.Assets {
		if strings.EqualFold(a.Name, targetBinary) {
			info.BinaryURL = a.BrowserDownloadURL
		} else if strings.EqualFold(a.Name, "SHA256SUMS.txt") || strings.EqualFold(a.Name, "checksums.txt") {
			info.ChecksumURL = a.BrowserDownloadURL
		}
	}

	if info.BinaryURL == "" {
		return nil, fmt.Errorf("no matching binary asset '%s' found in release %s", targetBinary, info.TagName)
	}

	return info, nil
}

// ParseExpectedChecksum extracts the SHA256 checksum for targetFileName from SHA256SUMS.txt content
func ParseExpectedChecksum(content, targetFileName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := fields[0]
			name := filepath.Base(fields[1])
			if strings.EqualFold(name, targetFileName) {
				return hash, nil
			}
		}
	}
	return "", fmt.Errorf("checksum for '%s' not found in SHA256SUMS.txt", targetFileName)
}

// DownloadWithChecksum downloads a file, calculates its SHA256, and verifies against expectedHash
func DownloadWithChecksum(downloadURL, destPath, expectedHash string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Dmanga-CLI-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	// Stream file to disk and calculate hash simultaneously
	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("error writing downloaded file: %w", err)
	}

	if expectedHash != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualHash, expectedHash) {
			return fmt.Errorf("checksum mismatch!\nExpected : %s\nActual   : %s", expectedHash, actualHash)
		}
	}

	return nil
}

// ExecuteSelfUpdate performs the complete self-update flow
func ExecuteSelfUpdate() {
	targetBinary, _, err := GetPlatformAssetNames()
	if err != nil {
		fmt.Printf("[!] Unsupported platform for auto-update: %v\n", err)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("[!] Failed to locate running Dmanga binary: %v\n", err)
		return
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil && resolved != "" {
		exePath = resolved
	}

	installDir := filepath.Dir(exePath)

	// 1. Validate write permission in the installation directory
	testFile := filepath.Join(installDir, fmt.Sprintf(".dmanga_perm_test_%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		fmt.Printf("\n[!] Permission denied writing to '%s'\n", installDir)
		if runtime.GOOS == "windows" {
			fmt.Println("[*] Please run Command Prompt or PowerShell as Administrator to update.")
		} else {
			fmt.Println("[*] Please run 'sudo Dmanga update' or verify directory permissions.")
		}
		return
	}
	_ = os.Remove(testFile)

	fmt.Printf("[*] Checking for updates (current version: %s)...\n", Version)
	releaseInfo, err := FetchReleaseAssets(ReleaseAPIURL, targetBinary)
	if err != nil {
		fmt.Printf("[!] Update check failed: %v\n", err)
		return
	}

	if !IsUpdateAvailable(Version, releaseInfo.TagName) {
		fmt.Printf("[OK] Dmanga is already up to date (%s).\n", Version)
		return
	}

	fmt.Printf("[+] New version available: %s -> %s\n", Version, releaseInfo.TagName)

	// 2. Fetch expected checksum if available
	expectedHash := ""
	if releaseInfo.ChecksumURL != "" {
		fmt.Println("[*] Fetching SHA256 checksums...")
		client := &http.Client{Timeout: 10 * time.Second}
		if cResp, err := client.Get(releaseInfo.ChecksumURL); err == nil && cResp.StatusCode == http.StatusOK {
			defer cResp.Body.Close()
			if cBytes, err := io.ReadAll(cResp.Body); err == nil {
				if hash, err := ParseExpectedChecksum(string(cBytes), targetBinary); err == nil {
					expectedHash = hash
				}
			}
		}
	}

	// 3. Download new binary to temporary file
	tempFile := exePath + ".tmp"
	defer func() { _ = os.Remove(tempFile) }()

	fmt.Printf("[*] Downloading %s...\n", releaseInfo.BinaryName)
	if err := DownloadWithChecksum(releaseInfo.BinaryURL, tempFile, expectedHash); err != nil {
		fmt.Printf("[!] Download failed: %v\n", err)
		return
	}

	if expectedHash != "" {
		fmt.Println("[*] Checksum verified successfully [OK]")
	}

	// 4. Safe swap binary
	fmt.Printf("[*] Installing new binary to %s...\n", exePath)
	if runtime.GOOS == "windows" {
		oldFile := exePath + ".old"
		_ = os.Remove(oldFile) // Clean any lingering old file

		if err := os.Rename(exePath, oldFile); err != nil {
			fmt.Printf("[!] Failed to rename current binary: %v\n", err)
			return
		}

		if err := os.Rename(tempFile, exePath); err != nil {
			// Rollback immediately
			_ = os.Rename(oldFile, exePath)
			fmt.Printf("[!] Failed to swap new binary: %v (rolled back to previous version)\n", err)
			return
		}

		// Attempt cleanup of old file
		_ = os.Remove(oldFile)
	} else {
		// Linux: set executable bit and atomic rename
		_ = os.Chmod(tempFile, 0755)
		if err := os.Rename(tempFile, exePath); err != nil {
			fmt.Printf("[!] Failed to install new binary: %v\n", err)
			return
		}
	}

	// Update local cache
	_ = SaveReleaseCache(GetCacheFilePath(), &ReleaseCache{
		CheckedAt:     time.Now(),
		LatestVersion: releaseInfo.TagName,
		ReleaseURL:    fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", RepoOwner, RepoName, releaseInfo.TagName),
	})

	fmt.Printf("\n[OK] Dmanga has been successfully updated to %s!\n\n", releaseInfo.TagName)
}
