package steamsession

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const tokenRefreshMargin = 5 * time.Minute

// refreshBypassKey is a context key used to signal authTransport to skip
// token refresh checks. This prevents recursive interception when
// FinalizeLogin makes requests to steamcommunity.com during the refresh path.
type refreshBypassKey struct{}

// authTransport is an http.RoundTripper that transparently refreshes
// the Steam access token in two ways:
//
//   - Proactive: refreshes before the JWT expires (based on the exp claim)
//   - Reactive: if Steam responds with a redirect to the login page
//     (e.g. server-side token revocation), refreshes and retries once
//
// The refresh strategy depends on the session's platform type:
//
//   - WebBrowser: re-establishes web cookies via FinalizeLogin (transfer info
//     flow). Requires a bypass context to prevent recursive interception.
//   - MobileApp: calls GenerateAccessTokenForApp (Steam Web API) to get a fresh
//     access token, then updates cookies via setSteamCommunityWebCookies.
//     No bypass context needed since the API call goes to api.steampowered.com.
//
// Only triggers for steamcommunity.com to avoid interfering with
// Steam Web API calls (which authenticate via protobuf body, not cookies)
// and to prevent recursive refresh loops.
type authTransport struct {
	base    http.RoundTripper
	session *Session

	mu          sync.Mutex
	tokenExpiry time.Time
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "steamcommunity.com" {
		return t.base.RoundTrip(req)
	}

	// Bypass auth checks during internal refresh operations — FinalizeLogin
	// submits transfer info to steamcommunity.com and must not trigger
	// recursive refresh attempts.
	if req.Context().Value(refreshBypassKey{}) != nil {
		return t.base.RoundTrip(req)
	}

	// Proactive: refresh before expiry
	if t.needsRefresh() {
		if err := t.refreshAndPatchRequest(req); err != nil {
			return nil, fmt.Errorf("auto-refresh access token: %w", err)
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Reactive: detect server-side token revocation (redirect to login page)
	// and retry once with a fresh token
	if isLoginRedirect(resp) {
		return t.retryAfterRefresh(req, resp)
	}

	return resp, nil
}

func (t *authTransport) needsRefresh() bool {
	return time.Now().Add(tokenRefreshMargin).After(t.tokenExpiry)
}

// refresh obtains a fresh access token and updates the cookie jar.
// The strategy depends on the session's platform type:
//   - WebBrowser: FinalizeLogin → extract token from jar
//   - MobileApp: GenerateAccessTokenForApp → setSteamCommunityWebCookies
func (t *authTransport) refresh(ctx context.Context) error {
	if t.session.platformType == PlatformTypeMobileApp {
		return t.refreshMobileApp(ctx)
	}
	return t.refreshWebBrowser(ctx)
}

// refreshWebBrowser re-establishes web cookies via FinalizeLogin and extracts
// the fresh access token from the cookie jar.
func (t *authTransport) refreshWebBrowser(ctx context.Context) error {
	// Use a bypass context so FinalizeLogin's requests to steamcommunity.com
	// (transfer info submission) don't trigger recursive refresh.
	bypassCtx := context.WithValue(ctx, refreshBypassKey{}, true)

	if err := t.session.FinalizeLogin(bypassCtx); err != nil {
		return err
	}

	// Extract the new access token set by the transfer info response.
	token, err := t.session.accessTokenFromJar()
	if err != nil {
		return fmt.Errorf("extract refreshed access token: %w", err)
	}
	t.session.AccessToken = token

	exp, err := jwtExpiry(token)
	if err != nil {
		return fmt.Errorf("parse token expiry: %w", err)
	}
	t.tokenExpiry = exp
	return nil
}

// refreshMobileApp uses GenerateAccessTokenForApp to get a fresh access token,
// then updates the cookie jar via setSteamCommunityWebCookies.
// No bypass context is needed since the API call goes to api.steampowered.com,
// not steamcommunity.com.
func (t *authTransport) refreshMobileApp(ctx context.Context) error {
	if err := t.session.refreshAccessToken(ctx); err != nil {
		return err
	}

	t.session.setSteamCommunityWebCookies()

	exp, err := jwtExpiry(t.session.AccessToken)
	if err != nil {
		return fmt.Errorf("parse token expiry: %w", err)
	}
	t.tokenExpiry = exp
	return nil
}

// refreshAndPatchRequest refreshes the token and replaces the cookies
// on the request with fresh ones from the jar.
func (t *authTransport) refreshAndPatchRequest(req *http.Request) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.needsRefresh() {
		return nil // another goroutine refreshed while we waited
	}

	if err := t.refresh(req.Context()); err != nil {
		return err
	}

	t.patchRequestCookies(req)
	return nil
}

// retryAfterRefresh handles server-side token revocation: refreshes the
// token and retries the request exactly once. If the retry also fails
// or the request body can't be replayed, returns the original response.
func (t *authTransport) retryAfterRefresh(req *http.Request, originalResp *http.Response) (*http.Response, error) {
	// Can't replay requests with consumed bodies unless GetBody is set
	if req.Body != nil && req.GetBody == nil {
		return originalResp, nil
	}

	t.mu.Lock()
	err := t.refresh(req.Context())
	t.mu.Unlock()
	if err != nil {
		return originalResp, nil
	}

	// Reset the request body for the retry
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return originalResp, nil
		}
		req.Body = body
	}

	t.patchRequestCookies(req)
	originalResp.Body.Close()

	return t.base.RoundTrip(req)
}

// patchRequestCookies replaces cookies on the request with fresh ones from the jar.
func (t *authTransport) patchRequestCookies(req *http.Request) {
	if jar := t.session.httpClient.Jar; jar != nil {
		req.Header.Del("Cookie")
		for _, c := range jar.Cookies(req.URL) {
			req.AddCookie(c)
		}
	}
}

// isLoginRedirect detects when Steam rejects authentication by redirecting
// to the login page (302 with Location containing "/login").
func isLoginRedirect(resp *http.Response) bool {
	if resp.StatusCode != http.StatusFound {
		return false
	}
	return strings.Contains(resp.Header.Get("Location"), "/login")
}
