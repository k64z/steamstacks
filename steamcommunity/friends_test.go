package steamcommunity

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/k64z/steamstacks/steamid"
)

func newTestCommunity(t *testing.T, serverURL string) *Community {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}

	// Set cookies on both URLs so ensureInit finds them on steamcommunity.com.
	for _, raw := range []string{serverURL, "https://steamcommunity.com"} {
		u, _ := url.Parse(raw)
		jar.SetCookies(u, []*http.Cookie{
			{Name: "sessionid", Value: "test-session-id"},
			{Name: "steamLoginSecure", Value: "76561198000000000%7C%7Ctoken"},
		})
	}

	c, err := New(WithHTTPClient(&http.Client{Jar: jar}))
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	return c
}

func TestGetFriendsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/textfilter/ajaxgetfriendslist" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": 1,
			"friendslist": {
				"friends": [
					{"ulfriendid": "76561198111111111", "efriendrelationship": 3},
					{"ulfriendid": "76561198222222222", "efriendrelationship": 2}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	friends, err := c.GetFriendsList(context.Background())
	if err != nil {
		t.Fatalf("GetFriendsList: %v", err)
	}

	if got, want := len(friends), 2; got != want {
		t.Fatalf("len(friends) = %d; want %d", got, want)
	}

	sid1 := steamid.FromSteamID64(76561198111111111)
	if got, want := friends[sid1], EFriendRelationshipFriend; got != want {
		t.Errorf("friends[%d] = %d; want %d", sid1, got, want)
	}

	sid2 := steamid.FromSteamID64(76561198222222222)
	if got, want := friends[sid2], EFriendRelationshipRequestRecipient; got != want {
		t.Errorf("friends[%d] = %d; want %d", sid2, got, want)
	}
}

func TestGetFriendsList_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": 0}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	_, err := c.GetFriendsList(context.Background())
	if err == nil {
		t.Fatal("expected error for success=0")
	}
}

func TestAddFriend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/AddFriendAjax" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostFormValue("sessionID") != "test-session-id" {
			t.Errorf("sessionID = %q; want %q", r.PostFormValue("sessionID"), "test-session-id")
		}
		if r.PostFormValue("accept_invite") != "0" {
			t.Errorf("accept_invite = %q; want %q", r.PostFormValue("accept_invite"), "0")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.AddFriend(context.Background(), target); err != nil {
		t.Fatalf("AddFriend: %v", err)
	}
}

func TestAddFriend_NumericSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": 1}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.AddFriend(context.Background(), target); err != nil {
		t.Fatalf("AddFriend with numeric success: %v", err)
	}
}

func TestAddFriend_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": false}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.AddFriend(context.Background(), target); err == nil {
		t.Fatal("expected error for success=false")
	}
}

func TestAcceptFriendRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/AddFriendAjax" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostFormValue("accept_invite") != "1" {
			t.Errorf("accept_invite = %q; want %q", r.PostFormValue("accept_invite"), "1")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.AcceptFriendRequest(context.Background(), target); err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
}

func TestRemoveFriend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/RemoveFriendAjax" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.RemoveFriend(context.Background(), target); err != nil {
		t.Fatalf("RemoveFriend: %v", err)
	}
}

func TestBlockUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/BlockUserAjax" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.BlockUser(context.Background(), target); err != nil {
		t.Fatalf("BlockUser: %v", err)
	}
}

func TestUnblockUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/actions/BlockUserAjax" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostFormValue("sessionID") != "test-session-id" {
			t.Errorf("sessionID = %q; want %q", r.PostFormValue("sessionID"), "test-session-id")
		}
		if r.PostFormValue("block") != "0" {
			t.Errorf("block = %q; want %q", r.PostFormValue("block"), "0")
		}
		if r.PostFormValue("steamid") != "76561198333333333" {
			t.Errorf("steamid = %q; want %q", r.PostFormValue("steamid"), "76561198333333333")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.UnblockUser(context.Background(), target); err != nil {
		t.Fatalf("UnblockUser: %v", err)
	}
}

func TestUnblockUser_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.UnblockUser(context.Background(), target); err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestPostAction_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	target := steamid.FromSteamID64(76561198333333333)
	if err := c.RemoveFriend(context.Background(), target); err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func rewriteHostTransport(srv *httptest.Server) http.RoundTripper {
	return &rewriteTransport{server: srv, base: srv.Client().Transport}
}

type rewriteTransport struct {
	server *httptest.Server
	base   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	srvURL, _ := url.Parse(t.server.URL)
	req.URL.Scheme = srvURL.Scheme
	req.URL.Host = srvURL.Host
	return t.base.RoundTrip(req)
}
