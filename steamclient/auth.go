package steamclient

import (
	"context"
	"fmt"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// GenerateAccessTokenForApp requests a new access token (and optionally a
// rotated refresh token) via the CM service method protocol. Unlike the Web API
// variant, this works for SteamClient platform tokens.
func (c *Client) GenerateAccessTokenForApp(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error) {
	c.mu.Lock()
	sid := c.steamID.ToSteamID64()
	c.mu.Unlock()

	body, err := proto.Marshal(&protocol.CAuthentication_AccessToken_GenerateForApp_Request{
		RefreshToken: proto.String(refreshToken),
		Steamid:      proto.Uint64(sid),
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal GenerateAccessTokenForApp request: %w", err)
	}

	pkt, err := c.callServiceMethod(ctx, "Authentication.GenerateAccessTokenForApp#1", body)
	if err != nil {
		return "", "", err
	}

	var resp protocol.CAuthentication_AccessToken_GenerateForApp_Response
	if err := proto.Unmarshal(pkt.Body, &resp); err != nil {
		return "", "", fmt.Errorf("unmarshal GenerateAccessTokenForApp response: %w", err)
	}

	return resp.GetAccessToken(), resp.GetRefreshToken(), nil
}
