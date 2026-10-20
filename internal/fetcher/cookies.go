package fetcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

// MarkAttemptedSolve flags that verification has been attempted for this host
func MarkAttemptedSolve(host string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	attemptedHosts[strings.ToLower(host)] = true
}

// ResetAttemptedSolve clears the attempt flag for a domain upon successful solve
func ResetAttemptedSolve(host string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	delete(attemptedHosts, strings.ToLower(host))
}

// GetSessionStore returns the singleton SessionStore instance
func GetSessionStore() *SessionStore {
	storeOnce.Do(func() {
		globalStore = &SessionStore{
			filePath: findCookiePath(),
			sessions: make(map[string]*HostSession),
		}
		globalStore.load()
	})
	return globalStore
}

func findCookiePath() string {
	candidates := []string{
		".cf_cookies.json",
		"dmanga_cookies.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
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
