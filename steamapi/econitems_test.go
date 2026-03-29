package steamapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSchemaItems_URLAndAPIKeyAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/IEconItems_440/GetSchemaItems/v1/"; got != want {
			t.Errorf("path = %q; want %q", got, want)
		}
		q := r.URL.Query()
		if got, want := q.Get("key"), "TESTKEY"; got != want {
			t.Errorf("key = %q; want %q", got, want)
		}
		if q.Get("access_token") != "" {
			t.Error("access_token should not be set when API key is present")
		}
		if got, want := q.Get("start"), "100"; got != want {
			t.Errorf("start = %q; want %q", got, want)
		}
		if got, want := q.Get("language"), "en"; got != want {
			t.Errorf("language = %q; want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL), WithAPIKey("TESTKEY"))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("TOKENVAL")

	body, err := api.GetSchemaItems(context.Background(), 440, GetSchemaItemsOptions{
		Start:    100,
		Language: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	if got := string(data); got != `{"result":{}}` {
		t.Errorf("body = %q; want %q", got, `{"result":{}}`)
	}
}

func TestGetSchemaItems_AccessTokenFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("key") != "" {
			t.Error("key should not be set when no API key configured")
		}
		if got, want := q.Get("access_token"), "MYTOKEN"; got != want {
			t.Errorf("access_token = %q; want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("MYTOKEN")

	body, err := api.GetSchemaItems(context.Background(), 440, GetSchemaItemsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
}

func TestGetSchemaItems_NoAuthError(t *testing.T) {
	api, err := New()
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.GetSchemaItems(context.Background(), 440, GetSchemaItemsOptions{})
	if err == nil {
		t.Fatal("expected error when no auth is configured")
	}
}

func TestGetSchemaItems_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Access Denied"))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL), WithAPIKey("KEY"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.GetSchemaItems(context.Background(), 440, GetSchemaItemsOptions{})
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %q; want it to contain status code 403", err.Error())
	}
	if !strings.Contains(err.Error(), "Access Denied") {
		t.Errorf("error = %q; want it to contain body snippet", err.Error())
	}
}

func TestGetSchemaItems_ZeroValueOmitsParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("start") != "" {
			t.Errorf("start should be omitted for zero value; got %q", q.Get("start"))
		}
		if q.Get("language") != "" {
			t.Errorf("language should be omitted for empty value; got %q", q.Get("language"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL), WithAPIKey("KEY"))
	if err != nil {
		t.Fatal(err)
	}

	body, err := api.GetSchemaItems(context.Background(), 440, GetSchemaItemsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
}

func TestGetSchemaItems_GenericAppID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/IEconItems_730/GetSchemaItems/v1/"; got != want {
			t.Errorf("path = %q; want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL), WithAPIKey("KEY"))
	if err != nil {
		t.Fatal(err)
	}

	body, err := api.GetSchemaItems(context.Background(), 730, GetSchemaItemsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
}
