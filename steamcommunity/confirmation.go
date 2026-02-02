package steamcommunity

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/k64z/steamstacks/steamapi"
)

// ConfirmationType represents the type of confirmation.
type ConfirmationType int

const (
	ConfirmationTypeUnknown       ConfirmationType = 0
	ConfirmationTypeTrade         ConfirmationType = 2
	ConfirmationTypeMarketListing ConfirmationType = 3
)

func (t ConfirmationType) String() string {
	switch t {
	case ConfirmationTypeTrade:
		return "Trade"
	case ConfirmationTypeMarketListing:
		return "Market Listing"
	default:
		return "Unknown"
	}
}

// Confirmation represents a pending mobile confirmation.
type Confirmation struct {
	ID        string           `json:"id"`
	Type      ConfirmationType `json:"type"`
	CreatorID string           `json:"creator_id"` // TradeOfferID for trades, listing ID for market
	Key       string           `json:"nonce"`      // Used for responding to confirmation
	Title     string           `json:"title"`
	Headline  string           `json:"headline"`
	Summary   []string         `json:"summary"`
	Timestamp time.Time        `json:"timestamp"`
	Icon      string           `json:"icon"`
}

// getConfirmationKey generates an HMAC-SHA1 confirmation key.
func getConfirmationKey(identitySecret []byte, timestamp int64, tag string) string {
	buf := make([]byte, 8+len(tag))
	binary.BigEndian.PutUint64(buf[:8], uint64(timestamp))
	copy(buf[8:], tag)

	mac := hmac.New(sha1.New, identitySecret)
	mac.Write(buf)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// getDeviceID generates a device ID from a SteamID64.
func getDeviceID(steamID64 uint64) string {
	h := sha1.Sum(fmt.Appendf(nil, "%d", steamID64))
	hex := fmt.Sprintf("%x", h)
	// Format as: android:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

// buildConfirmationParams builds the common query parameters for confirmation requests.
func (c *Community) buildConfirmationParams(identitySecret []byte, tag string) (url.Values, error) {
	serverTime, _, err := steamapi.GetSteamTimeWithClient(context.Background(), c.httpClient)
	if err != nil {
		return nil, fmt.Errorf("get steam time: %w", err)
	}

	steamID64 := c.SteamID.ToSteamID64()
	key := getConfirmationKey(identitySecret, serverTime, tag)
	deviceID := getDeviceID(steamID64)

	params := url.Values{}
	params.Set("p", deviceID)
	params.Set("a", strconv.FormatUint(steamID64, 10))
	params.Set("k", key)
	params.Set("t", strconv.FormatInt(serverTime, 10))
	params.Set("m", "react")
	params.Set("tag", tag)

	return params, nil
}

// GetConfirmations retrieves all pending confirmations.
// The identitySecret should be the base64-decoded identity_secret from your maFile.
func (c *Community) GetConfirmations(ctx context.Context, identitySecret []byte) ([]Confirmation, error) {
	params, err := c.buildConfirmationParams(identitySecret, "list")
	if err != nil {
		return nil, err
	}

	reqURL := "https://steamcommunity.com/mobileconf/getlist?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
		Success bool `json:"success"`
		Conf    []struct {
			ID           string   `json:"id"`
			Type         int      `json:"type"`
			CreatorID    string   `json:"creator_id"`
			Nonce        string   `json:"nonce"`
			TypeName     string   `json:"type_name"`
			Headline     string   `json:"headline"`
			Summary      []string `json:"summary"`
			CreationTime int64    `json:"creation_time"`
			Icon         string   `json:"icon"`
		} `json:"conf"`
		NeedAuth bool   `json:"needauth"`
		Message  string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.NeedAuth {
		return nil, fmt.Errorf("authentication required")
	}

	if !result.Success {
		if result.Message != "" {
			return nil, fmt.Errorf("steam error: %s", result.Message)
		}
		return nil, fmt.Errorf("request failed")
	}

	confirmations := make([]Confirmation, len(result.Conf))
	for i, c := range result.Conf {
		confirmations[i] = Confirmation{
			ID:        c.ID,
			Type:      ConfirmationType(c.Type),
			CreatorID: c.CreatorID,
			Key:       c.Nonce,
			Title:     c.TypeName,
			Headline:  c.Headline,
			Summary:   c.Summary,
			Timestamp: time.Unix(c.CreationTime, 0),
			Icon:      c.Icon,
		}
	}

	return confirmations, nil
}

// respondToConfirmation sends an accept or reject response for a confirmation.
func (c *Community) respondToConfirmation(ctx context.Context, conf Confirmation, identitySecret []byte, accept bool) error {
	tag := "reject"
	op := "cancel"
	if accept {
		tag = "accept"
		op = "allow"
	}

	params, err := c.buildConfirmationParams(identitySecret, tag)
	if err != nil {
		return err
	}

	params.Set("op", op)
	params.Set("cid", conf.ID)
	params.Set("ck", conf.Key)

	reqURL := "https://steamcommunity.com/mobileconf/ajaxop?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !result.Success {
		if result.Message != "" {
			return fmt.Errorf("steam error: %s", result.Message)
		}
		return fmt.Errorf("operation failed")
	}

	return nil
}

// AcceptConfirmation accepts a pending confirmation.
func (c *Community) AcceptConfirmation(ctx context.Context, conf Confirmation, identitySecret []byte) error {
	return c.respondToConfirmation(ctx, conf, identitySecret, true)
}

// RejectConfirmation rejects a pending confirmation.
func (c *Community) RejectConfirmation(ctx context.Context, conf Confirmation, identitySecret []byte) error {
	return c.respondToConfirmation(ctx, conf, identitySecret, false)
}

// AcceptConfirmationByCreatorID finds and accepts a confirmation by its creator ID.
// For trade offers, the creator ID is the trade offer ID.
// For market listings, the creator ID is the listing ID.
func (c *Community) AcceptConfirmationByCreatorID(ctx context.Context, identitySecret []byte, creatorID string) error {
	confirmations, err := c.GetConfirmations(ctx, identitySecret)
	if err != nil {
		return fmt.Errorf("get confirmations: %w", err)
	}

	for _, conf := range confirmations {
		if conf.CreatorID == creatorID {
			return c.AcceptConfirmation(ctx, conf, identitySecret)
		}
	}

	return fmt.Errorf("confirmation with creator ID %s not found", creatorID)
}

// RejectConfirmationByCreatorID finds and rejects a confirmation by its creator ID.
func (c *Community) RejectConfirmationByCreatorID(ctx context.Context, identitySecret []byte, creatorID string) error {
	confirmations, err := c.GetConfirmations(ctx, identitySecret)
	if err != nil {
		return fmt.Errorf("get confirmations: %w", err)
	}

	for _, conf := range confirmations {
		if conf.CreatorID == creatorID {
			return c.RejectConfirmation(ctx, conf, identitySecret)
		}
	}

	return fmt.Errorf("confirmation with creator ID %s not found", creatorID)
}
