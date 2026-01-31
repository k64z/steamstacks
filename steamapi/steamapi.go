package steamapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type API struct {
	httpClient  *http.Client
	accessToken string
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

func New(opts ...Option) (*API, error) {
	var cfg config
	for _, opt := range opts {
		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	a := &API{}

	if cfg.httpClient != nil {
		a.httpClient = cfg.httpClient
	} else {
		a.httpClient = http.DefaultClient
	}

	// Extract access token from cookie jar (if available)
	if a.httpClient.Jar != nil {
		a.accessToken, _ = extractAccessToken(a.httpClient.Jar)
	}

	return a, nil
}

// extractAccessToken extracts the access token from the steamLoginSecure cookie.
// The cookie format is "{steamid}||{access_token}" (URL encoded as "%7C%7C").
func extractAccessToken(jar http.CookieJar) (string, error) {
	u, _ := url.Parse("https://steamcommunity.com")
	cookies := jar.Cookies(u)

	for _, cookie := range cookies {
		if cookie.Name == "steamLoginSecure" {
			parts := strings.Split(cookie.Value, "%7C%7C") // URL encoded "||"
			if len(parts) < 2 {
				return "", errors.New("unsplittable steamLoginSecure cookie")
			}
			return parts[1], nil
		}
	}

	return "", errors.New("missing steamLoginSecure cookie")
}

// DoRequest executes an arbitrary HTTP request using the API's httpClient
func (a *API) DoRequest(req *http.Request) (*http.Response, error) {
	return a.httpClient.Do(req)
}
