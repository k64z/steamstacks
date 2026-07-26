// Package steamid converts between the textual and numeric forms of a
// Steam identity: SteamID64 (76561198000000000), Steam2
// ("STEAM_0:0:12345"), Steam3 ("[U:1:12345]"), and the bare account ID.
//
// The core type is SteamID, a uint64 bit field with accessors for the
// universe, account type, instance, and account ID components. Values are
// cheap to copy and comparable, so they can be used directly as map keys.
package steamid
