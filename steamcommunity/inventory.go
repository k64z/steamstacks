package steamcommunity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/k64z/steamstacks/steamid"
)

type InventoryItem struct {
	AssetID    string `json:"assetid"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`

	Name                        string            `json:"name"`
	MarketHashName              string            `json:"market_hash_name,omitzero"`
	Type                        string            `json:"type"`
	Tradable                    bool              `json:"tradable"`
	Marketable                  bool              `json:"marketable"`
	Commodity                   bool              `json:"commodity"`
	MarketTradableRestriction   int               `json:"market_tradable_restriction,omitzero"`
	MarketMarketableRestriction int               `json:"market_marketable_restriction,omitzero"`
	IconURL                     string            `json:"icon_url"`
	IconURLLarge                string            `json:"icon_url_large,omitzero"`
	Descriptions                []DescriptionLine `json:"descriptions,omitzero"`
	Tags                        []InventoryTag    `json:"tags,omitzero"`
	Actions                     []InventoryAction `json:"actions,omitzero"`
	FraudWarnings               []string          `json:"fraudwarnings,omitzero"`
}

type DescriptionLine struct {
	Type  string `json:"type,omitzero"`
	Value string `json:"value"`
	Color string `json:"color,omitzero"`
}

type InventoryTag struct {
	Category              string `json:"category"`
	InternalName          string `json:"internal_name"`
	LocalizedCategoryName string `json:"localized_category_name"`
	LocalizedTagName      string `json:"localized_tag_name"`
	Color                 string `json:"color,omitzero"`
}

type InventoryAction struct {
	Link string `json:"link"`
	Name string `json:"name"`
}

type inventoryResponse struct {
	Success             int                    `json:"success"`
	TotalInventoryCount int                    `json:"total_inventory_count"`
	Assets              []inventoryAsset       `json:"assets"`
	Descriptions        []inventoryDescription `json:"descriptions"`
	MoreItems           int                    `json:"more_items,omitzero"`
	LastAssetID         string                 `json:"last_assetid,omitzero"`
}

type inventoryAsset struct {
	AppID      int    `json:"appid"`
	ContextID  string `json:"contextid"`
	AssetID    string `json:"assetid"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`
}

type inventoryDescription struct {
	ClassID                     string            `json:"classid"`
	InstanceID                  string            `json:"instanceid"`
	Name                        string            `json:"name"`
	MarketHashName              string            `json:"market_hash_name"`
	Type                        string            `json:"type"`
	Tradable                    int               `json:"tradable"`
	Marketable                  int               `json:"marketable"`
	Commodity                   int               `json:"commodity"`
	MarketTradableRestriction   int               `json:"market_tradable_restriction"`
	MarketMarketableRestriction int               `json:"market_marketable_restriction"`
	IconURL                     string            `json:"icon_url"`
	IconURLLarge                string            `json:"icon_url_large"`
	Descriptions                []DescriptionLine `json:"descriptions"`
	Tags                        []InventoryTag    `json:"tags"`
	Actions                     []InventoryAction `json:"actions"`
	FraudWarnings               []string          `json:"fraudwarnings"`
}

func descriptionKey(classID, instanceID string) string {
	return classID + "_" + instanceID
}

func parseInventoryResponse(data []byte) (items []InventoryItem, hasMore bool, lastAssetID string, err error) {
	var resp inventoryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, "", fmt.Errorf("decode response: %w", err)
	}

	if resp.Success != 1 {
		return nil, false, "", fmt.Errorf("request failed: success=%d", resp.Success)
	}

	descMap := make(map[string]inventoryDescription, len(resp.Descriptions))
	for _, desc := range resp.Descriptions {
		descMap[descriptionKey(desc.ClassID, desc.InstanceID)] = desc
	}

	items = make([]InventoryItem, 0, len(resp.Assets))
	for _, asset := range resp.Assets {
		desc := descMap[descriptionKey(asset.ClassID, asset.InstanceID)]
		items = append(items, InventoryItem{
			AssetID:                     asset.AssetID,
			ClassID:                     asset.ClassID,
			InstanceID:                  asset.InstanceID,
			Amount:                      asset.Amount,
			Name:                        desc.Name,
			MarketHashName:              desc.MarketHashName,
			Type:                        desc.Type,
			Tradable:                    desc.Tradable == 1,
			Marketable:                  desc.Marketable == 1,
			Commodity:                   desc.Commodity == 1,
			MarketTradableRestriction:   desc.MarketTradableRestriction,
			MarketMarketableRestriction: desc.MarketMarketableRestriction,
			IconURL:                     desc.IconURL,
			IconURLLarge:                desc.IconURLLarge,
			Descriptions:                desc.Descriptions,
			Tags:                        desc.Tags,
			Actions:                     desc.Actions,
			FraudWarnings:               desc.FraudWarnings,
		})
	}

	return items, resp.MoreItems == 1, resp.LastAssetID, nil
}

var (
	errInventoryPrivate = errors.New("inventory is private")
	errRateLimited      = errors.New("rate limited")
)

func (c *Community) GetInventory(ctx context.Context, steamID steamid.SteamID, appID int, contextID string) ([]InventoryItem, error) {
	steamID64 := strconv.FormatUint(steamID.ToSteamID64(), 10)
	referer := fmt.Sprintf("https://steamcommunity.com/profiles/%s/inventory", steamID64)

	var allItems []InventoryItem
	var startAssetID string

	for {
		reqURL := fmt.Sprintf(
			"https://steamcommunity.com/inventory/%s/%d/%s?l=english&count=1000",
			steamID64, appID, contextID,
		)
		if startAssetID != "" {
			reqURL += "&start_assetid=" + startAssetID
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Referer", referer)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			// continue processing below
		case http.StatusForbidden:
			return nil, errInventoryPrivate
		case http.StatusTooManyRequests:
			return nil, errRateLimited
		default:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		items, hasMore, lastAssetID, err := parseInventoryResponse(body)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)

		if !hasMore {
			break
		}
		startAssetID = lastAssetID
	}

	return allItems, nil
}

func (c *Community) GetOwnInventory(ctx context.Context, appID int, contextID string) ([]InventoryItem, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	return c.GetInventory(ctx, c.SteamID, appID, contextID)
}
