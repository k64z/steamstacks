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

	"github.com/k64z/steamstacks/steamapi"
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
	ErrMarketItemServerDown        = errors.New("market: game's item server may be down")
	ErrMarketPendingConfirmation   = errors.New("market: listing pending confirmation for this item")
	ErrMarketItemNotInInventory    = errors.New("market: item no longer in inventory")
	ErrMarketListingProblem        = errors.New("market: generic listing problem; retry")
	ErrMarketWalletTooMuchMoney    = errors.New("market: wallet holds too much money")
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
		return nil, steamapi.HTTPStatusError(resp.StatusCode, body)
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
		return nil, steamapi.HTTPStatusError(resp.StatusCode, body)
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

// pendingListingRE matches only rows in Steam's "Listings awaiting
// confirmation" section. Active-listing rows render RemoveMarketListing(...)
// in their cancel-button href; pending rows render
// CancelMarketListingConfirmation(...) instead, so this regex won't
// pick up active rows. Capture groups: [1]=listing_id, [2]=asset_id.
var pendingListingRE = regexp.MustCompile(
	`CancelMarketListingConfirmation\('mylisting',\s*'(\d+)',\s*\d+,\s*'\d+',\s*'(\d+)'\)`,
)

// PendingListing represents one row in Steam's "Listings awaiting
// confirmation" section — a listing that was created via SellMarketItem
// but whose mobile confirmation was never accepted (and may already have
// expired off /mobileconf/getlist, leaving the listing orphaned).
type PendingListing struct {
	ListingID string
	AssetID   string
}

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

// GetMyPendingMarketListings scrapes pending-confirmation rows from the
// authenticated /market/ page. These are listings created via
// SellMarketItem that Steam still considers pending (mobile confirmation
// not accepted, or the confirmation expired before being accepted).
// The /mylistings/render/ JSON endpoint does NOT surface these, so an
// HTML scrape is the only path. Returns deduplicated entries in
// document order; empty slice when the section is empty.
func (c *Community) GetMyPendingMarketListings(ctx context.Context) ([]PendingListing, error) {
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

	matches := pendingListingRE.FindAllStringSubmatch(string(body), -1)
	seen := make(map[string]bool, len(matches))
	out := make([]PendingListing, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, PendingListing{ListingID: m[1], AssetID: m[2]})
	}
	return out, nil
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

// browserUserAgent is a common-browser UA. Steam serves a stripped page
// (no embedded user/wallet scripts) to the Go default UA on several market
// routes, so the fetches that need that embedded data send this instead.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// WalletFeeInfo is the Steam Community Market fee configuration for the
// authenticated account's wallet currency, parsed from the g_rgWalletInfo
// block Steam embeds in the classic /market/ home page. These are the exact
// values Steam's own sell-fee calculator (CalculateFeeAmount) uses.
// FeeMinimum is the per-component floor — Steam's $0.01 base converted into
// the wallet currency, so it tracks the exchange rate. All *minor-unit
// fields are integers in Currency.
type WalletFeeInfo struct {
	Currency                   int     // Steam currency code
	FeeMinimum                 int     // per-component fee floor, minor units
	FeeBase                    int     // flat addition to the Steam fee, minor units
	FeePercent                 float64 // Steam transaction fee fraction (e.g. 0.05)
	PublisherFeePercentDefault float64 // default publisher fee fraction (e.g. 0.10)
	CurrencyIncrement          int     // smallest accepted price step, minor units
	// Balance is the wallet's current funds and MaxBalance is Steam's
	// per-currency wallet cap (the "you have too much money" ceiling),
	// both minor units. Both are best-effort: 0 means the field was absent
	// from g_rgWalletInfo (a logged-out or stripped page omits them), so
	// callers must treat 0 as "unknown", not an authoritative empty wallet.
	Balance    int
	MaxBalance int
}

// ErrWalletFeeInfoUnavailable is returned when the /market/ page carries no
// g_rgWalletInfo block — e.g. a logged-out session, or Steam served the
// stripped React page (the latter is why this fetch sends a browser UA).
var ErrWalletFeeInfoUnavailable = errors.New("market: g_rgWalletInfo not present on market page")

var (
	walletFeeMinimumRE = regexp.MustCompile(`"wallet_fee_minimum":\s*"?([0-9]+)"?`)
	walletFeeBaseRE    = regexp.MustCompile(`"wallet_fee_base":\s*"?([0-9]+)"?`)
	walletFeePercentRE = regexp.MustCompile(`"wallet_fee_percent":\s*"?([0-9.]+)"?`)
	walletPubFeeRE     = regexp.MustCompile(`"wallet_publisher_fee_percent_default":\s*"?([0-9.]+)"?`)
	walletCurrencyRE   = regexp.MustCompile(`"wallet_currency":\s*"?([0-9]+)"?`)
	walletIncrementRE  = regexp.MustCompile(`"wallet_currency_increment":\s*"?([0-9]+)"?`)
	walletBalanceRE    = regexp.MustCompile(`"wallet_balance":\s*"?([0-9]+)"?`)
	walletMaxBalanceRE = regexp.MustCompile(`"wallet_max_balance":\s*"?([0-9]+)"?`)
)

// GetWalletFeeInfo fetches the authenticated account's market fee
// configuration from the g_rgWalletInfo block embedded in the classic
// /market/ home page. A browser User-Agent is required — Steam serves a
// stripped page without g_rgWalletInfo to the default Go UA. Returns
// ErrWalletFeeInfoUnavailable when the block (or its fee minimum) is absent.
func (c *Community) GetWalletFeeInfo(ctx context.Context) (*WalletFeeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://steamcommunity.com/market/", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

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
		return nil, steamapi.HTTPStatusError(resp.StatusCode, body)
	}
	return parseWalletFeeInfo(body)
}

// parseWalletFeeInfo extracts the fee fields from a /market/ page body. Split
// out so tests can feed a saved page directly. Steam quotes the numeric fee
// fields ("500", "0.05") and leaves wallet_currency bare; the regexes accept
// either. The fee minimum is required — its absence means no usable block.
func parseWalletFeeInfo(body []byte) (*WalletFeeInfo, error) {
	s := string(body)
	m := walletFeeMinimumRE.FindStringSubmatch(s)
	if len(m) != 2 {
		return nil, ErrWalletFeeInfoUnavailable
	}
	out := &WalletFeeInfo{}
	out.FeeMinimum, _ = strconv.Atoi(m[1])
	if g := walletFeeBaseRE.FindStringSubmatch(s); len(g) == 2 {
		out.FeeBase, _ = strconv.Atoi(g[1])
	}
	if g := walletCurrencyRE.FindStringSubmatch(s); len(g) == 2 {
		out.Currency, _ = strconv.Atoi(g[1])
	}
	if g := walletIncrementRE.FindStringSubmatch(s); len(g) == 2 {
		out.CurrencyIncrement, _ = strconv.Atoi(g[1])
	}
	if g := walletFeePercentRE.FindStringSubmatch(s); len(g) == 2 {
		out.FeePercent, _ = strconv.ParseFloat(g[1], 64)
	}
	if g := walletPubFeeRE.FindStringSubmatch(s); len(g) == 2 {
		out.PublisherFeePercentDefault, _ = strconv.ParseFloat(g[1], 64)
	}
	if g := walletBalanceRE.FindStringSubmatch(s); len(g) == 2 {
		out.Balance, _ = strconv.Atoi(g[1])
	}
	if g := walletMaxBalanceRE.FindStringSubmatch(s); len(g) == 2 {
		out.MaxBalance, _ = strconv.Atoi(g[1])
	}
	return out, nil
}

// MarketItemPageData is the order-book + recent-price summary parsed
// from the React-Query state Steam embeds in a /market/listings/ page
// under window.SSR.renderContext.
//
// All *Cents fields are integer minor units (cents/kopecks/tiyin) in
// Currency. HighestBuyCents and LowestSellCents are buyer-facing
// prices — what a buyer pays — matching how Steam renders the order
// book. MedianPriceCents is the most recent price-history median;
// Volume24h is the unit count sold in the trailing 24h of history.
type MarketItemPageData struct {
	// Currency is the Steam currency code Steam rendered the page in.
	// Authenticated requests get the account's wallet currency.
	Currency int

	HighestBuyCents int
	LowestSellCents int
	BuyOrderCount   int
	SellOrderCount  int

	MedianPriceCents int
	Volume24h        int

	// BuyOrders and SellOrders are the per-price-rung order book.
	// BuyOrders is sorted highest price first, SellOrders lowest
	// price first — i.e. both lead with the rung nearest the spread.
	BuyOrders  []MarketOrderLevel
	SellOrders []MarketOrderLevel

	// AddTax / TaxRate carry Steam's regional market tax (e.g.
	// Kazakhstan VAT). When AddTax is true and TaxRate > 0, the buyer
	// pays an extra `floor(fee * TaxRate/100 + 0.5)` on top of the
	// Steam + publisher fee — see Steam's own market JS (the `xa`
	// helper). TaxRate is a whole-number percent (12 == 12%). Zero
	// when the page carries no tax config (most regions).
	AddTax  bool
	TaxRate float64

	// PriceIncrement is the smallest price step the market accepts
	// for this currency, in minor units — the GCD of every order-book
	// level price. 1 for cent-granular currencies (USD/EUR/…); 100 for
	// currencies whose market only lists at whole major units (KZT
	// lists in whole tenge). Callers must snap listing prices to a
	// multiple of this; Steam silently re-rounds a finer-grained price.
	PriceIncrement int

	// FeeMinimum is Steam's per-currency `wallet_fee_minimum` — the
	// floor each fee component is clamped up to (`max(net*pct,
	// FeeMinimum)`). 1 for USD; larger for weak currencies (KZT ~500),
	// so cheap items pay a flat minimum fee rather than the percentage.
	// 0 when the page carries no wallet info (anonymous fetch).
	FeeMinimum int

	// Commodity is true for stackable, fungible items (metal, keys)
	// whose listings Steam pools into one order book.
	Commodity bool
}

// MarketOrderLevel is one price rung of the order book: the number of
// orders standing at exactly PriceCents (not cumulative depth).
type MarketOrderLevel struct {
	PriceCents int
	Count      int
}

// GetMarketItemPageData fetches a /market/listings/ page and parses
// the React-Query cache Steam embeds in it for the item's live order
// book and recent price history. One HTTP round-trip, no item_nameid
// lookup. Authenticated requests (cookie jar populated) get the data
// in the account's wallet currency; anonymous requests get Steam's
// geo default.
func (c *Community) GetMarketItemPageData(ctx context.Context, appID int, marketHashName string) (*MarketItemPageData, error) {
	reqURL := "https://steamcommunity.com/market/listings/" +
		strconv.Itoa(appID) + "/" + url.PathEscape(marketHashName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	// A common-browser UA avoids the stripped "install Steam" fallback
	// page Steam serves to the Go default UA on some listing routes.
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

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
		return nil, steamapi.HTTPStatusError(resp.StatusCode, body)
	}
	return parseMarketItemPage(body)
}

// renderCtxMarker prefixes the React SSR state blob. The value after
// it is `JSON.parse("<escaped json>")` — i.e. the argument is a JSON
// string literal whose decoded contents are themselves JSON.
const renderCtxMarker = "window.SSR.renderContext=JSON.parse("

// titleRE pulls the first <title> contents out of an HTML body so a
// parse failure can attach a one-line hint about what Steam served.
var titleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// taxRateRE / addTaxRE match Steam's regional market-tax config. The
// values live in the page's server-info block (only present for a
// logged-in user in a taxed region), so the patterns scan the whole
// page body rather than a single embedded blob.
var (
	taxRateRE = regexp.MustCompile(`"tradefee_taxrate":\s*([0-9.]+)`)
	addTaxRE  = regexp.MustCompile(`"tradefee_addtax":\s*(true|false)`)
	// feeMinimumRE matches Steam's per-currency minimum fee from the
	// logged-in wallet_info block. The value may be a JSON number or a
	// quoted string.
	feeMinimumRE = regexp.MustCompile(`"wallet_fee_minimum":\s*"?([0-9]+)"?`)
)

// parseMarketItemPage extracts the order book + price history from
// the embedded React-Query cache. Split out from the HTTP path so
// unit tests can feed a saved page body directly.
func parseMarketItemPage(body []byte) (*MarketItemPageData, error) {
	s := string(body)
	idx := strings.Index(s, renderCtxMarker)
	if idx < 0 {
		return nil, fmt.Errorf("market: renderContext not found on page (%s)", pageTitle(body))
	}
	// The argument to JSON.parse is a JSON string literal; decoding it
	// once yields the renderContext JSON text.
	dec := json.NewDecoder(strings.NewReader(s[idx+len(renderCtxMarker):]))
	var renderCtxJSON string
	if err := dec.Decode(&renderCtxJSON); err != nil {
		return nil, fmt.Errorf("market: decode renderContext arg: %w", err)
	}
	// renderContext.queryData is *itself* a JSON string (double-encoded).
	var rc struct {
		QueryData string `json:"queryData"`
	}
	if err := json.Unmarshal([]byte(renderCtxJSON), &rc); err != nil {
		return nil, fmt.Errorf("market: decode renderContext: %w", err)
	}
	var qd struct {
		Queries []struct {
			QueryKey []json.RawMessage `json:"queryKey"`
			State    struct {
				Data json.RawMessage `json:"data"`
			} `json:"state"`
		} `json:"queries"`
	}
	if err := json.Unmarshal([]byte(rc.QueryData), &qd); err != nil {
		return nil, fmt.Errorf("market: decode queryData: %w", err)
	}

	out := &MarketItemPageData{}
	// Regional market tax (e.g. Kazakhstan VAT). Scanned over the whole
	// page body — the config sits in the server-info block, not the
	// renderContext blob.
	if m := addTaxRE.FindString(s); m != "" {
		out.AddTax = strings.HasSuffix(m, "true")
	}
	if m := taxRateRE.FindStringSubmatch(s); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			out.TaxRate = v
		}
	}
	if m := feeMinimumRE.FindStringSubmatch(s); len(m) == 2 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			out.FeeMinimum = v
		}
	}
	foundOrderbook := false
	for _, q := range qd.Queries {
		switch marketQueryKind(q.QueryKey) {
		case "orderbook":
			var ob struct {
				AmtMaxBuyOrder  int   `json:"amtMaxBuyOrder"`
				AmtMinSellOrder int   `json:"amtMinSellOrder"`
				ECurrency       int   `json:"eCurrency"`
				CBuyOrders      int   `json:"cBuyOrders"`
				CSellOrders     int   `json:"cSellOrders"`
				RgCompactBuy    []int `json:"rgCompactBuyOrders"`
				RgCompactSell   []int `json:"rgCompactSellOrders"`
			}
			if err := json.Unmarshal(q.State.Data, &ob); err != nil {
				continue
			}
			out.HighestBuyCents = ob.AmtMaxBuyOrder
			out.LowestSellCents = ob.AmtMinSellOrder
			out.BuyOrderCount = ob.CBuyOrders
			out.SellOrderCount = ob.CSellOrders
			out.BuyOrders = pairsToLevels(ob.RgCompactBuy)
			out.SellOrders = pairsToLevels(ob.RgCompactSell)
			if ob.ECurrency != 0 {
				out.Currency = ob.ECurrency
			}
			foundOrderbook = true
		case "pricehistory":
			var ph struct {
				ECurrency int `json:"ecurrency"`
				Prices    []struct {
					Time        int64   `json:"time"`
					PriceMedian float64 `json:"price_median"`
					Purchases   int     `json:"purchases"`
				} `json:"prices"`
			}
			if err := json.Unmarshal(q.State.Data, &ph); err != nil {
				continue
			}
			if n := len(ph.Prices); n > 0 {
				last := ph.Prices[n-1]
				out.MedianPriceCents = int(last.PriceMedian*100 + 0.5)
				// Steam keeps hourly granularity for recent history;
				// sum purchases over the trailing 24h of that history.
				cutoff := last.Time - 24*3600
				for i := n - 1; i >= 0 && ph.Prices[i].Time >= cutoff; i-- {
					out.Volume24h += ph.Prices[i].Purchases
				}
			}
			if out.Currency == 0 && ph.ECurrency != 0 {
				out.Currency = ph.ECurrency
			}
		case "description":
			var d struct {
				Commodity int `json:"commodity"`
			}
			if err := json.Unmarshal(q.State.Data, &d); err == nil {
				out.Commodity = d.Commodity == 1
			}
		}
	}
	if !foundOrderbook {
		return nil, errors.New("market: orderbook data not present in page state")
	}
	out.PriceIncrement = priceIncrement(out.BuyOrders, out.SellOrders)
	// wallet_fee_minimum isn't exposed in the page's React state, so
	// fall back to the known per-currency table when the regex above
	// found nothing.
	if out.FeeMinimum == 0 {
		out.FeeMinimum = feeMinimumForCurrency(out.Currency)
	}
	return out, nil
}

// currencyKZT is the Steam currency code for Kazakhstani tenge.
const currencyKZT = 37

// feeMinimumForCurrency returns Steam's `wallet_fee_minimum` — the
// floor each market fee component is clamped up to — for currencies
// where it isn't 1 minor unit. Steam keeps these in per-user
// g_rgWalletInfo, which the new React market page does not serialize;
// values here are observed empirically. Weak currencies (KZT) use a
// larger minimum so cheap items still pay a meaningful fee. Unknown
// currencies default to 1 (the value for USD/EUR/GBP and most others).
func feeMinimumForCurrency(currency int) int {
	switch currency {
	case currencyKZT: // confirmed: a 34₸ item's fee is a flat 5₸+5₸.
		return 500
	default:
		return 1
	}
}

// priceIncrement infers the market's minimum price step for this
// currency: the GCD of every order-book level price. Every standing
// order sits at a price Steam accepted, so their GCD is the listing
// granularity (1 for cent-granular currencies, 100 for whole-major-
// unit currencies like KZT). Falls back to 1 when the book is too
// thin to trust the GCD.
func priceIncrement(buy, sell []MarketOrderLevel) int {
	g, n := 0, 0
	for _, lvl := range buy {
		if lvl.PriceCents > 0 {
			g = gcdInt(g, lvl.PriceCents)
			n++
		}
	}
	for _, lvl := range sell {
		if lvl.PriceCents > 0 {
			g = gcdInt(g, lvl.PriceCents)
			n++
		}
	}
	if n < 4 || g < 1 {
		return 1
	}
	return g
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// marketQueryKind classifies a React-Query key like
// ["market","orderbook",440,"Backpack Expander"] — returns the second
// element ("orderbook"/"pricehistory"/"description") when the first is
// "market", else "".
func marketQueryKind(key []json.RawMessage) string {
	if len(key) < 2 {
		return ""
	}
	var ns, kind string
	if json.Unmarshal(key[0], &ns) != nil || ns != "market" {
		return ""
	}
	if json.Unmarshal(key[1], &kind) != nil {
		return ""
	}
	return kind
}

func pageTitle(body []byte) string {
	m := titleRE.FindSubmatch(body)
	if len(m) < 2 {
		return "no <title>"
	}
	return strings.Join(strings.Fields(string(m[1])), " ")
}

// pairsToLevels turns Steam's flat [price0,count0,price1,count1,…]
// rgCompact array into typed order-book rungs. A trailing odd element
// (a price with no count) is dropped.
func pairsToLevels(flat []int) []MarketOrderLevel {
	out := make([]MarketOrderLevel, 0, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		out = append(out, MarketOrderLevel{PriceCents: flat[i], Count: flat[i+1]})
	}
	return out
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
		return steamapi.HTTPStatusError(resp.StatusCode, body)
	}
	return nil
}
