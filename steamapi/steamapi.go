package steamapi

import (
	"errors"
	"net/http"
)

type API struct {
	httpClient *http.Client
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

	return a, nil
}
