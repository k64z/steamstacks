package steamcommunity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/k64z/steamstacks/steamid"
)

func TestGetMiniProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// AccountID for SteamID64 76561198000000000 is 39734272.
		if !strings.HasPrefix(r.URL.Path, "/miniprofile/39734272/") {
			t.Errorf("path = %q; want /miniprofile/39734272/...", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"persona_name": "Alice",
			"avatar_url": "https://avatars.example/abc.jpg",
			"level": 42,
			"in_game": {"name": "Team Fortress 2", "rich_presence": "Practicing"}
		}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	sid := steamid.FromSteamID64(76561198000000000)
	p, err := c.GetMiniProfile(context.Background(), sid)
	if err != nil {
		t.Fatalf("GetMiniProfile: %v", err)
	}
	if p.PersonaName != "Alice" {
		t.Errorf("PersonaName = %q; want Alice", p.PersonaName)
	}
	if p.AvatarURL != "https://avatars.example/abc.jpg" {
		t.Errorf("AvatarURL = %q", p.AvatarURL)
	}
	if p.Level != 42 {
		t.Errorf("Level = %d; want 42", p.Level)
	}
	if p.InGame == nil || p.InGame.Name != "Team Fortress 2" {
		t.Errorf("InGame = %+v; want TF2", p.InGame)
	}
}

func TestGetMiniProfiles_BatchAndFailureTolerance(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		// Fail the second-account requests; succeed others.
		if strings.Contains(r.URL.Path, "/miniprofile/40000001/") {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"persona_name":"ok","avatar_url":"x"}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	ids := []steamid.SteamID{
		steamid.FromSteamID64(76561197960265728 + 40000000),
		steamid.FromSteamID64(76561197960265728 + 40000001),
		steamid.FromSteamID64(76561197960265728 + 40000002),
	}
	out, err := c.GetMiniProfiles(context.Background(), ids)
	if err != nil {
		t.Fatalf("GetMiniProfiles: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("len(out) = %d; want 2 (one failure dropped)", len(out))
	}
	if got := count.Load(); got != 3 {
		t.Errorf("HTTP calls = %d; want 3", got)
	}
}

func TestGetMiniProfiles_CancelledContextNoPhantoms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"persona_name":"ok","avatar_url":"x"}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the batch is scheduled

	ids := make([]steamid.SteamID, 64)
	for i := range ids {
		ids[i] = steamid.FromSteamID64(uint64(76561198000000000 + i))
	}

	out, err := c.GetMiniProfiles(ctx, ids)

	// However the scheduler raced with the cancellation, the result must
	// never be a partial batch padded with phantom zero-value profiles:
	// the fix either reports the cancellation as an error (nil result) or
	// returns only real, non-zero-SteamID profiles.
	for _, p := range out {
		if p.SteamID == 0 {
			t.Fatalf("phantom zero-SteamID profile leaked into output: %+v", p)
		}
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v; want context.Canceled or nil", err)
	}
}
