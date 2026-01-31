package steamcommunity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
)

// SendTradeOfferOptions contains options for sending a trade offer
type SendTradeOfferOptions struct {
	Partner        steamid.SteamID     // Required: trade partner's SteamID
	Token          string              // Optional: trade token for non-friends
	Message        string              // Optional: message to include (max 128 chars)
	ItemsToGive    []steamapi.TradeAsset // Items to give
	ItemsToReceive []steamapi.TradeAsset // Items to receive
}

// SendTradeOfferResponse contains the response from SendTradeOffer
type SendTradeOfferResponse struct {
	TradeOfferID         string `json:"tradeofferid"`
	NeedsConfirmation    bool   `json:"needs_mobile_confirmation"`
	NeedsEmailConfirm    bool   `json:"needs_email_confirmation"`
	EmailDomain          string `json:"email_domain"`
}

// AcceptTradeOfferResponse contains the response from AcceptTradeOffer
type AcceptTradeOfferResponse struct {
	NeedsConfirmation    bool   `json:"needs_mobile_confirmation"`
	NeedsEmailConfirm    bool   `json:"needs_email_confirmation"`
	EmailDomain          string `json:"email_domain"`
}

// tradeOfferJSON is the internal format for json_tradeoffer
type tradeOfferJSON struct {
	NewVersion bool                 `json:"newversion"`
	Version    int                  `json:"version"`
	Me         tradeOfferParty      `json:"me"`
	Them       tradeOfferParty      `json:"them"`
}

type tradeOfferParty struct {
	Assets   []tradeOfferAsset `json:"assets"`
	Currency []any             `json:"currency"`
	Ready    bool              `json:"ready"`
}

type tradeOfferAsset struct {
	AppID     int    `json:"appid"`
	ContextID string `json:"contextid"`
	Amount    int    `json:"amount"`
	AssetID   string `json:"assetid"`
}

// SendTradeOffer sends a new trade offer to a partner
func (c *Community) SendTradeOffer(ctx context.Context, opts SendTradeOfferOptions) (*SendTradeOfferResponse, error) {
	partnerAccountID := opts.Partner.AccountID()

	// Build the json_tradeoffer structure
	var myAssets []tradeOfferAsset
	for _, item := range opts.ItemsToGive {
		amount := 1
		if item.Amount != "" {
			if parsed, err := strconv.Atoi(item.Amount); err == nil {
				amount = parsed
			}
		}
		myAssets = append(myAssets, tradeOfferAsset{
			AppID:     item.AppID,
			ContextID: item.ContextID,
			Amount:    amount,
			AssetID:   item.AssetID,
		})
	}

	var theirAssets []tradeOfferAsset
	for _, item := range opts.ItemsToReceive {
		amount := 1
		if item.Amount != "" {
			if parsed, err := strconv.Atoi(item.Amount); err == nil {
				amount = parsed
			}
		}
		theirAssets = append(theirAssets, tradeOfferAsset{
			AppID:     item.AppID,
			ContextID: item.ContextID,
			Amount:    amount,
			AssetID:   item.AssetID,
		})
	}

	tradeJSON := tradeOfferJSON{
		NewVersion: true,
		Version:    len(opts.ItemsToGive) + len(opts.ItemsToReceive) + 1,
		Me: tradeOfferParty{
			Assets:   myAssets,
			Currency: []any{},
			Ready:    false,
		},
		Them: tradeOfferParty{
			Assets:   theirAssets,
			Currency: []any{},
			Ready:    false,
		},
	}

	tradeJSONBytes, err := json.Marshal(tradeJSON)
	if err != nil {
		return nil, fmt.Errorf("marshal trade json: %w", err)
	}

	// Build trade_offer_create_params
	createParams := "{}"
	if opts.Token != "" {
		createParamsJSON, _ := json.Marshal(map[string]string{
			"trade_offer_access_token": opts.Token,
		})
		createParams = string(createParamsJSON)
	}

	// Build form data
	formData := url.Values{}
	formData.Set("sessionid", c.sessionID)
	formData.Set("serverid", "1")
	formData.Set("partner", strconv.FormatUint(opts.Partner.ToSteamID64(), 10))
	formData.Set("tradeoffermessage", opts.Message)
	formData.Set("json_tradeoffer", string(tradeJSONBytes))
	formData.Set("captcha", "")
	formData.Set("trade_offer_create_params", createParams)

	// Build referer URL
	refererURL := fmt.Sprintf("https://steamcommunity.com/tradeoffer/new/?partner=%d", partnerAccountID)
	if opts.Token != "" {
		refererURL += "&token=" + opts.Token
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://steamcommunity.com/tradeoffer/new/send", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", refererURL)

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
		TradeOfferID             string `json:"tradeofferid"`
		NeedsMobileConfirmation  bool   `json:"needs_mobile_confirmation"`
		NeedsEmailConfirmation   bool   `json:"needs_email_confirmation"`
		EmailDomain              string `json:"email_domain"`
		StrError                 string `json:"strError"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.StrError != "" {
		return nil, fmt.Errorf("steam error: %s", result.StrError)
	}

	return &SendTradeOfferResponse{
		TradeOfferID:      result.TradeOfferID,
		NeedsConfirmation: result.NeedsMobileConfirmation,
		NeedsEmailConfirm: result.NeedsEmailConfirmation,
		EmailDomain:       result.EmailDomain,
	}, nil
}

// AcceptTradeOffer accepts a received trade offer
func (c *Community) AcceptTradeOffer(ctx context.Context, offerID string, partnerSteamID steamid.SteamID) (*AcceptTradeOfferResponse, error) {
	acceptURL := fmt.Sprintf("https://steamcommunity.com/tradeoffer/%s/accept", offerID)
	refererURL := fmt.Sprintf("https://steamcommunity.com/tradeoffer/%s/", offerID)

	formData := url.Values{}
	formData.Set("sessionid", c.sessionID)
	formData.Set("serverid", "1")
	formData.Set("tradeofferid", offerID)
	formData.Set("partner", strconv.FormatUint(partnerSteamID.ToSteamID64(), 10))
	formData.Set("captcha", "")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, acceptURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", refererURL)

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
		NeedsMobileConfirmation bool   `json:"needs_mobile_confirmation"`
		NeedsEmailConfirmation  bool   `json:"needs_email_confirmation"`
		EmailDomain             string `json:"email_domain"`
		StrError                string `json:"strError"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.StrError != "" {
		return nil, fmt.Errorf("steam error: %s", result.StrError)
	}

	return &AcceptTradeOfferResponse{
		NeedsConfirmation: result.NeedsMobileConfirmation,
		NeedsEmailConfirm: result.NeedsEmailConfirmation,
		EmailDomain:       result.EmailDomain,
	}, nil
}

// CancelTradeOffer cancels a sent trade offer
func (c *Community) CancelTradeOffer(ctx context.Context, offerID string) error {
	return c.cancelOrDeclineOffer(ctx, offerID, "cancel")
}

// DeclineTradeOffer declines a received trade offer
func (c *Community) DeclineTradeOffer(ctx context.Context, offerID string) error {
	return c.cancelOrDeclineOffer(ctx, offerID, "decline")
}

func (c *Community) cancelOrDeclineOffer(ctx context.Context, offerID, action string) error {
	actionURL := fmt.Sprintf("https://steamcommunity.com/tradeoffer/%s/%s", offerID, action)
	refererURL := fmt.Sprintf("https://steamcommunity.com/tradeoffer/%s/", offerID)

	formData := url.Values{}
	formData.Set("sessionid", c.sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, actionURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", refererURL)

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
		TradeOfferID string `json:"tradeofferid"`
		StrError     string `json:"strError"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if result.StrError != "" {
		return fmt.Errorf("steam error: %s", result.StrError)
	}

	return nil
}
