package fetcher

import (
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	imageClientOnce sync.Once
	imageClient     *http.Client

	htmlClientOnce sync.Once
	htmlClient     *http.Client
)

// newTransport builds a tuned transport with a large idle pool per host and HTTP/2 enabled,
// so concurrent workers reuse connections instead of re-doing TCP and TLS handshakes.
func newTransport(disableCompression bool) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    disableCompression,
		ReadBufferSize:        64 * 1024,
		WriteBufferSize:       64 * 1024,
	}
}

// ImageClient returns the shared client used for image downloads.
// Compression is disabled because images are already compressed.
// No overall Client.Timeout is set so large images that keep progressing are not cut off;
// connection and header phases are bounded by the transport.
func ImageClient() *http.Client {
	imageClientOnce.Do(func() {
		imageClient = &http.Client{Transport: newTransport(true)}
	})
	return imageClient
}

// HTMLClient returns the shared client used for page and API fetches (compression enabled).
func HTMLClient() *http.Client {
	htmlClientOnce.Do(func() {
		htmlClient = &http.Client{
			Transport: newTransport(false),
			Timeout:   30 * time.Second,
		}
	})
	return htmlClient
}
