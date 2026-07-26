// Package steamsession performs the Steam login flow and turns it into
// something the rest of this module can use: access/refresh tokens and
// authenticated web cookies for steamcommunity.com and
// store.steampowered.com.
//
// A Session is created with New and logged in either interactively
// (StartWithCredentials followed by SubmitSteamGuardCode and
// PollAuthSessionStatus) or in one shot with LoginWithDeviceCode when the
// account's shared_secret is available for TOTP generation. Sessions
// persist to disk with SaveToFile/LoadFromFile so long-running programs
// can skip login entirely across restarts.
//
// HTTPClient returns an *http.Client whose transport injects the session's
// cookies and refreshes expired access tokens automatically; pass it to
// steamcommunity.New, steamstore.New, or steamapi.New via their
// WithHTTPClient options.
package steamsession
