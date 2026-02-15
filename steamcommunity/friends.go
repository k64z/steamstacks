package steamcommunity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/k64z/steamstacks/steamid"
)

type EFriendRelationship int

const (
	EFriendRelationshipNone             EFriendRelationship = 0
	EFriendRelationshipBlocked          EFriendRelationship = 1
	EFriendRelationshipRequestRecipient EFriendRelationship = 2 // they sent you a request
	EFriendRelationshipFriend           EFriendRelationship = 3
	EFriendRelationshipRequestInitiator EFriendRelationship = 4 // you sent them a request
	EFriendRelationshipIgnored          EFriendRelationship = 5
	EFriendRelationshipIgnoredFriend    EFriendRelationship = 6
)

func (c *Community) GetFriendsList(ctx context.Context) (map[steamid.SteamID]EFriendRelationship, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://steamcommunity.com/textfilter/ajaxgetfriendslist", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success    int `json:"success"`
		FriendList struct {
			Friends []struct {
				SteamID      string `json:"ulfriendid"`
				Relationship int    `json:"efriendrelationship"`
			} `json:"friends"`
		} `json:"friendslist"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Success != 1 {
		return nil, fmt.Errorf("unexpected success value: %d", result.Success)
	}

	friends := make(map[steamid.SteamID]EFriendRelationship, len(result.FriendList.Friends))
	for _, f := range result.FriendList.Friends {
		sid, err := steamid.FromString(f.SteamID)
		if err != nil {
			return nil, fmt.Errorf("parse friend SteamID %q: %w", f.SteamID, err)
		}
		friends[sid] = EFriendRelationship(f.Relationship)
	}

	return friends, nil
}

func (c *Community) AddFriend(ctx context.Context, target steamid.SteamID) error {
	extra := url.Values{}
	extra.Set("steamid", strconv.FormatUint(target.ToSteamID64(), 10))
	extra.Set("accept_invite", "0")

	resp, err := c.postAction(ctx, "https://steamcommunity.com/actions/AddFriendAjax", extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Success json.RawMessage `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	// Steam returns either true or 1 for success
	if string(result.Success) != "true" && string(result.Success) != "1" {
		return fmt.Errorf("add friend failed")
	}
	return nil
}

func (c *Community) AcceptFriendRequest(ctx context.Context, target steamid.SteamID) error {
	extra := url.Values{}
	extra.Set("steamid", strconv.FormatUint(target.ToSteamID64(), 10))
	extra.Set("accept_invite", "1")

	resp, err := c.postAction(ctx, "https://steamcommunity.com/actions/AddFriendAjax", extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Community) RemoveFriend(ctx context.Context, target steamid.SteamID) error {
	extra := url.Values{}
	extra.Set("steamid", strconv.FormatUint(target.ToSteamID64(), 10))

	resp, err := c.postAction(ctx, "https://steamcommunity.com/actions/RemoveFriendAjax", extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// BlockUser also removes the target from the friends list.
func (c *Community) BlockUser(ctx context.Context, target steamid.SteamID) error {
	extra := url.Values{}
	extra.Set("steamid", strconv.FormatUint(target.ToSteamID64(), 10))

	resp, err := c.postAction(ctx, "https://steamcommunity.com/actions/BlockUserAjax", extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Community) UnblockUser(ctx context.Context, target steamid.SteamID) error {
	extra := url.Values{}
	extra.Set("steamid", strconv.FormatUint(target.ToSteamID64(), 10))
	extra.Set("block", "0")

	resp, err := c.postAction(ctx, "https://steamcommunity.com/actions/BlockUserAjax", extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Community) postAction(ctx context.Context, endpoint string, extra url.Values) (*http.Response, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}

	formData := url.Values{}
	formData.Set("sessionID", c.sessionID)
	maps.Copy(formData, extra)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
