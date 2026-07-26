// Package steamstore is an authenticated client for
// store.steampowered.com: account data, wallet balance and wallet codes,
// in-game item purchases, free licenses, gifting, phone number
// management, and display languages.
//
// Construct a Store with New, passing the authenticated *http.Client from
// steamsession's HTTPClient via WithHTTPClient. Store operations return
// typed StoreError values carrying Steam's EResult and EPurchaseResult
// codes so callers can react to specific failure reasons.
package steamstore
