package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// GetSteamTime fetches the current time from Steam servers.
// Returns the server timestamp and the offset from the local clock.
// This endpoint doesn't require authentication.
func GetSteamTime(ctx context.Context) (serverTime int64, offset int64, err error) {
	return GetSteamTimeWithClient(ctx, http.DefaultClient)
}

// GetSteamTimeWithClient fetches the current time from Steam servers using a custom HTTP client.
func GetSteamTimeWithClient(ctx context.Context, client *http.Client) (serverTime int64, offset int64, err error) {
	localTime := time.Now().Unix()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.steampowered.com/ITwoFactorService/QueryTime/v1/", nil)
	if err != nil {
		return 0, 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Response struct {
			ServerTime string `json:"server_time"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("decode response: %w", err)
	}

	serverTime, err = strconv.ParseInt(result.Response.ServerTime, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse server time: %w", err)
	}

	offset = serverTime - localTime
	return serverTime, offset, nil
}
