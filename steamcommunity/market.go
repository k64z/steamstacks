package steamcommunity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MarketPriceOverview is the Steam Community Market /priceoverview/
// response. Prices are localized strings (e.g. "$1.50", "1,50€").
type MarketPriceOverview struct {
	Success     bool   `json:"success"`
	LowestPrice string `json:"lowest_price"`
	MedianPrice string `json:"median_price"`
	Volume      string `json:"volume"`
}

// MarketSellResult captures the /sellitem/ response. A successful
// listing generally returns RequiresConfirmation=1 and
// MobileConfirmationRequired=true; the caller is expected to accept
// the resulting mobile confirmation before the listing goes live.
type MarketSellResult struct {
	Success                    bool   `json:"success"`
	Message                    string `json:"message"`
	RequiresConfirmation       uint32 `json:"requires_confirmation"`
	MobileConfirmationRequired bool   `json:"needs_mobile_confirmation"`
	EmailConfirmationRequired  bool   `json:"needs_email_confirmation"`
	EmailDomain                string `json:"email_domain"`
}

// Market-side error taxonomy. Callers typically log-and-continue on
// these rather than aborting the whole cycle.
var (
	ErrMarketItemServerDown       = errors.New("market: game's item server may be down")
	ErrMarketPendingConfirmation  = errors.New("market: listing pending confirmation for this item")
	ErrMarketItemNotInInventory   = errors.New("market: item no longer in inventory")
	ErrMarketListingProblem       = errors.New("market: generic listing problem; retry")
	ErrMarketWalletTooMuchMoney   = errors.New("market: wallet holds too much money")
	ErrMarketPreviousActionPending = errors.New("market: previous action still pending")
)

// GetMarketPriceOverview fetches the market-wide price overview for a
// single item. currency is a Steam currency code (1 = USD, 5 = GBP, ...).
func (c *Community) GetMarketPriceOverview(ctx context.Context, appID, currency int, marketHashName string) (*MarketPriceOverview, error) {
	q := url.Values{}
	q.Set("appid", strconv.Itoa(appID))
	q.Set("currency", strconv.Itoa(currency))
	q.Set("market_hash_name", marketHashName)

	reqURL := "https://steamcommunity.com/market/priceoverview/?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	out := &MarketPriceOverview{}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

// SellMarketItem lists an inventory item on the Steam Community Market.
// priceCents is what the seller receives (i.e. Steam's cut is added on
// top for the buyer). amount is almost always 1 for TF2 items.
//
// On known failure patterns we return a typed error so the caller can
// log-and-skip without string matching. On success the listing is
// pending mobile confirmation — callers should accept the confirmation
// whose CreatorID matches the listing. Since Steam doesn't return the
// listing ID in this response, matching by timestamp/position is the
// practical approach.
func (c *Community) SellMarketItem(ctx context.Context, appID int, contextID uint64, assetID uint64, amount, priceCents int) (*MarketSellResult, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("amount", strconv.Itoa(amount))
	form.Set("appid", strconv.Itoa(appID))
	form.Set("assetid", strconv.FormatUint(assetID, 10))
	form.Set("contextid", strconv.FormatUint(contextID, 10))
	form.Set("price", strconv.Itoa(priceCents))
	form.Set("sessionid", c.sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://steamcommunity.com/market/sellitem/",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://steamcommunity.com/market/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	out := &MarketSellResult{}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.Success {
		return out, nil
	}

	switch {
	case strings.Contains(out.Message, "The game's item server may be down"):
		return out, ErrMarketItemServerDown
	case strings.Contains(out.Message, "You already have a listing for this item pending confirmation"):
		return out, ErrMarketPendingConfirmation
	case strings.Contains(out.Message, "The item specified is no longer in your inventory"):
		return out, ErrMarketItemNotInInventory
	case strings.Contains(out.Message, "There was a problem listing your item"):
		return out, ErrMarketListingProblem
	case strings.Contains(out.Message, "You must spend some Steam Wallet funds"):
		return out, ErrMarketWalletTooMuchMoney
	case strings.Contains(out.Message, "You cannot sell any items until your previous action completes"):
		return out, ErrMarketPreviousActionPending
	}
	return out, fmt.Errorf("market sell failed: %s", out.Message)
}

// listingIDRE matches numeric listing IDs embedded in the market HTML
// (the row class is `market_recent_listing_row listing_<id>`).
var listingIDRE = regexp.MustCompile(`market_recent_listing_row listing_(\d+)`)

// GetMyMarketListingIDs scrapes listing IDs from the authenticated
// user's market home page. Steam does not expose a JSON endpoint for
// this, so we parse the HTML — fragile but tracks Fhub's long-running
// approach. Returns deduplicated listing IDs in document order.
func (c *Community) GetMyMarketListingIDs(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://steamcommunity.com/market/", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	matches := listingIDRE.FindAllStringSubmatch(string(body), -1)
	seen := make(map[string]bool, len(matches))
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids, nil
}

// CancelMarketListing removes a single active listing by its ID.
func (c *Community) CancelMarketListing(ctx context.Context, listingID string) error {
	if err := c.ensureInit(); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("sessionid", c.sessionID)

	reqURL := "https://steamcommunity.com/market/removelisting/" + url.PathEscape(listingID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://steamcommunity.com/market")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
