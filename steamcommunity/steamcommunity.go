package steamcommunity

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/k64z/steamstacks/steamid"
)

type Community struct {
	httpClient *http.Client
	// initMu guards lazy initialisation of sessionID + SteamID. Callers
	// that fan parallel goroutines into the same *Community (e.g.
	// fh-backend's friends module hitting three /friends/* pages
	// concurrently) would otherwise race on the first ensureInit.
	initMu    sync.Mutex
	sessionID string
	SteamID   steamid.SteamID
}

type config struct {
	httpClient *http.Client
}

type Option func(options *config) error

func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *config) error {
		if httpClient == nil {
			return errors.New("httpClient should be non-nil")
		}
		options.httpClient = httpClient
		return nil
	}
}

func New(opts ...Option) (*Community, error) {
	var cfg config
	for _, opt := range opts {
		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	c := &Community{}

	if cfg.httpClient != nil {
		c.httpClient = cfg.httpClient
	} else {
		c.httpClient = http.DefaultClient
	}

	return c, nil
}

// ensureInit lazily extracts session credentials from the cookie jar.
// It caches on success and retries on failure. Safe for concurrent
// callers — initMu serialises the first-time extraction.
func (c *Community) ensureInit() error {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	if c.sessionID != "" {
		return nil
	}
	sessionID, err := extractSessionID(c.httpClient.Jar)
	if err != nil {
		return fmt.Errorf("extract sessionID: %w", err)
	}
	steamID, err := extractSteamID(c.httpClient.Jar)
	if err != nil {
		return fmt.Errorf("extract steamID: %w", err)
	}
	c.sessionID = sessionID
	c.SteamID = steamID
	return nil
}

func extractSessionID(jar http.CookieJar) (string, error) {
	u, _ := url.Parse("https://steamcommunity.com")
	cookies := jar.Cookies(u)

	for _, cookie := range cookies {
		if cookie.Name == "sessionid" {
			return cookie.Value, nil
		}
	}

	return "", errors.New("sessionID is missing")
}

func extractSteamID(jar http.CookieJar) (steamid.SteamID, error) {
	u, _ := url.Parse("https://steamcommunity.com")
	cookies := jar.Cookies(u)

	for _, cookie := range cookies {
		if cookie.Name == "steamLoginSecure" {
			t := strings.Split(cookie.Value, "%7C%7C") // URL encoded "||"
			if len(t) < 2 {
				return steamid.SteamID(0), errors.New("unsplittable steamLoginSecure cookie")
			}

			sid, err := steamid.FromString(t[0])
			if err != nil {
				return steamid.SteamID(0), fmt.Errorf("parse SteamID: %w", err)
			}

			return sid, nil
		}
	}

	return steamid.SteamID(0), errors.New("missing steamLoginSecure cookie")
}

// DoRequest executes an arbitrary HTTP request using the Community's httpClient
func (c *Community) DoRequest(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
