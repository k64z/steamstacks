package steamcommunity

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/k64z/steamstacks/steamid"
)

type Community struct {
	httpClient *http.Client
	sessionID  string
	steamID    steamid.SteamID
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

	var err error
	c.sessionID, err = extractSessionID(c.httpClient.Jar)
	if err != nil {
		return nil, fmt.Errorf("extract sessionID: %w", err)
	}

	c.steamID, err = extractSteamID(c.httpClient.Jar)
	if err != nil {
		return nil, fmt.Errorf("extract steamID: %w", err)
	}

	return c, nil
}

func extractSessionID(jar http.CookieJar) (string, error) {
	u, _ := url.Parse("https://steamcommunity.com")
	cookies := jar.Cookies(u)
	log.Printf("cookie: %+v", cookies)

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

	log.Printf("cookie: %+v", cookies)

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
