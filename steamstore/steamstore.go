package steamstore

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/k64z/steamstacks/steamid"
)

type Store struct {
	httpClient *http.Client
	sessionID  string
	SteamID    steamid.SteamID
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

func New(opts ...Option) (*Store, error) {
	var cfg config
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	sessionID, err := extractSessionID(httpClient.Jar)
	if err != nil {
		return nil, fmt.Errorf("extract sessionID: %w", err)
	}

	steamID, err := extractSteamID(httpClient.Jar)
	if err != nil {
		return nil, fmt.Errorf("extract steamID: %w", err)
	}

	return &Store{
		httpClient: httpClient,
		sessionID:  sessionID,
		SteamID:    steamID,
	}, nil
}

func extractSessionID(jar http.CookieJar) (string, error) {
	u, _ := url.Parse("https://store.steampowered.com")
	cookies := jar.Cookies(u)

	for _, cookie := range cookies {
		if cookie.Name == "sessionid" {
			return cookie.Value, nil
		}
	}

	return "", errors.New("sessionID is missing")
}

func extractSteamID(jar http.CookieJar) (steamid.SteamID, error) {
	u, _ := url.Parse("https://store.steampowered.com")
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

// DoRequest executes an arbitrary HTTP request using the Store's httpClient
func (s *Store) DoRequest(req *http.Request) (*http.Response, error) {
	return s.httpClient.Do(req)
}
