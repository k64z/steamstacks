package steamapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/k64z/steamstacks/steamid"
)

func TestGetPlayerSummaries(t *testing.T) {
	var seenSteamIDs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/ISteamUser/GetPlayerSummaries/v2/"; got != want {
			t.Errorf("path = %q; want %q", got, want)
		}
		q := r.URL.Query()
		if got, want := q.Get("access_token"), "TOKEN"; got != want {
			t.Errorf("access_token = %q; want %q", got, want)
		}
		seenSteamIDs = append(seenSteamIDs, q.Get("steamids"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"response": {
				"players": [
					{
						"steamid": "76561198111111111",
						"personaname": "Alice",
						"profileurl": "https://steamcommunity.com/id/alice/",
						"avatar": "https://avatars/a_s.jpg",
						"avatarmedium": "https://avatars/a_m.jpg",
						"avatarfull": "https://avatars/a_l.jpg",
						"avatarhash": "abc",
						"personastate": 1,
						"communityvisibilitystate": 3,
						"profilestate": 1,
						"lastlogoff": 1700000000,
						"timecreated": 1500000000,
						"gameid": "440",
						"gameextrainfo": "Team Fortress 2"
					},
					{
						"steamid": "76561198222222222",
						"personaname": "Bob",
						"personastate": 0
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("TOKEN")

	ids := []steamid.SteamID{
		steamid.FromSteamID64(76561198111111111),
		steamid.FromSteamID64(76561198222222222),
	}
	summaries, err := api.GetPlayerSummaries(context.Background(), ids)
	if err != nil {
		t.Fatalf("GetPlayerSummaries: %v", err)
	}

	if len(seenSteamIDs) != 1 {
		t.Fatalf("expected 1 batch call; got %d", len(seenSteamIDs))
	}
	if got, want := seenSteamIDs[0], "76561198111111111,76561198222222222"; got != want {
		t.Errorf("steamids = %q; want %q", got, want)
	}

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d; want 2", len(summaries))
	}
	alice := summaries[0]
	if alice.PersonaName != "Alice" {
		t.Errorf("alice.PersonaName = %q; want Alice", alice.PersonaName)
	}
	if alice.PersonaState != EPersonaStateOnline {
		t.Errorf("alice.PersonaState = %d; want %d", alice.PersonaState, EPersonaStateOnline)
	}
	if alice.AvatarLarge != "https://avatars/a_l.jpg" {
		t.Errorf("alice.AvatarLarge = %q", alice.AvatarLarge)
	}
	if alice.GameExtraInfo != "Team Fortress 2" {
		t.Errorf("alice.GameExtraInfo = %q; want Team Fortress 2", alice.GameExtraInfo)
	}
}

func TestGetPlayerSummaries_Batches(t *testing.T) {
	var batches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batches++
		ids := strings.Split(r.URL.Query().Get("steamids"), ",")
		if len(ids) > 100 {
			t.Errorf("batch size = %d; want <= 100", len(ids))
		}
		w.Write([]byte(`{"response":{"players":[]}}`))
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("TOKEN")

	// 250 ids → expect 3 batches (100, 100, 50)
	ids := make([]steamid.SteamID, 250)
	for i := range ids {
		ids[i] = steamid.FromSteamID64(uint64(76561198000000000 + i))
	}
	if _, err := api.GetPlayerSummaries(context.Background(), ids); err != nil {
		t.Fatalf("GetPlayerSummaries: %v", err)
	}
	if batches != 3 {
		t.Errorf("batches = %d; want 3", batches)
	}
}

func TestGetPlayerSummaries_Empty(t *testing.T) {
	api, err := New(WithBaseURL("http://unused"))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("TOKEN")

	out, err := api.GetPlayerSummaries(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetPlayerSummaries(nil): %v", err)
	}
	if out != nil {
		t.Errorf("out = %v; want nil for empty input", out)
	}
}

func TestGetPlayerSummaries_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer srv.Close()

	api, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	api.SetAccessToken("TOKEN")

	_, err = api.GetPlayerSummaries(context.Background(), []steamid.SteamID{steamid.FromSteamID64(76561198000000000)})
	if err == nil {
		t.Fatal("expected error on HTTP 403")
	}
}
