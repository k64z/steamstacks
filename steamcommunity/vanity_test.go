package steamcommunity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k64z/steamstacks/steamid"
)

func TestResolveVanityURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/id/sampleuser/"; got != want {
			t.Errorf("path = %q; want %q", got, want)
		}
		if got, want := r.URL.Query().Get("xml"), "1"; got != want {
			t.Errorf("xml = %q; want %q", got, want)
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
<profile><steamID64>76561198000000123</steamID64><steamID>SampleUser</steamID></profile>`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	sid, err := c.ResolveVanityURL(context.Background(), "sampleuser")
	if err != nil {
		t.Fatalf("ResolveVanityURL: %v", err)
	}
	if want := steamid.FromSteamID64(76561198000000123); sid != want {
		t.Errorf("sid = %d; want %d", sid, want)
	}
}

func TestResolveVanityURL_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Steam returns an error response wrapper when the vanity isn't found.
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<response><error>The specified profile could not be found.</error></response>`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	if _, err := c.ResolveVanityURL(context.Background(), "nonexistent"); !errors.Is(err, ErrVanityNotFound) {
		t.Fatalf("err = %v; want ErrVanityNotFound", err)
	}
}

func TestResolveVanityURL_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	if _, err := c.ResolveVanityURL(context.Background(), "anything"); err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}

func TestParseProfileInput(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantSID    steamid.SteamID
		wantVanity string
		wantErr    bool
	}{
		{
			name:    "raw steamid64",
			in:      "76561198000000123",
			wantSID: steamid.FromSteamID64(76561198000000123),
		},
		{
			name:    "profile URL with https",
			in:      "https://steamcommunity.com/profiles/76561198000000123",
			wantSID: steamid.FromSteamID64(76561198000000123),
		},
		{
			name:    "profile URL with trailing slash",
			in:      "steamcommunity.com/profiles/76561198000000123/",
			wantSID: steamid.FromSteamID64(76561198000000123),
		},
		{
			name:       "vanity URL with https",
			in:         "https://steamcommunity.com/id/SampleUser",
			wantVanity: "SampleUser",
		},
		{
			name:       "vanity URL no scheme",
			in:         "steamcommunity.com/id/some-name_42/",
			wantVanity: "some-name_42",
		},
		{
			name:       "bare vanity slug",
			in:         "SampleUser",
			wantVanity: "SampleUser",
		},
		{
			name:    "empty",
			in:      "",
			wantErr: true,
		},
		{
			name:    "junk with spaces",
			in:      "hello world",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sid, vanity, err := ParseProfileInput(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got sid=%d vanity=%q", sid, vanity)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sid != tc.wantSID {
				t.Errorf("sid = %d; want %d", sid, tc.wantSID)
			}
			if vanity != tc.wantVanity {
				t.Errorf("vanity = %q; want %q", vanity, tc.wantVanity)
			}
		})
	}
}
