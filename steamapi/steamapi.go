package steamapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://api.steampowered.com"

type API struct {
	httpClient  *http.Client
	baseURL     string
	accessToken string
}

type config struct {
	httpClient *http.Client
	baseURL    string
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

func WithBaseURL(baseURL string) Option {
	return func(options *config) error {
		options.baseURL = baseURL
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

	a := &API{
		baseURL: defaultBaseURL,
	}

	if cfg.httpClient != nil {
		a.httpClient = cfg.httpClient
	} else {
		a.httpClient = http.DefaultClient
	}

	if cfg.baseURL != "" {
		a.baseURL = cfg.baseURL
	}

	return a, nil
}

// getAccessToken returns a fresh access token, preferring the cookie jar
// (which reflects any token refresh) and falling back to a manually-set token.
func (a *API) getAccessToken() (string, error) {
	if a.httpClient.Jar != nil {
		if token, err := extractAccessToken(a.httpClient.Jar); err == nil {
			return token, nil
		}
	}
	if a.accessToken != "" {
		return a.accessToken, nil
	}
	return "", errors.New("no access token available (no cookie jar or token set)")
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

// SetAccessToken sets the access token used to authenticate API requests.
func (a *API) SetAccessToken(token string) {
	a.accessToken = token
}

// DoRequest executes an arbitrary HTTP request using the API's httpClient
func (a *API) DoRequest(req *http.Request) (*http.Response, error) {
	return a.httpClient.Do(req)
}
