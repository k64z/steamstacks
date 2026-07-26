// Package steamapi is a typed client for the official Web API at
// api.steampowered.com: authentication and two-factor service methods,
// trade offers (IEconService), asset class info, player summaries, the
// TF2 item schema, and Steam server time.
//
// Construct an API with New and authenticate it with SetAPIKey (classic
// key-based endpoints) or SetAccessToken (token-based service methods).
// Endpoint coverage is driven by what the rest of this module needs
// rather than completeness; unwrapped endpoints are straightforward to
// call through the same patterns.
package steamapi
