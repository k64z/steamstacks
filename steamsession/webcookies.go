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
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

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

	if s.platformType == PlatformTypeSteamClient || s.platformType == PlatformTypeMobileApp {
		// TODO: SteamClient's and MobileApp's steamLoginSecure is s.AccessToken
		return nil
	}

	err := s.FinalizeLogin(ctx)
	if err != nil {
		return fmt.Errorf("finalize login: %w", err)
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

	log.Println("RefreshToken", s.RefreshToken)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://login.steampowered.com/jwt/finalizelogin", buf)
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
	log.Printf("Setting token on %s (%d)", transferInfo.URL, s.steamID)

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
	w.WriteField("steamID", strconv.FormatUint(s.steamID.ToSteamID64(), 10))
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transferInfo.URL, buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	// req.AddCookie(&http.Cookie{Name: "sessionid", Value: s.sessionID})

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
