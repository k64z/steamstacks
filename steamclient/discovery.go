package steamclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CMServer represents a Steam CM server endpoint.
type CMServer struct {
	Addr string // "host:port" for TCP, "host" for WebSocket
	Type string // "websockets" or "netfilter"
}

const cmListURL = "https://api.steampowered.com/ISteamDirectory/GetCMListForConnect/v1/?cellid=0"

// DiscoverServers fetches the CM server list from the Steam Web API.
func DiscoverServers(ctx context.Context, httpClient *http.Client) ([]CMServer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cmListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseCMList(body)
}

type cmListResponse struct {
	Response struct {
		ServerList []struct {
			Endpoint string `json:"endpoint"`
			Type     string `json:"type"`
		} `json:"serverlist"`
	} `json:"response"`
}

func parseCMList(data []byte) ([]CMServer, error) {
	var resp cmListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	servers := make([]CMServer, 0, len(resp.Response.ServerList))
	for _, s := range resp.Response.ServerList {
		servers = append(servers, CMServer{
			Addr: s.Endpoint,
			Type: s.Type,
		})
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers in response")
	}

	return servers, nil
}
