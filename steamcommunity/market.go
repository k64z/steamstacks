package steamcommunity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	ErrMarketItemServerDown        = errors.New("steamcommunity: game's item server may be down")
	ErrMarketPendingConfirmation   = errors.New("steamcommunity: listing pending confirmation for this item")
	ErrMarketItemNotInInventory    = errors.New("steamcommunity: item no longer in inventory")
	ErrMarketListingProblem        = errors.New("steamcommunity: generic listing problem; retry")
	ErrMarketWalletTooMuchMoney    = errors.New("steamcommunity: wallet holds too much money")
	ErrMarketPreviousActionPending = errors.New("steamcommunity: previous action still pending")
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

// marketListingsPageSize is the page size GetMyMarketListingIDs asks the
// render endpoint for — Steam's documented maximum. The /market/ home page
// this replaced yielded ~10 per load, so a full delist walk now costs an
// order of magnitude fewer requests.
const marketListingsPageSize = 100

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

// GetMyMarketListingIDs returns the first page of the authenticated user's
// active market listing IDs, deduplicated, in document order.
//
// It reads the /mylistings/render/ JSON endpoint (via GetMarketListings),
// NOT the /market/ home page. The home page renders only ~10 listings per
// load and is the most rate-limited route on steamcommunity.com, so callers
// that paginate by "cancel this page, re-fetch, repeat" issued one heavyweight
// request per 10 listings and reliably 429'd the IP — which then broke
// unrelated fetches, GetWalletFeeInfo above most visibly. The render endpoint
// serves 100 per page and is far cheaper.
//
// Callers that cancel what they get and loop must re-call this: Steam
// re-pages after each cancellation, and the endpoint has read-your-write lag,
// so a returned ID may already be gone.
func (c *Community) GetMyMarketListingIDs(ctx context.Context) ([]string, error) {
	page, err := c.GetMarketListings(ctx, 0, marketListingsPageSize)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(page.Listings))
	ids := make([]string, 0, len(page.Listings))
	for _, l := range page.Listings {
		if l.ID == "" || seen[l.ID] {
			continue
		}
		seen[l.ID] = true
		ids = append(ids, l.ID)
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
var ErrWalletFeeInfoUnavailable = errors.New("steamcommunity: g_rgWalletInfo not present on market page")

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
//
// This is an expensive read: /market/ is the most aggressively rate-limited
// route on steamcommunity.com, and a 429 here also poisons every other
// community fetch from the same IP. Callers that only need the balance
// should use GetWalletBalance, which is a ~200-byte JSON read on a
// different host. As of 2026-07 g_rgWalletInfo survives ONLY on /market/ —
// verified absent from /market/search, /market/listings/<app>/<name>,
// /market/mylistings, the store front page and /steamaccount/addfunds —
// so there is no cheaper source for the *fee* fields.
func (c *Community) GetWalletFeeInfo(ctx context.Context) (*WalletFeeInfo, error) {
	body, err := c.fetchBrowserPage(ctx, "https://steamcommunity.com/market/")
	if err != nil {
		return nil, err
	}
	return parseWalletFeeInfo(body)
}

// WalletBalance is the authenticated account's wallet funds, read from the
// store's add-funds JSON endpoint rather than from g_rgWalletInfo. It carries
// no fee fields — those live only on the /market/ home page (GetWalletFeeInfo).
type WalletBalance struct {
	// Balance is the wallet's current funds in minor units of CurrencyCode.
	Balance int
	// CurrencyCode is the ISO 4217 code Steam reports ("RUB", "USD").
	CurrencyCode string
	// Currency is CurrencyCode mapped to Steam's numeric currency id, or 0
	// when the code isn't in walletCurrencyIDs. Treat 0 as "unverified" —
	// it is NOT USD.
	Currency int
	// CountryCode is the wallet's country ("RU"); best-effort, may be empty.
	CountryCode string
}

// ErrWalletBalanceUnavailable is returned when getfundwalletinfo answers
// without a user_wallet block — a logged-out session, or an account with no
// wallet in this region.
var ErrWalletBalanceUnavailable = errors.New("steamcommunity: user_wallet not present in getfundwalletinfo response")

// fundWalletInfoResponse mirrors the JSON shape of
// store.steampowered.com/api/getfundwalletinfo. Only the fields we use are
// modelled; the rest of the payload drives the store's add-funds picker.
// Amounts are stringified minor units, as everywhere else in Steam's economy.
type fundWalletInfoResponse struct {
	Success     int    `json:"success"`
	Currency    string `json:"currency"`
	CountryCode string `json:"country_code"`
	UserWallet  *struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	} `json:"user_wallet"`
}

// walletCurrencyIDs maps the ISO codes getfundwalletinfo returns onto Steam's
// numeric currency ids (ECurrencyCode — see steamapi.xpaw.me), which is what
// g_rgWalletInfo and the rest of the market API speak. Codes absent here
// resolve to 0 so a caller can tell "we couldn't verify the currency" from a
// positive match; never guess, because a wrong id mislabels real money.
var walletCurrencyIDs = map[string]int{
	"USD": 1, "GBP": 2, "EUR": 3, "CHF": 4, "RUB": 5, "PLN": 6,
	"BRL": 7, "JPY": 8, "NOK": 9, "IDR": 10, "MYR": 11, "PHP": 12,
	"SGD": 13, "THB": 14, "VND": 15, "KRW": 16, "TRY": 17, "UAH": 18,
	"MXN": 19, "CAD": 20, "AUD": 21, "NZD": 22, "CNY": 23, "INR": 24,
	"CLP": 25, "PEN": 26, "COP": 27, "ZAR": 28, "HKD": 29, "TWD": 30,
	"SAR": 31, "AED": 32, "SEK": 33, "ARS": 34, "ILS": 35, "BYN": 36,
	"KZT": 37, "KWD": 38, "QAR": 39, "CRC": 40, "UYU": 41, "BGN": 42,
	"HRK": 43, "CZK": 44, "DKK": 45, "HUF": 46, "RON": 47,
}

// GetWalletBalance fetches the authenticated account's wallet balance from
// store.steampowered.com/api/getfundwalletinfo — the JSON the store's
// add-funds flow reads. Two reasons to prefer it over GetWalletFeeInfo when
// only the balance is wanted: the response is ~200 bytes rather than a full
// market home page, and it lives on store.steampowered.com, a different host
// from steamcommunity.com and therefore a different rate-limit bucket, so
// polling it can't 429 the market fetches. The session seeds login cookies
// for both hosts (steamsession/webcookies.go), so no extra auth is needed.
func (c *Community) GetWalletBalance(ctx context.Context) (*WalletBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://store.steampowered.com/api/getfundwalletinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", browserUserAgent)

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
	return parseWalletBalance(body)
}

// parseWalletBalance decodes a getfundwalletinfo payload. Split out so tests
// can feed a saved response directly.
func parseWalletBalance(body []byte) (*WalletBalance, error) {
	var r fundWalletInfoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if r.Success != 1 || r.UserWallet == nil {
		return nil, ErrWalletBalanceUnavailable
	}
	amount, err := strconv.Atoi(strings.TrimSpace(r.UserWallet.Amount))
	if err != nil {
		return nil, fmt.Errorf("parse wallet amount %q: %w", r.UserWallet.Amount, err)
	}
	// user_wallet.currency is the wallet's own currency; the top-level
	// `currency` is the storefront's, which can differ while travelling.
	code := r.UserWallet.Currency
	if code == "" {
		code = r.Currency
	}
	return &WalletBalance{
		Balance:      amount,
		CurrencyCode: code,
		Currency:     walletCurrencyIDs[strings.ToUpper(code)],
		CountryCode:  r.CountryCode,
	}, nil
}

// fetchBrowserPage GETs pageURL with a common-browser User-Agent and
// returns the response body. Steam serves a stripped page (without the
// embedded user/wallet scripts) to the Go default UA on several market
// routes, so the fetches that need that embedded data go through here.
func (c *Community) fetchBrowserPage(ctx context.Context, pageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
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
	return body, nil
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

	// A field whose regex matched but whose value fails to parse means a
	// malformed g_rgWalletInfo block; fail rather than silently returning
	// zero-valued money fields.
	var perr error
	atoi := func(name, v string) int {
		n, err := strconv.Atoi(v)
		if err != nil && perr == nil {
			perr = fmt.Errorf("parse %s %q: %w", name, v, err)
		}
		return n
	}
	atof := func(name, v string) float64 {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil && perr == nil {
			perr = fmt.Errorf("parse %s %q: %w", name, v, err)
		}
		return f
	}

	out := &WalletFeeInfo{}
	out.FeeMinimum = atoi("fee minimum", m[1])
	if g := walletFeeBaseRE.FindStringSubmatch(s); len(g) == 2 {
		out.FeeBase = atoi("fee base", g[1])
	}
	if g := walletCurrencyRE.FindStringSubmatch(s); len(g) == 2 {
		out.Currency = atoi("wallet currency", g[1])
	}
	if g := walletIncrementRE.FindStringSubmatch(s); len(g) == 2 {
		out.CurrencyIncrement = atoi("currency increment", g[1])
	}
	if g := walletFeePercentRE.FindStringSubmatch(s); len(g) == 2 {
		out.FeePercent = atof("fee percent", g[1])
	}
	if g := walletPubFeeRE.FindStringSubmatch(s); len(g) == 2 {
		out.PublisherFeePercentDefault = atof("publisher fee percent", g[1])
	}
	if g := walletBalanceRE.FindStringSubmatch(s); len(g) == 2 {
		out.Balance = atoi("wallet balance", g[1])
	}
	if g := walletMaxBalanceRE.FindStringSubmatch(s); len(g) == 2 {
		out.MaxBalance = atoi("max balance", g[1])
	}
	if perr != nil {
		return nil, perr
	}
	return out, nil
}

// MarketItemPageData is one item's order-book + recent-price summary,
// fetched from the JSON loaders the React market frontend uses (the
// /market/orderbook queryAction route + the classic
// /market/pricehistory/ endpoint). The name is historical — until
// Steam's 2026-07 client-shell migration this was parsed from the
// React-Query state embedded in the /market/listings/ page.
//
// All *Cents fields are integer minor units (cents/kopecks/tiyin) in
// Currency. HighestBuyCents and LowestSellCents are buyer-facing
// prices — what a buyer pays — matching how Steam renders the order
// book. MedianPriceCents is the most recent price-history median;
// Volume24h is the unit count sold in the trailing 24h of history.
type MarketItemPageData struct {
	// Currency is the Steam currency code the order book is priced in.
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
	// Always sourced from the hand-observed feeMinimumForCurrency table
	// (the market JSON loaders don't expose it).
	FeeMinimum int
}

// MarketOrderLevel is one price rung of the order book: the number of
// orders standing at exactly PriceCents (not cumulative depth).
type MarketOrderLevel struct {
	PriceCents int
	Count      int
}

// GetMarketItemPageData fetches an item's live order book and recent
// price history from the JSON loaders the React market frontend uses
// (no /market/listings/ page fetch — Steam has been converting that
// page to a client-side shell without embedded market data since
// 2026-07-10). Authenticated requests (cookie jar populated) get the
// data in the account's wallet currency; anonymous requests get
// Steam's geo default. Median/volume are best-effort: the classic
// /market/pricehistory/ endpoint is login-gated (anonymous sessions
// get a bare `[]`), and history failures degrade to zero instead of
// failing the fetch.
func (c *Community) GetMarketItemPageData(ctx context.Context, appID int, marketHashName string) (*MarketItemPageData, error) {
	ob, err := c.fetchOrderbookQueryAction(ctx, appID, marketHashName)
	if err != nil {
		return nil, fmt.Errorf("fetch orderbook: %w", err)
	}
	out := &MarketItemPageData{}
	out.applyOrderbook(ob)
	_ = c.fillPriceHistoryClassic(ctx, appID, marketHashName, out)
	finalizeMarketItemPageData(out)
	return out, nil
}

// finalizeMarketItemPageData derives the fields that depend on the
// resolved currency, whichever path (page state or queryAction
// fallback) supplied the order book.
func finalizeMarketItemPageData(out *MarketItemPageData) {
	out.PriceIncrement = currencyIncrement(out.Currency)
	// wallet_fee_minimum isn't exposed in the page's React state, so
	// fall back to the known per-currency table when the page regex
	// found nothing.
	if out.FeeMinimum == 0 {
		out.FeeMinimum = feeMinimumForCurrency(out.Currency)
	}
}

// marketOrderbookState is the ["market","orderbook",…] React-Query
// payload — identical whether it arrives dehydrated in the page's SSR
// state or from the /market/orderbook queryAction route. The rgCompact
// slices are flat [price, count, price, count, …] pairs.
type marketOrderbookState struct {
	AmtMaxBuyOrder  int   `json:"amtMaxBuyOrder"`
	AmtMinSellOrder int   `json:"amtMinSellOrder"`
	ECurrency       int   `json:"eCurrency"`
	CBuyOrders      int   `json:"cBuyOrders"`
	CSellOrders     int   `json:"cSellOrders"`
	RgCompactBuy    []int `json:"rgCompactBuyOrders"`
	RgCompactSell   []int `json:"rgCompactSellOrders"`
}

func (out *MarketItemPageData) applyOrderbook(ob *marketOrderbookState) {
	out.HighestBuyCents = ob.AmtMaxBuyOrder
	out.LowestSellCents = ob.AmtMinSellOrder
	out.BuyOrderCount = ob.CBuyOrders
	out.SellOrderCount = ob.CSellOrders
	out.BuyOrders = pairsToLevels(ob.RgCompactBuy)
	out.SellOrders = pairsToLevels(ob.RgCompactSell)
	if ob.ECurrency != 0 {
		out.Currency = ob.ECurrency
	}
}

// fetchOrderbookQueryAction fetches the order book from the JSON
// loader route the client-shell market page uses:
// GET /market/orderbook?q=Load&qp=[appid,"name"] with an
// x-valve-request-type: queryAction header (without the header Steam
// serves the HTML page instead). Works anonymously (geo-default
// currency); authenticated sessions get the wallet currency.
func (c *Community) fetchOrderbookQueryAction(ctx context.Context, appID int, marketHashName string) (*marketOrderbookState, error) {
	qp, err := json.Marshal([]any{appID, marketHashName})
	if err != nil {
		return nil, fmt.Errorf("marshal qp: %w", err)
	}
	q := url.Values{"q": {"Load"}, "qp": {string(qp)}}
	reqURL := "https://steamcommunity.com/market/orderbook?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("x-valve-request-type", "queryAction")

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
	var envelope struct {
		Success bool                  `json:"success"`
		Data    *marketOrderbookState `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode orderbook response: %w", err)
	}
	if !envelope.Success || envelope.Data == nil {
		return nil, errors.New("orderbook response missing data")
	}
	return envelope.Data, nil
}

// priceHistoryTimeLayout parses the classic /market/pricehistory/
// timestamps ("Jul 10 2026 23: +0" — hourly buckets for recent
// history, daily further back; the "+0" zone is a fixed literal).
const priceHistoryTimeLayout = "Jan 02 2006 15: +0"

// fillPriceHistoryClassic populates MedianPriceCents/Volume24h from
// the classic /market/pricehistory/ endpoint, mirroring the math the
// page-state path applies to its pricehistory query. The endpoint is
// login-gated: anonymous sessions get a bare `[]`, which surfaces here
// as a decode error the caller treats as "history unavailable".
func (c *Community) fillPriceHistoryClassic(ctx context.Context, appID int, marketHashName string, out *MarketItemPageData) error {
	q := url.Values{
		"appid":            {strconv.Itoa(appID)},
		"market_hash_name": {marketHashName},
	}
	body, err := c.fetchBrowserPage(ctx, "https://steamcommunity.com/market/pricehistory/?"+q.Encode())
	if err != nil {
		return err
	}
	var ph struct {
		Success bool                `json:"success"`
		Prices  [][]json.RawMessage `json:"prices"`
	}
	if err := json.Unmarshal(body, &ph); err != nil {
		return fmt.Errorf("decode pricehistory response: %w", err)
	}
	if !ph.Success || len(ph.Prices) == 0 {
		return errors.New("pricehistory response missing prices")
	}
	type point struct {
		ts        int64
		median    float64
		purchases int
	}
	points := make([]point, 0, len(ph.Prices))
	for _, row := range ph.Prices {
		// Each row is a mixed-type triple: ["Jul 10 2026 23: +0", 178.449, "2074"].
		if len(row) != 3 {
			continue
		}
		var (
			when   string
			median float64
			count  string
		)
		if json.Unmarshal(row[0], &when) != nil ||
			json.Unmarshal(row[1], &median) != nil ||
			json.Unmarshal(row[2], &count) != nil {
			continue
		}
		t, err := time.Parse(priceHistoryTimeLayout, when)
		if err != nil {
			continue
		}
		n, err := strconv.Atoi(count)
		if err != nil {
			continue
		}
		points = append(points, point{ts: t.Unix(), median: median, purchases: n})
	}
	if len(points) == 0 {
		return errors.New("pricehistory response had no parseable rows")
	}
	last := points[len(points)-1]
	out.MedianPriceCents = int(math.Round(last.median * 100))
	// Steam keeps hourly granularity for recent history; sum purchases
	// over the trailing 24h of that history.
	cutoff := last.ts - 24*3600
	out.Volume24h = 0
	for i := len(points) - 1; i >= 0 && points[i].ts >= cutoff; i-- {
		out.Volume24h += points[i].purchases
	}
	return nil
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

// currencyIncrement returns the market's minimum listing price step, in
// minor units, for the given Steam currency code. It is a property of the
// currency, not the order book: Steam prices most currencies to the cent
// (step 1) and a few weak currencies in whole major units (KZT: step 100).
//
// Inferring the step from the GCD of sampled order-book prices — as an
// earlier version did — over-reports for cent-granular currencies whenever
// the sampled prices happen to share a common factor (e.g. an all-even
// USD book yields a GCD of 2), which would wrongly forbid odd-cent
// listings. Unknown currencies default to 1: under-reporting the step is
// harmless (1 is always an accepted multiple) whereas over-reporting is not.
func currencyIncrement(currency int) int {
	switch currency {
	case currencyKZT:
		return 100
	default:
		return 1
	}
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
		return steamapi.HTTPStatusErrorFromResponse(resp)
	}
	return nil
}
