package steamsession

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
)

type finalizeLoginResp struct {
	SteamID      steamid.SteamID `json:"steamID,string"`
	TransferInfo []*TransferInfo `json:"transfer_info"`
}

type TransferInfo struct {
	URL    string `json:"url"`
	Params struct {
		Nonce string `json:"nonce"`
		Auth  string `json:"auth"`
	} `json:"params"`
}

func (s *Session) GetWebCookies(ctx context.Context) error {
	if s.RefreshToken == "" {
		return errors.New("refresh token is required")
	}

	s.sessionID = mustGenerateSessionID()

	// SteamClient and MobileApp tokens are constructed directly from the access
	// token obtained during login (PollAuthSessionStatus). Unlike WebBrowser,
	// no FinalizeLogin roundtrip is needed.
	//
	// NOTE: GenerateAccessTokenForApp via the Web API only works for MobileApp.
	// SteamClient returns EResult 63 (AccountLogonDenied) — the real Steam
	// desktop client refreshes tokens via CM (Connection Manager), a persistent
	// binary protocol not available here. When the access token expires (~24h),
	// a full re-login is required.
	if s.platformType == PlatformTypeSteamClient || s.platformType == PlatformTypeMobileApp {
		if s.AccessToken == "" {
			return errors.New("access token is required for SteamClient/MobileApp (re-login needed)")
		}

		exp, err := jwtExpiry(s.AccessToken)
		if err != nil {
			return fmt.Errorf("parse access token expiry: %w", err)
		}
		if time.Now().After(exp) {
			return errors.New("access token has expired (re-login needed)")
		}

		s.setSteamCommunityWebCookies()

		if s.platformType == PlatformTypeMobileApp {
			s.installAuthTransport(exp)
		}

		return nil
	}

	if err := s.FinalizeLogin(ctx); err != nil {
		return err
	}

	// Install authTransport if we have a valid access token from login.
	// For saved sessions with expired access tokens, FinalizeLogin has
	// already set cookies via transfer info — requests work, just without
	// proactive token refresh.
	if s.AccessToken != "" {
		if exp, err := jwtExpiry(s.AccessToken); err == nil && time.Now().Before(exp) {
			s.installAuthTransport(exp)
		}
	}

	return nil
}

func (s *Session) FinalizeLogin(ctx context.Context) error {
	// TODO: init cookie jar at the start
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	w.WriteField("nonce", s.RefreshToken)
	w.WriteField("sessionid", s.sessionID)
	w.WriteField("redir", "https://steamcommunity.com/login/home/?goto=")
	w.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.loginURL+"/jwt/finalizelogin", buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("Origin", "https://steamcommunity.com")
	httpReq.Header.Set("Referer", "https://steamcommunity.com")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// The response has the Set-Cookie header, which contains a single cookie.
	// The cookie is "steamRefresh_steam", which is essentially `steam_id||refresh_token`
	// I'm not sure if we actually need to keep it

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	var result finalizeLoginResp
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("json: %w", err)
	}

	if len(result.TransferInfo) == 0 {
		return errors.New("invalid response: empty transfer_info")
	}

	for _, transferInfo := range result.TransferInfo {
		err := s.submitTransferInfo(ctx, *transferInfo)
		if err != nil {
			return fmt.Errorf("submit transfer info on %s: %w", transferInfo.URL, err)
		}
	}

	return nil
}

func (s *Session) submitTransferInfo(ctx context.Context, transferInfo TransferInfo) error {
	u, err := url.Parse(transferInfo.URL)
	if err != nil {
		return fmt.Errorf("parseURL: %w", err)
	}

	s.httpClient.Jar.SetCookies(u, []*http.Cookie{
		{
			Name:     "sessionid",
			Value:    s.sessionID,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
			HttpOnly: true,
			Path:     "/",
		},
	})

	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	w.WriteField("nonce", transferInfo.Params.Nonce)
	w.WriteField("auth", transferInfo.Params.Auth)
	w.WriteField("steamID", strconv.FormatUint(s.SteamID.ToSteamID64(), 10))
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transferInfo.URL, buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// setSteamCommunityWebCookies populates the cookie jar with sessionid and
// steamLoginSecure for steamcommunity.com using the current AccessToken.
func (s *Session) setSteamCommunityWebCookies() {
	u, _ := url.Parse("https://steamcommunity.com")
	s.httpClient.Jar.SetCookies(u, []*http.Cookie{
		{
			Name:     "sessionid",
			Value:    s.sessionID,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:     "steamLoginSecure",
			Value:    fmt.Sprintf("%d%%7C%%7C%s", s.SteamID.ToSteamID64(), s.AccessToken),
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
		},
	})
}

// installAuthTransport wraps the HTTP client's transport with an authTransport
// that automatically refreshes the access token before it expires.
func (s *Session) installAuthTransport(tokenExpiry time.Time) {
	// If already installed, just update the expiry
	if at, ok := s.httpClient.Transport.(*authTransport); ok {
		at.mu.Lock()
		at.tokenExpiry = tokenExpiry
		at.mu.Unlock()
		return
	}

	base := s.httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	s.httpClient.Transport = &authTransport{
		base:        base,
		session:     s,
		tokenExpiry: tokenExpiry,
	}
}

// accessTokenFromJar extracts the access token from the steamLoginSecure
// cookie in the cookie jar for steamcommunity.com.
func (s *Session) accessTokenFromJar() (string, error) {
	u, _ := url.Parse("https://steamcommunity.com")
	for _, c := range s.httpClient.Jar.Cookies(u) {
		if c.Name == "steamLoginSecure" {
			parts := strings.Split(c.Value, "%7C%7C")
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", errors.New("steamLoginSecure cookie not found")
}

// refreshAccessToken uses the refresh token to obtain a fresh access token
// via the GenerateAccessTokenForApp Steam API.
//
// NOTE: This only works for MobileApp tokens. WebBrowser tokens get
// EResult 15 (AccessDenied) and SteamClient tokens get EResult 63
// (AccountLogonDenied) unless sent over an authenticated CM session.
// For WebBrowser, authTransport uses FinalizeLogin instead.
func (s *Session) refreshAccessToken(ctx context.Context) error {
	sid := s.SteamID.ToSteamID64()
	resp, err := s.steamAPI.GenerateAccessTokenForApp(ctx, &protocol.CAuthentication_AccessToken_GenerateForApp_Request{
		RefreshToken: &s.RefreshToken,
		Steamid:      &sid,
	})
	if err != nil {
		return err
	}
	if resp.AccessToken == nil {
		return errors.New("access token is nil")
	}
	s.AccessToken = *resp.AccessToken
	return nil
}

// ExpireAuthTransportToken forces the authTransport to treat the current
// access token as expired. The next request to steamcommunity.com will
// trigger a proactive token refresh.
// This is a no-op if authTransport is not installed.
func (s *Session) ExpireAuthTransportToken() {
	at, ok := s.httpClient.Transport.(*authTransport)
	if !ok {
		return
	}
	at.mu.Lock()
	at.tokenExpiry = time.Time{}
	at.mu.Unlock()
}

// mustGenerateSessionID generates a session ID.
// Returns a 24-character hexadecimal string (hex-encoded 12 random bytes).
// Panics if the system's random number generator is unavailable
// or fails to provide sufficient entropy
//
// Example output: "06464bc0126a6a8ed1bb9089"
func mustGenerateSessionID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("crypto random source unavailable: " + err.Error())
	}
	sessionID := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(sessionID, b)
	return string(sessionID)
}
