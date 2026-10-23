package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RepoOwner         = "bubz727"
	RepoName          = "Dmanga"
	ReleaseAPIURL     = "https://api.github.com/repos/bubz727/Dmanga/releases/latest"
	CacheTTL          = 24 * time.Hour
	HTTPClientTimeout = 1500 * time.Millisecond
)

// ReleaseCache stores the cached result of the latest GitHub release check
type ReleaseCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url"`
}

// UpdateInfo holds details about an available newer version
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
}

// GetCacheFilePath returns the platform-specific path for release cache
func GetCacheFilePath() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "dmanga", "release_cache.json")
	}
	return filepath.Join(os.TempDir(), "dmanga_release_cache.json")
}

// LoadReleaseCache loads cached release info if present
func LoadReleaseCache(cachePath string) (*ReleaseCache, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var c ReleaseCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveReleaseCache persists release info to the cache file
func SaveReleaseCache(cachePath string, c *ReleaseCache) error {
	dir := filepath.Dir(cachePath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}

// FetchLatestRelease queries the GitHub Releases API with a strict timeout
func FetchLatestRelease(apiURL string, timeout time.Duration) (string, string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Dmanga-CLI")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", err
	}

	tag := strings.TrimSpace(data.TagName)
	if tag == "" {
		return "", "", fmt.Errorf("empty tag_name in release response")
	}

	url := strings.TrimSpace(data.HTMLURL)
	if url == "" {
		url = fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", RepoOwner, RepoName, tag)
	}

	return tag, url, nil
}

// CheckUpdateExplicit performs a synchronous, visible check for updates (used by --check-update)
func CheckUpdateExplicit() {
	fmt.Printf("[*] Checking for updates (current: %s)...\n", GetVersion())
	tag, releaseURL, err := FetchLatestRelease(ReleaseAPIURL, 3*time.Second)
	if err != nil {
		fmt.Printf("[!] Failed to check for updates: %v\n", err)
		return
	}

	cacheFile := GetCacheFilePath()
	_ = SaveReleaseCache(cacheFile, &ReleaseCache{
		CheckedAt:     time.Now(),
		LatestVersion: tag,
		ReleaseURL:    releaseURL,
	})

	if IsUpdateAvailable(Version, tag) {
		fmt.Printf("\n[+] A new version of Dmanga is available: %s (current: %s)\n", tag, GetVersion())
		fmt.Printf("[*] Release URL: %s\n", releaseURL)
		fmt.Printf("[*] Run 'Dmanga update' to update automatically.\n\n")
	} else {
		fmt.Printf("[OK] Dmanga is up to date (%s).\n\n", GetVersion())
	}
}

// StartPassiveUpdateCheck launches an asynchronous, non-blocking check.
// It sends an UpdateInfo pointer to the returned channel ONLY if an update is detected.
// The main process must NOT block waiting on this channel; it reads it only if ready on exit.
func StartPassiveUpdateCheck(skipCheck bool) <-chan *UpdateInfo {
	ch := make(chan *UpdateInfo, 1)
	if skipCheck || strings.EqualFold(strings.TrimSpace(Version), "dev") {
		return ch
	}

	go func() {
		cacheFile := GetCacheFilePath()
		cache, err := LoadReleaseCache(cacheFile)

		// 1. Check if cache is still fresh (< 24h)
		if err == nil && cache != nil && time.Since(cache.CheckedAt) < CacheTTL {
			if IsUpdateAvailable(Version, cache.LatestVersion) {
				ch <- &UpdateInfo{
					CurrentVersion: Version,
					LatestVersion:  cache.LatestVersion,
					ReleaseURL:     cache.ReleaseURL,
				}
			}
			return
		}

		// 2. Cache is expired or missing: fetch from GitHub with strict timeout (1.5s)
		tag, releaseURL, err := FetchLatestRelease(ReleaseAPIURL, HTTPClientTimeout)
		if err != nil {
			// Silent error: do not disrupt user experience
			return
		}

		_ = SaveReleaseCache(cacheFile, &ReleaseCache{
			CheckedAt:     time.Now(),
			LatestVersion: tag,
			ReleaseURL:    releaseURL,
		})

		if IsUpdateAvailable(Version, tag) {
			ch <- &UpdateInfo{
				CurrentVersion: Version,
				LatestVersion:  tag,
				ReleaseURL:     releaseURL,
			}
		}
	}()

	return ch
}

// PrintUpdateBanner prints a clean update notification banner at the end of execution
func PrintUpdateBanner(info *UpdateInfo) {
	if info == nil {
		return
	}
	fmt.Println()
	fmt.Printf("[!] A new version of Dmanga is available: %s -> %s\n", info.CurrentVersion, info.LatestVersion)
	fmt.Printf("    Release page : %s\n", info.ReleaseURL)
	fmt.Println("    Run 'Dmanga update' to install the latest version.")
	fmt.Println()
}
