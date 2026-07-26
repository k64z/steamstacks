// Package steamtotp implements the Steam Guard mobile authenticator
// algorithms: the 5-character login codes (GenerateAuthCode), the
// confirmation keys used to accept trade and market confirmations
// (GenerateConfirmationKey), and the device ID derived from a SteamID64
// (GetDeviceID).
//
// The shared_secret and identity_secret inputs are the base64 values from
// a mobile authenticator (maFile or equivalent). Keep them out of source
// control; every function here accepts them as arguments so callers can
// load them from the environment or a secret store.
package steamtotp
