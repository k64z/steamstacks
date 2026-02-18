package steamsession

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

var hexPattern = regexp.MustCompile("^[0-9a-f]+$")

func TestMustGenerateSessionID(t *testing.T) {
	t.Run("validate format", func(t *testing.T) {
		sessionID := mustGenerateSessionID()

		if len(sessionID) != 24 {
			t.Errorf("want sessionID length of 24, got %d", len(sessionID))
		}

		if !hexPattern.MatchString(sessionID) {
			t.Errorf("sessionID contains non-hexadecimal characters: %s", sessionID)
		}
	})

	t.Run("check uniqueness", func(t *testing.T) {
		sessionIDs := make(map[string]bool)
		for range 1000 {
			id := mustGenerateSessionID()
			if sessionIDs[id] {
				t.Errorf("duplicate sessionID generated: %s", id)
			}
			sessionIDs[id] = true
		}
	})
}

// fakeJWT builds a minimal JWT with the given expiry for testing.
func fakeJWT(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"none"}`))
	claims, _ := json.Marshal(map[string]any{"exp": exp.Unix()})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return fmt.Sprintf("%s.%s.fakesig", header, payload)
}

func TestGetWebCookiesDirect(t *testing.T) {
	testCases := []struct {
		name         string
		platformType protocol.EAuthTokenPlatformType
	}{
		{"SteamClient", protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient},
		{"MobileApp", protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_MobileApp},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := fakeJWT(time.Now().Add(24 * time.Hour))

			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}

			s := &Session{
				httpClient:   &http.Client{Jar: jar},
				platformType: tc.platformType,
				SteamID:      steamid.FromSteamID64(76561198012345678),
				RefreshToken: "eyFakeRefresh456",
				AccessToken:  token,
			}

			if err := s.GetWebCookies(context.Background()); err != nil {
				t.Fatalf("GetWebCookies returned error: %v", err)
			}

			u, _ := url.Parse("https://steamcommunity.com")
			cookies := jar.Cookies(u)

			cookieMap := make(map[string]string, len(cookies))
			for _, c := range cookies {
				cookieMap[c.Name] = c.Value
			}

			assertCookie(t, cookieMap, "sessionid", "")
			assertCookie(t, cookieMap, "steamLoginSecure", "76561198012345678%7C%7C"+token)
		})
	}
}

func TestGetWebCookiesInstallsAuthTransport(t *testing.T) {
	t.Run("MobileApp gets authTransport", func(t *testing.T) {
		token := fakeJWT(time.Now().Add(24 * time.Hour))
		jar, _ := cookiejar.New(nil)

		s := &Session{
			httpClient:   &http.Client{Jar: jar},
			platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_MobileApp,
			SteamID:      steamid.FromSteamID64(76561198012345678),
			RefreshToken: "eyFakeRefresh456",
			AccessToken:  token,
		}

		if err := s.GetWebCookies(context.Background()); err != nil {
			t.Fatalf("GetWebCookies: %v", err)
		}

		if _, ok := s.httpClient.Transport.(*authTransport); !ok {
			t.Fatal("authTransport not installed for MobileApp after GetWebCookies")
		}
	})

	t.Run("SteamClient does not get authTransport", func(t *testing.T) {
		token := fakeJWT(time.Now().Add(24 * time.Hour))
		jar, _ := cookiejar.New(nil)

		s := &Session{
			httpClient:   &http.Client{Jar: jar},
			platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient,
			SteamID:      steamid.FromSteamID64(76561198012345678),
			RefreshToken: "eyFakeRefresh456",
			AccessToken:  token,
		}

		if err := s.GetWebCookies(context.Background()); err != nil {
			t.Fatalf("GetWebCookies: %v", err)
		}

		if _, ok := s.httpClient.Transport.(*authTransport); ok {
			t.Fatal("authTransport should NOT be installed for SteamClient after GetWebCookies")
		}
	})
}

func TestGetWebCookiesRejectsExpiredToken(t *testing.T) {
	expired := fakeJWT(time.Now().Add(-1 * time.Hour))

	jar, _ := cookiejar.New(nil)
	s := &Session{
		httpClient:   &http.Client{Jar: jar},
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient,
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  expired,
	}

	err := s.GetWebCookies(context.Background())
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if err.Error() != "access token has expired (re-login needed)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetWebCookiesRejectsMissingToken(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	s := &Session{
		httpClient:   &http.Client{Jar: jar},
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient,
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  "", // no access token
	}

	err := s.GetWebCookies(context.Background())
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
	if err.Error() != "access token is required for SteamClient/MobileApp (re-login needed)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthTransportViaGetWebCookies(t *testing.T) {
	accessToken := fakeJWT(time.Now().Add(24 * time.Hour))

	mux := http.NewServeMux()

	// FinalizeLogin: return transfer info pointing at /settoken.
	var tsURL string
	mux.HandleFunc("/jwt/finalizelogin", func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf(`{"steamID":"76561198012345678","transfer_info":[{"url":"%s/settoken","params":{"nonce":"n","auth":"opaque"}}]}`, tsURL)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	})

	// Transfer info endpoint: return 200 OK.
	mux.HandleFunc("/settoken", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()
	tsURL = ts.URL

	jar, _ := cookiejar.New(nil)
	httpClient := ts.Client()
	httpClient.Jar = jar

	s := &Session{
		httpClient:   httpClient,
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_WebBrowser,
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  accessToken, // set during login (PollAuthSessionStatus)
		loginURL:     ts.URL,
	}

	// GetWebCookies: FinalizeLogin → install authTransport using existing AccessToken
	if err := s.GetWebCookies(context.Background()); err != nil {
		t.Fatalf("GetWebCookies: %v", err)
	}

	at, ok := httpClient.Transport.(*authTransport)
	if !ok {
		t.Fatal("authTransport not installed after GetWebCookies")
	}

	// With a fresh 24h token, should NOT need refresh
	if at.needsRefresh() {
		t.Fatal("needsRefresh should be false with fresh token")
	}

	// Simulate near-expiry
	at.mu.Lock()
	at.tokenExpiry = time.Now().Add(-1 * time.Minute)
	at.mu.Unlock()

	if !at.needsRefresh() {
		t.Fatal("needsRefresh should be true with expired token")
	}
}

// hostRewriter is a test helper that rewrites the URL scheme+host to the
// target test server while preserving the original Host header and path.
// This lets authTransport see "steamcommunity.com" while the request
// actually goes to the local TLS test server.
type hostRewriter struct {
	base   http.RoundTripper
	target *url.URL
}

func (h *hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = h.target.Scheme
	req.URL.Host = h.target.Host
	return h.base.RoundTrip(req)
}

func TestAuthTransportProactiveRefresh(t *testing.T) {
	freshToken := fakeJWT(time.Now().Add(24 * time.Hour))
	var refreshCount atomic.Int32

	mux := http.NewServeMux()

	// Profile endpoint: always returns 200.
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// FinalizeLogin: return transfer info pointing at steamcommunity.com.
	// The hostRewriter will redirect these to the test server.
	mux.HandleFunc("/jwt/finalizelogin", func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		resp := `{"steamID":"76561198012345678","transfer_info":[{"url":"https://steamcommunity.com/settoken","params":{"nonce":"n","auth":"opaque"}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	})

	// Transfer info endpoint: set the steamLoginSecure cookie with the fresh token.
	mux.HandleFunc("/settoken", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "steamLoginSecure",
			Value: fmt.Sprintf("76561198012345678%%7C%%7C%s", freshToken),
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()
	tsURL, _ := url.Parse(ts.URL)

	jar, _ := cookiejar.New(nil)
	tlsClient := ts.Client()

	expiredToken := fakeJWT(time.Now().Add(-1 * time.Hour))
	s := &Session{
		httpClient:   &http.Client{Jar: jar},
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  expiredToken,
		sessionID:    "testsession123",
		loginURL:     ts.URL,
	}

	// Pre-populate cookies so the jar has something for patchRequestCookies.
	s.setSteamCommunityWebCookies()

	// Install authTransport with an already-expired tokenExpiry so it
	// triggers proactive refresh on the first request.
	at := &authTransport{
		base: &hostRewriter{
			base:   tlsClient.Transport,
			target: tsURL,
		},
		session:     s,
		tokenExpiry: time.Now().Add(-1 * time.Minute),
	}
	s.httpClient.Transport = at

	// Make a request to "steamcommunity.com" — authTransport should
	// proactively refresh via FinalizeLogin before forwarding.
	req, _ := http.NewRequest("GET", "https://steamcommunity.com/profile", nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want status 200, got %d", resp.StatusCode)
	}
	if refreshCount.Load() != 1 {
		t.Fatalf("want 1 FinalizeLogin call, got %d", refreshCount.Load())
	}
	if s.AccessToken != freshToken {
		t.Fatalf("AccessToken not updated after refresh")
	}

	// Verify the cookie jar was updated with the new token.
	u, _ := url.Parse("https://steamcommunity.com")
	for _, c := range jar.Cookies(u) {
		if c.Name == "steamLoginSecure" {
			want := fmt.Sprintf("76561198012345678%%7C%%7C%s", freshToken)
			if c.Value != want {
				t.Fatalf("steamLoginSecure = %q, want %q", c.Value, want)
			}
			return
		}
	}
	t.Fatal("steamLoginSecure cookie not found after refresh")
}

func TestAuthTransportReactiveRefresh(t *testing.T) {
	freshToken := fakeJWT(time.Now().Add(24 * time.Hour))
	var refreshCount atomic.Int32
	var profileCount atomic.Int32

	mux := http.NewServeMux()

	// Profile endpoint: first call returns 302 → /login, subsequent calls return 200.
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		n := profileCount.Add(1)
		if n == 1 {
			w.Header().Set("Location", "https://steamcommunity.com/login")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.Write([]byte("ok"))
	})

	// FinalizeLogin: return transfer info pointing at steamcommunity.com.
	mux.HandleFunc("/jwt/finalizelogin", func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		resp := `{"steamID":"76561198012345678","transfer_info":[{"url":"https://steamcommunity.com/settoken","params":{"nonce":"n","auth":"opaque"}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	})

	// Transfer info endpoint: set the steamLoginSecure cookie with the fresh token.
	mux.HandleFunc("/settoken", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "steamLoginSecure",
			Value: fmt.Sprintf("76561198012345678%%7C%%7C%s", freshToken),
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()
	tsURL, _ := url.Parse(ts.URL)

	jar, _ := cookiejar.New(nil)
	tlsClient := ts.Client()
	tlsClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Start with a valid (non-expired) token so proactive refresh is skipped.
	validToken := fakeJWT(time.Now().Add(24 * time.Hour))
	s := &Session{
		httpClient:   &http.Client{Jar: jar},
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  validToken,
		sessionID:    "testsession123",
		loginURL:     ts.URL,
	}

	s.setSteamCommunityWebCookies()

	// Install authTransport with a future tokenExpiry — no proactive refresh.
	at := &authTransport{
		base: &hostRewriter{
			base:   tlsClient.Transport,
			target: tsURL,
		},
		session:     s,
		tokenExpiry: time.Now().Add(24 * time.Hour),
	}
	s.httpClient.Transport = at

	req, _ := http.NewRequest("GET", "https://steamcommunity.com/profile", nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// The final response should be 200, not the 302.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want status 200, got %d", resp.StatusCode)
	}
	if refreshCount.Load() != 1 {
		t.Fatalf("want 1 FinalizeLogin call, got %d", refreshCount.Load())
	}
	if profileCount.Load() != 2 {
		t.Fatalf("want 2 profile requests (original + retry), got %d", profileCount.Load())
	}
	if s.AccessToken != freshToken {
		t.Fatalf("AccessToken not updated after reactive refresh")
	}
}

func TestAuthTransportProactiveRefreshMobileApp(t *testing.T) {
	freshToken := fakeJWT(time.Now().Add(24 * time.Hour))
	var refreshCount atomic.Int32

	mux := http.NewServeMux()

	// Profile endpoint: always returns 200.
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// GenerateAccessTokenForApp: return a protobuf response with the fresh token.
	mux.HandleFunc("/IAuthenticationService/GenerateAccessTokenForApp/v1", func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		resp := &protocol.CAuthentication_AccessToken_GenerateForApp_Response{
			AccessToken: &freshToken,
		}
		body, err := proto.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Eresult", "1")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	})

	ts := httptest.NewTLSServer(mux)
	defer ts.Close()
	tsURL, _ := url.Parse(ts.URL)

	jar, _ := cookiejar.New(nil)
	tlsClient := ts.Client()

	expiredToken := fakeJWT(time.Now().Add(-1 * time.Hour))

	// Create a steamapi.API pointing at the test server.
	api, err := steamapi.New(
		steamapi.WithHTTPClient(tlsClient),
		steamapi.WithBaseURL(ts.URL),
	)
	if err != nil {
		t.Fatalf("create steamapi: %v", err)
	}

	s := &Session{
		httpClient:   &http.Client{Jar: jar},
		steamAPI:     api,
		platformType: protocol.EAuthTokenPlatformType_k_EAuthTokenPlatformType_MobileApp,
		SteamID:      steamid.FromSteamID64(76561198012345678),
		RefreshToken: "eyFakeRefresh456",
		AccessToken:  expiredToken,
		sessionID:    "testsession123",
	}

	// Pre-populate cookies so the jar has something for patchRequestCookies.
	s.setSteamCommunityWebCookies()

	// Install authTransport with an already-expired tokenExpiry so it
	// triggers proactive refresh on the first request.
	at := &authTransport{
		base: &hostRewriter{
			base:   tlsClient.Transport,
			target: tsURL,
		},
		session:     s,
		tokenExpiry: time.Now().Add(-1 * time.Minute),
	}
	s.httpClient.Transport = at

	// Make a request to "steamcommunity.com" — authTransport should
	// proactively refresh via GenerateAccessTokenForApp before forwarding.
	req, _ := http.NewRequest("GET", "https://steamcommunity.com/profile", nil)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want status 200, got %d", resp.StatusCode)
	}
	if refreshCount.Load() != 1 {
		t.Fatalf("want 1 GenerateAccessTokenForApp call, got %d", refreshCount.Load())
	}
	if s.AccessToken != freshToken {
		t.Fatalf("AccessToken not updated after refresh")
	}

	// Verify tokenExpiry was updated to the fresh token's expiry.
	if at.needsRefresh() {
		t.Fatal("needsRefresh should be false after refresh with fresh 24h token")
	}

	// Verify the cookie jar was updated with the new token.
	u, _ := url.Parse("https://steamcommunity.com")
	for _, c := range jar.Cookies(u) {
		if c.Name == "steamLoginSecure" {
			want := fmt.Sprintf("76561198012345678%%7C%%7C%s", freshToken)
			if c.Value != want {
				t.Fatalf("steamLoginSecure = %q, want %q", c.Value, want)
			}
			return
		}
	}
	t.Fatal("steamLoginSecure cookie not found after refresh")
}

// assertCookie checks that a cookie exists and, if wantValue is non-empty, matches the expected value.
func assertCookie(t *testing.T, cookies map[string]string, name, wantValue string) {
	t.Helper()

	v, ok := cookies[name]
	if !ok {
		t.Fatalf("%s cookie not set", name)
	}
	if v == "" {
		t.Fatalf("%s cookie is empty", name)
	}
	if wantValue != "" && v != wantValue {
		t.Fatalf("%s = %q, want %q", name, v, wantValue)
	}
}
