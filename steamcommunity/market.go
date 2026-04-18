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

// MarketListing is one of the caller's active market listings from
// the render endpoint. PriceText and CreatedText are localized
// strings Steam renders into the row; numeric price and precise
// timestamps are not returned by this endpoint (they live in
// g_rgListingInfo on the non-paginated /market/ page). Asset is
// populated by joining the per-row RemoveMarketListing asset_id
// against the response's assets sidecar.
type MarketListing struct {
	ID string `json:"id"`
	// PriceText is the localized buyer-facing price Steam renders
	// next to each row (e.g. "$1.23" / "1,23€"). No numeric value
	// available from this endpoint.
	PriceText string `json:"price_text,omitempty"`
	// CreatedText is the short date Steam renders (e.g. "18 Apr"
	// or "Listed: 18 Apr"). No precise timestamp available.
	CreatedText string `json:"created_text,omitempty"`
	// Asset carries structured item metadata joined from the
	// response's assets sidecar. nil when the join fails.
	Asset *MarketListingAsset `json:"asset,omitempty"`
}

// MarketListingAsset is the underlying Steam inventory asset a
// listing references. All fields come from Steam's canonical
// per-asset JSON, so MarketHashName here is authoritative (vs any
// HTML-rendered display name).
type MarketListingAsset struct {
	AssetID        string `json:"asset_id"`
	AppID          int    `json:"app_id"`
	ContextID      string `json:"context_id"`
	ClassID        string `json:"class_id"`
	InstanceID     string `json:"instance_id"`
	Amount         string `json:"amount"`
	MarketHashName string `json:"market_hash_name"`
	Name           string `json:"name"`
	IconURL        string `json:"icon_url"`
}

// MarketListingsPage pairs a single page of listings with the total
// across all pages, so callers can render "X of Y" and drive
// Previous/Next buttons without a separate count endpoint.
type MarketListingsPage struct {
	Listings []MarketListing `json:"listings"`
	Start    int             `json:"start"`
	PageSize int             `json:"pagesize"`
	Total    int             `json:"total"`
}

// marketListingsResponse mirrors the JSON shape of
// https://steamcommunity.com/market/mylistings/render/. Assets is a
// three-level nested map keyed by app_id → context_id → asset_id.
// Keys are strings even for app_id (Steam uses stringified ints).
type marketListingsResponse struct {
	Success     bool                                       `json:"success"`
	PageSize    int                                        `json:"pagesize"`
	TotalCount  int                                        `json:"total_count"`
	ResultsHTML string                                     `json:"results_html"`
	Assets      map[string]map[string]map[string]assetInfo `json:"assets"`
}

// assetInfo is the shape of each inner asset in the response's
// assets map. Field names match what Steam's market renderer uses
// on the HTML page side.
type assetInfo struct {
	AppID          int    `json:"appid"`
	ContextID      string `json:"contextid"`
	ID             string `json:"id"`
	ClassID        string `json:"classid"`
	InstanceID     string `json:"instanceid"`
	Amount         string `json:"amount"`
	MarketHashName string `json:"market_hash_name"`
	Name           string `json:"name"`
	IconURL        string `json:"icon_url"`
}

// removeMarketListingRE extracts the (app_id, context_id, asset_id)
// tuple from the RemoveMarketListing(...) JavaScript call in each
// row's cancel button. The listing_id is the outer row's id which
// we already pull via rowSplitRE, so we skip it here.
var removeMarketListingRE = regexp.MustCompile(`RemoveMarketListing\('mylisting',\s*'\d+',\s*(\d+),\s*'(\d+)',\s*'(\d+)'`)

// rowSplitRE locates the outer div of each listing row so the parser
// can slice results_html into per-listing chunks. Anchors on digits
// followed by a literal `"` to avoid colliding with the inner
// `id="mylisting_<id>_name"` span within the same row.
var rowSplitRE = regexp.MustCompile(`id="mylisting_(\d+)"`)

// Class-name markers used by parseMarketListings. All are locale-
// independent — the text content they wrap may be localized, but
// the class attributes are stable across Steam's translations.
//
// The trailing `"` on markerPrice anchors on the class attribute's
// closing quote so we don't accidentally match the historical
// `market_listing_price_with_fee` variant that shares the prefix.
const (
	markerItemName           = `market_listing_item_name_link`
	markerPrice              = `market_listing_price"`
	markerListedDateCombined = `market_listing_listed_date_combined`
	markerListedDateShort    = `market_listing_listed_date`
)

// GetMarketListings fetches a single page of the caller's active
// market listings from the render endpoint. Steam caps count at 100;
// larger values are clamped. count <= 0 defaults to 100. start is
// zero-indexed. Callers paginate by advancing start by len(page.Listings)
// until start >= page.Total (or the returned listings are empty).
func (c *Community) GetMarketListings(ctx context.Context, start, count int) (*MarketListingsPage, error) {
	if count <= 0 {
		count = 100
	}
	if count > 100 {
		count = 100
	}
	if start < 0 {
		start = 0
	}

	q := url.Values{}
	q.Set("query", "")
	q.Set("start", strconv.Itoa(start))
	q.Set("count", strconv.Itoa(count))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://steamcommunity.com/market/mylistings/render/?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
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
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var r marketListingsResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if !r.Success {
		return nil, errors.New("render endpoint returned success=false")
	}

	listings := parseMarketListings(r.ResultsHTML)
	// Enrich each listing's Asset with canonical metadata from the
	// `assets` sidecar, joined on the (app, context, asset_id)
	// triple parseMarketListings extracted from the cancel button.
	// Rows whose asset lookup misses keep the HTML-parsed
	// MarketHashName fallback set by parseMarketListings.
	for i := range listings {
		a := listings[i].Asset
		if a == nil {
			continue
		}
		appStr := strconv.Itoa(a.AppID)
		if ai, ok := lookupAsset(r.Assets, appStr, a.ContextID, a.AssetID); ok {
			a.ClassID = ai.ClassID
			a.InstanceID = ai.InstanceID
			a.Amount = ai.Amount
			a.IconURL = ai.IconURL
			a.Name = ai.Name
			if ai.MarketHashName != "" {
				a.MarketHashName = ai.MarketHashName
			}
		}
	}
	return &MarketListingsPage{
		Listings: listings,
		Start:    start,
		PageSize: r.PageSize,
		Total:    r.TotalCount,
	}, nil
}

// lookupAsset resolves assets[app][context][asset] without panicking
// on missing intermediate keys.
func lookupAsset(assets map[string]map[string]map[string]assetInfo, app, ctx, asset string) (assetInfo, bool) {
	if assets == nil {
		return assetInfo{}, false
	}
	ctxMap, ok := assets[app]
	if !ok {
		return assetInfo{}, false
	}
	assetMap, ok := ctxMap[ctx]
	if !ok {
		return assetInfo{}, false
	}
	a, ok := assetMap[asset]
	return a, ok
}

// parseMarketListings slices results_html by row boundaries and
// extracts (ID, price, date, asset ref) per row. The asset ref is
// the (app, context, asset_id) triple — callers join it against
// the response's `assets` sidecar to enrich the listing with
// canonical asset metadata. Missing fields are left empty rather
// than dropping the row, so a partial match still yields a usable
// overview entry.
//
// The parser is regex-based and necessarily coupled to Steam's
// market HTML layout. The three class markers it anchors on
// (markerItemName, markerPrice, markerListedDate*) are stable
// across locales; only the text CONTENT those elements wrap is
// translated.
func parseMarketListings(html string) []MarketListing {
	indices := rowSplitRE.FindAllStringSubmatchIndex(html, -1)
	if len(indices) == 0 {
		return nil
	}
	out := make([]MarketListing, 0, len(indices))
	for i, m := range indices {
		id := html[m[2]:m[3]]
		rowEnd := len(html)
		if i+1 < len(indices) {
			rowEnd = indices[i+1][0]
		}
		chunk := html[m[0]:rowEnd]

		listing := MarketListing{
			ID:          id,
			PriceText:   firstInnerText(chunk, markerPrice),
			CreatedText: firstInnerText(chunk, markerListedDateCombined),
		}
		// Some renders omit the _combined wrapper; fall back to
		// the short date field (no "Listed:" prefix).
		if listing.CreatedText == "" {
			listing.CreatedText = firstInnerText(chunk, markerListedDateShort)
		}
		// Asset ref is in the cancel button's RemoveMarketListing
		// call: (listing_id, app, context, asset_id). Captured
		// here so the caller can look up the full asset metadata
		// in the response's `assets` sidecar.
		if am := removeMarketListingRE.FindStringSubmatch(chunk); len(am) == 4 {
			asset := &MarketListingAsset{
				ContextID: am[2],
				AssetID:   am[3],
			}
			if app, err := strconv.Atoi(am[1]); err == nil {
				asset.AppID = app
			}
			// HTML-parsed name is a best-guess fallback — will be
			// overridden in GetMarketListings with the canonical
			// value from the assets sidecar if present.
			asset.MarketHashName = firstInnerText(chunk, markerItemName)
			listing.Asset = asset
		}
		out = append(out, listing)
	}
	return out
}

// firstInnerText finds marker in html, advances past the enclosing
// tag's `>`, and returns the first non-whitespace text run inside
// that element — skipping any number of nested opening tags and
// intermediate whitespace. Whitespace is collapsed in the return.
//
// Semantics match what a user reading the rendered page would see
// as "the first visible text in this element". Works for both
// direct-text elements (`<a class="x">Hello</a>` → "Hello") and
// elements whose meaningful text is wrapped in nested spans
// (`<span class="x"><span>Hello</span>…</span>` → "Hello").
//
// Returns "" if the marker isn't found, the element has no text,
// or the first text run is cut off by the end of html.
func firstInnerText(html, marker string) string {
	idx := strings.Index(html, marker)
	if idx < 0 {
		return ""
	}
	sub := html[idx:]
	gt := strings.Index(sub, ">")
	if gt < 0 {
		return ""
	}
	sub = sub[gt+1:]

	var b strings.Builder
	inTag := false
	for _, r := range sub {
		if inTag {
			if r == '>' {
				inTag = false
			}
			continue
		}
		if r == '<' {
			// If we've collected meaningful text, the next tag
			// terminates the run. Otherwise we're still skipping
			// leading whitespace + nested opening tags, so reset
			// and keep going.
			if strings.TrimSpace(b.String()) != "" {
				break
			}
			b.Reset()
			inTag = true
			continue
		}
		b.WriteRune(r)
	}
	return normalizeWhitespace(b.String())
}

// normalizeWhitespace trims and collapses interior whitespace runs
// to single spaces.
func normalizeWhitespace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
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
