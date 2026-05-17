package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/k64z/steamstacks/steamtotp"
)

type TwoFactorData struct {
	SharedSecret   string `json:"shared_secret"`
	SerialNumber   string `json:"serial_number"`
	RevocationCode string `json:"revocation_code"`
	URI            string `json:"uri"`
	ServerTime     string `json:"server_time"`
	AccountName    string `json:"account_name"`
	TokenGID       string `json:"token_gid"`
	IdentitySecret string `json:"identity_secret"`
	Secret1        string `json:"secret_1"`
	Status         int    `json:"status"`
}

type addAuthenticatorResponse struct {
	Response *TwoFactorData `json:"response"`
}

type finalizeAuthenticatorResponse struct {
	Response struct {
		Success    bool   `json:"success"`
		ServerTime string `json:"server_time"`
		Status     int    `json:"status"`
	} `json:"response"`
}

// AddAuthenticator calls ITwoFactorService/AddAuthenticator/v1 to begin 2FA setup.
func (a *API) AddAuthenticator(ctx context.Context, steamID uint64) (*TwoFactorData, error) {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	formData := url.Values{
		"steamid":            {strconv.FormatUint(steamID, 10)},
		"authenticator_type": {"1"},
		"device_identifier":  {steamtotp.GetDeviceID(steamID)},
		"sms_phone_id":       {"1"},
	}

	apiURL := a.baseURL + "/ITwoFactorService/AddAuthenticator/v1?access_token=" + url.QueryEscape(accessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	eresult := resp.Header.Get("X-Eresult")
	log.Printf("[2FA] AddAuthenticator X-Eresult: %s, HTTP status: %d", eresult, resp.StatusCode)

	if eresult != "" && eresult != "1" {
		return nil, fmt.Errorf("X-Eresult: %s", eresult)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	log.Printf("[2FA] AddAuthenticator response: %s", string(body))

	var result addAuthenticatorResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if result.Response == nil {
		return nil, fmt.Errorf("empty response body: %s", string(body))
	}

	if result.Response.Status != 1 {
		return nil, fmt.Errorf("bad status %d (body: %s)", result.Response.Status, string(body))
	}

	return result.Response, nil
}

// FinalizeAddAuthenticator calls ITwoFactorService/FinalizeAddAuthenticator/v1 to complete 2FA setup.
func (a *API) FinalizeAddAuthenticator(ctx context.Context, steamID uint64, sharedSecret, smsCode string) error {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	guardCode, err := steamtotp.GenerateAuthCode(sharedSecret, 0)
	if err != nil {
		return fmt.Errorf("generate guard code: %w", err)
	}

	serverTime, _, err := GetSteamTimeWithClient(ctx, a.httpClient)
	if err != nil {
		return fmt.Errorf("get steam time: %w", err)
	}

	formData := url.Values{
		"steamid":            {strconv.FormatUint(steamID, 10)},
		"activation_code":    {smsCode},
		"authenticator_code": {guardCode},
		"authenticator_time": {strconv.FormatInt(serverTime, 10)},
	}

	apiURL := a.baseURL + "/ITwoFactorService/FinalizeAddAuthenticator/v1?access_token=" + url.QueryEscape(accessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	eresult := resp.Header.Get("X-Eresult")
	log.Printf("[2FA] FinalizeAddAuthenticator X-Eresult: %s, HTTP status: %d", eresult, resp.StatusCode)

	if eresult != "" && eresult != "1" {
		return fmt.Errorf("X-Eresult: %s", eresult)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	log.Printf("[2FA] FinalizeAddAuthenticator response: %s", string(body))

	var result finalizeAuthenticatorResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if !result.Response.Success {
		return fmt.Errorf("finalize failed (status %d, body: %s)", result.Response.Status, string(body))
	}

	return nil
}
