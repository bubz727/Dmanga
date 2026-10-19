package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// HostSession stores session data (User-Agent and cookies) for a domain
type HostSession struct {
	UserAgent string            `json:"userAgent"`
	Cookies   map[string]string `json:"cookies"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// CookieHeader returns the cookies formatted as a Cookie header string
func (s *HostSession) CookieHeader() string {
	if s == nil || len(s.Cookies) == 0 {
		return ""
	}
	parts := make([]string, 0, len(s.Cookies))
	for k, v := range s.Cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, "; ")
}

// SessionStore manages persisting sessions per host
type SessionStore struct {
	sync.RWMutex
	filePath string
	sessions map[string]*HostSession
}

var globalStore *SessionStore
var storeOnce sync.Once

// Attempt tracking to prevent endless retry loops on the same domain
var (
	attemptMu      sync.Mutex
	attemptedHosts = make(map[string]bool)
)

// HasAttemptedSolve returns whether a domain has already attempted verification in this session
func HasAttemptedSolve(host string) bool {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	return attemptedHosts[strings.ToLower(host)]
}

// MarkAttemptedSolve records that a domain verification has been attempted
func MarkAttemptedSolve(host string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	attemptedHosts[strings.ToLower(host)] = true
}

// ResetAttemptedSolve resets verification attempt tracking for a domain
func ResetAttemptedSolve(host string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	delete(attemptedHosts, strings.ToLower(host))
}

// GetSessionStore returns the singleton session store
func GetSessionStore() *SessionStore {
	storeOnce.Do(func() {
		storePath := getStoreFilePath()
		globalStore = &SessionStore{
			filePath: storePath,
			sessions: make(map[string]*HostSession),
		}
		globalStore.load()
	})
	return globalStore
}

func getStoreFilePath() string {
	if _, err := os.Stat(".cf_cookies.json"); err == nil {
		return ".cf_cookies.json"
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		p := filepath.Join(dir, ".cf_cookies.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".dmanga_cookies.json")
	}
	return ".cf_cookies.json"
}

func (s *SessionStore) load() {
	if s.filePath == "" {
		return
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var loaded map[string]*HostSession
	if err := json.Unmarshal(data, &loaded); err == nil {
		s.sessions = loaded
	}
}

func (s *SessionStore) save() {
	if s.filePath == "" {
		return
	}
	data, err := json.MarshalIndent(s.sessions, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filePath, data, 0600)
}

// GetSession returns the cached session for the specified host
func GetSession(host string) *HostSession {
	store := GetSessionStore()
	store.RLock()
	defer store.RUnlock()

	cleanHost := strings.ToLower(host)
	if s, exists := store.sessions[cleanHost]; exists {
		return s
	}

	for d, s := range store.sessions {
		if strings.HasSuffix(cleanHost, d) || strings.HasSuffix(d, cleanHost) {
			return s
		}
	}
	return nil
}

// SaveSession saves session cookies and User-Agent for a host
func SaveSession(host, userAgent string, cookies map[string]string) {
	store := GetSessionStore()
	store.Lock()
	defer store.Unlock()

	cleanHost := strings.ToLower(host)
	store.sessions[cleanHost] = &HostSession{
		UserAgent: userAgent,
		Cookies:   cookies,
		UpdatedAt: time.Now(),
	}
	store.save()
}

// ApplySessionHeaders attaches cached cookies and User-Agent to an http.Request
func ApplySessionHeaders(req *http.Request, host string) {
	session := GetSession(host)
	if session != nil {
		if session.UserAgent != "" {
			req.Header.Set("User-Agent", session.UserAgent)
		}
		if cookieStr := session.CookieHeader(); cookieStr != "" {
			existing := req.Header.Get("Cookie")
			if existing != "" {
				req.Header.Set("Cookie", existing+"; "+cookieStr)
			} else {
				req.Header.Set("Cookie", cookieStr)
			}
		}
	}
}

// IsCloudflareChallenge inspects HTTP status code, headers, and body for Cloudflare challenge markers
func IsCloudflareChallenge(statusCode int, body string, header http.Header) bool {
	if statusCode != http.StatusForbidden && statusCode != http.StatusServiceUnavailable {
		return false
	}

	server := strings.ToLower(header.Get("Server"))
	if strings.Contains(server, "cloudflare") {
		return true
	}

	bodyLower := strings.ToLower(body)
	markers := []string{
		"just a moment",
		"cf-browser-verification",
		"challenge-running",
		"turnstile",
		"cf-chl-widget",
		"ray id:",
		"attention required",
	}

	for _, m := range markers {
		if strings.Contains(bodyLower, m) {
			return true
		}
	}
	return false
}

// findBrowserPath locates an installed browser executable on Windows (Edge or Chrome)
func findBrowserPath() (string, error) {
	candidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates,
			filepath.Join(localAppData, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`),
		)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	if p, err := exec.LookPath("msedge"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chrome"); err == nil {
		return p, nil
	}

	return "", errors.New("no compatible browser (Microsoft Edge or Google Chrome) found")
}

func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// cdpClient provides lightweight JSON-RPC communication over WebSocket with Chromium DevTools Protocol
type cdpClient struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	nextID int64
}

func (c *cdpClient) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := atomic.AddInt64(&c.nextID, 1)
	req := map[string]interface{}{
		"id":     reqID,
		"method": method,
		"params": params,
	}

	if err := c.conn.WriteJSON(req); err != nil {
		return nil, err
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		var resp struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}

		if resp.ID == reqID {
			if resp.Error != nil {
				return nil, errors.New(resp.Error.Message)
			}
			return resp.Result, nil
		}
	}
}

func killProcessTree(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
}

// SolveCloudflare launches a visible browser window for the user to solve Cloudflare Turnstile,
// captures cf_clearance and session headers, and returns the rendered HTML.
func SolveCloudflare(targetURL string) (string, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	host := parsedURL.Host

	if HasAttemptedSolve(host) {
		return "", fmt.Errorf("Cloudflare challenge for '%s' was already attempted in this run. Halting to avoid endless loop", host)
	}
	MarkAttemptedSolve(host)

	browserPath, err := findBrowserPath()
	if err != nil {
		return "", err
	}

	port, err := getFreePort()
	if err != nil {
		port = 9222
	}

	tempDir, err := os.MkdirTemp("", "dmanga-edge-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary profile: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("[!] CLOUDFLARE CHALLENGE DETECTED!")
	fmt.Printf("[*] Domain: %s\n", host)
	fmt.Println("[*] Opening browser window to complete verification...")
	fmt.Println("[*] Please complete the verification checkbox on screen.")
	fmt.Println("==================================================")

	cmd := exec.Command(browserPath,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", tempDir),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		targetURL,
	)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to launch browser: %w", err)
	}

	processExited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(processExited)
	}()

	defer func() {
		if cmd.Process != nil {
			killProcessTree(cmd.Process.Pid)
		}
	}()

	// Wait for Chromium CDP HTTP server to be ready
	var wsURL string
	httpClient := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		resp, err := httpClient.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err != nil {
			continue
		}

		var targets []struct {
			Type                 string `json:"type"`
			URL                  string `json:"url"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&targets); err == nil {
			resp.Body.Close()
			for _, t := range targets {
				if t.Type == "page" && t.WebSocketDebuggerURL != "" {
					wsURL = t.WebSocketDebuggerURL
					break
				}
			}
		} else {
			resp.Body.Close()
		}

		if wsURL != "" {
			break
		}
	}

	if wsURL == "" {
		return "", errors.New("failed to connect to browser DevTools protocol")
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to open WebSocket to browser: %w", err)
	}
	defer conn.Close()

	client := &cdpClient{conn: conn}

	// Poll until solved or timeout (60s)
	solveTimeout := 60 * time.Second
	start := time.Now()
	var rawHTML string
	var solved bool
	lastStatusPrint := time.Now()

	for time.Since(start) < solveTimeout {
		select {
		case <-processExited:
			return "", errors.New("browser window was closed before Cloudflare verification completed")
		default:
		}

		time.Sleep(1 * time.Second)

		// Print progress reminder every 10 seconds
		if time.Since(lastStatusPrint) >= 10*time.Second {
			remaining := int((solveTimeout - time.Since(start)).Seconds())
			fmt.Printf("[*] Waiting for verification... (%ds remaining)\n", remaining)
			lastStatusPrint = time.Now()
		}

		// 1. Fetch current cookies for domain
		cookieRaw, err := client.call("Network.getCookies", map[string]interface{}{
			"urls": []string{targetURL, fmt.Sprintf("https://%s/", host)},
		})

		var cookieMap = make(map[string]string)
		hasClearance := false
		if err == nil {
			var cResp struct {
				Cookies []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"cookies"`
			}
			if err := json.Unmarshal(cookieRaw, &cResp); err == nil {
				for _, c := range cResp.Cookies {
					cookieMap[c.Name] = c.Value
					if c.Name == "cf_clearance" && len(c.Value) > 5 {
						hasClearance = true
					}
				}
			}
		}

		// Cloudflare Turnstile verification is ONLY complete once cf_clearance cookie is issued
		if hasClearance {
			fmt.Println("[*] Cloudflare clearance detected! Stabilizing session (4s)...")
			time.Sleep(4 * time.Second)

			// Fetch final rendered outerHTML
			htmlRaw, err := client.call("Runtime.evaluate", map[string]string{
				"expression": "document.documentElement.outerHTML",
			})
			if err == nil {
				var evalHTML struct {
					Result struct {
						Value string `json:"value"`
					} `json:"result"`
				}
				if err := json.Unmarshal(htmlRaw, &evalHTML); err == nil {
					rawHTML = evalHTML.Result.Value
				}
			}

			// Extract real browser User-Agent
			var ua string
			uaRaw, err := client.call("Runtime.evaluate", map[string]string{
				"expression": "navigator.userAgent",
			})
			if err == nil {
				var evalUA struct {
					Result struct {
						Value string `json:"value"`
					} `json:"result"`
				}
				if err := json.Unmarshal(uaRaw, &evalUA); err == nil {
					ua = evalUA.Result.Value
				}
			}

			// Fetch latest cookies once more after stabilization
			cookieRaw2, err := client.call("Network.getCookies", map[string]interface{}{
				"urls": []string{targetURL, fmt.Sprintf("https://%s/", host)},
			})
			if err == nil {
				var cResp2 struct {
					Cookies []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"cookies"`
				}
				if err := json.Unmarshal(cookieRaw2, &cResp2); err == nil {
					for _, c := range cResp2.Cookies {
						cookieMap[c.Name] = c.Value
					}
				}
			}

			SaveSession(host, ua, cookieMap)
			ResetAttemptedSolve(host)
			solved = true
			break
		}
	}

	if !solved {
		fmt.Println("==================================================")
		fmt.Println("[X] Cloudflare verification timed out!")
		fmt.Println("[!] Failed to verify within time limit.")
		fmt.Println("==================================================")
		return "", errors.New("timed out waiting for Cloudflare challenge completion")
	}

	fmt.Println("[OK] Cloudflare verification completed successfully!")
	fmt.Println("[OK] Session cookies saved. Resuming download...")
	fmt.Println("==================================================")
	fmt.Println()

	return rawHTML, nil
}
