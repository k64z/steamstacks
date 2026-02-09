package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const econServiceURL = "https://api.steampowered.com/IEconService"

// GetTradeOffer retrieves a single trade offer by ID
func (a *API) GetTradeOffer(ctx context.Context, offerID string) (*TradeOffer, error) {
	params := url.Values{}
	params.Set("access_token", a.accessToken)
	params.Set("tradeofferid", offerID)
	params.Set("language", "en")

	reqURL := econServiceURL + "/GetTradeOffer/v1/?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkEconResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		Response struct {
			Offer *TradeOffer `json:"offer"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Response.Offer == nil {
		return nil, fmt.Errorf("offer not found")
	}

	return result.Response.Offer, nil
}

// GetTradeOffers retrieves lists of sent and received trade offers
func (a *API) GetTradeOffers(ctx context.Context, opts GetTradeOffersOptions) (*TradeOffersResponse, error) {
	params := url.Values{}
	params.Set("access_token", a.accessToken)

	if opts.GetSentOffers {
		params.Set("get_sent_offers", "1")
	}
	if opts.GetReceivedOffers {
		params.Set("get_received_offers", "1")
	}
	if opts.GetDescriptions {
		params.Set("get_descriptions", "1")
	}
	if opts.ActiveOnly {
		params.Set("active_only", "1")
	}
	if opts.HistoricalOnly {
		params.Set("historical_only", "1")
	}
	if opts.Language != "" {
		params.Set("language", opts.Language)
	}
	if opts.TimeHistoricalCutoff > 0 {
		params.Set("time_historical_cutoff", strconv.FormatInt(opts.TimeHistoricalCutoff, 10))
	}

	reqURL := econServiceURL + "/GetTradeOffers/v1/?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkEconResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		Response struct {
			TradeOffersResponse
			RawDescriptions []rawAssetDescription `json:"descriptions"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := &result.Response.TradeOffersResponse
	out.Descriptions = convertDescriptions(result.Response.RawDescriptions)
	return out, nil
}

// GetTradeOfferWithDescriptions retrieves a single trade offer with item descriptions.
func (a *API) GetTradeOfferWithDescriptions(ctx context.Context, offerID string) (*GetTradeOfferResult, error) {
	params := url.Values{}
	params.Set("access_token", a.accessToken)
	params.Set("tradeofferid", offerID)
	params.Set("language", "en")
	params.Set("get_descriptions", "1")

	reqURL := econServiceURL + "/GetTradeOffer/v1/?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkEconResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		Response struct {
			Offer           *TradeOffer           `json:"offer"`
			RawDescriptions []rawAssetDescription `json:"descriptions"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Response.Offer == nil {
		return nil, fmt.Errorf("offer not found")
	}

	return &GetTradeOfferResult{
		Offer:        result.Response.Offer,
		Descriptions: convertDescriptions(result.Response.RawDescriptions),
	}, nil
}

func convertDescriptions(raw []rawAssetDescription) map[string]AssetDescription {
	if len(raw) == 0 {
		return nil
	}
	m := make(map[string]AssetDescription, len(raw))
	for _, d := range raw {
		m[AssetDescriptionKey(d.AppID, d.ClassID, d.InstanceID)] = AssetDescription{
			AppID:          d.AppID,
			ClassID:        d.ClassID,
			InstanceID:     d.InstanceID,
			Name:           d.Name,
			MarketHashName: d.MarketHashName,
			Type:           d.Type,
			Tradable:       d.Tradable == 1,
			Marketable:     d.Marketable == 1,
			Commodity:      d.Commodity == 1,
			IconURL:        d.IconURL,
			IconURLLarge:   d.IconURLLarge,
			Descriptions:   d.Descriptions,
			Tags:           d.Tags,
			Actions:        d.Actions,
			FraudWarnings:  d.FraudWarnings,
		}
	}
	return m
}

// checkEconResponse checks the response from IEconService endpoints
func checkEconResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	eresult := resp.Header.Get("X-Eresult")
	if eresult != "" && eresult != "1" {
		return fmt.Errorf("X-Eresult: %s", eresult)
	}

	return nil
}
