// Package steamcommunity is an authenticated client for
// steamcommunity.com: inventories, trade offers, mobile confirmations,
// friends, profiles, and the Community Market.
//
// Construct a Community with New, passing the authenticated *http.Client
// from steamsession's HTTPClient via WithHTTPClient. Steam exposes no
// official API for most of this surface, so the package speaks the same
// mix of JSON loaders and HTML pages the web frontend uses; endpoints are
// chosen to be the cheapest and least rate-limited route Steam offers for
// each operation, and doc comments record those tradeoffs where they are
// not obvious.
//
// Market operations that move real money (SellMarketItem, mobile
// confirmations) map Steam's undocumented failure modes onto typed
// sentinel errors so callers can distinguish retryable conditions from
// hard failures.
package steamcommunity
