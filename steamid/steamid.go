package steamid

import (
	"fmt"
	"strconv"
	"strings"
)

// SteamID represents a Steam identifier (from steamid.h)
type SteamID uint64

// EUniverse represents Steam universe types (from steammessages_base.proto)
type EUniverse uint32

// SetUniverse sets the universe part of the SteamID and returns the new SteamID.
func (s SteamID) SetUniverse(u int32) SteamID {
	s &= ^SteamID(0xFF << 56)     // Clear the universe part
	s |= SteamID(uint64(u) << 56) // Set the new universe
	return s
}

// Universe returns the universe part of the SteamID.
func (s SteamID) Universe() int32 {
	return int32(s >> 56)
}

// SetType sets the type part of the SteamID and returns the new SteamID.
func (s SteamID) SetType(t int32) SteamID {
	s &= ^SteamID(0xF << 52)      // Clear the type part
	s |= SteamID(uint64(t) << 52) // Set the new type
	return s
}

// Type returns the type part of the SteamID.
func (s SteamID) Type() int32 {
	return int32((s >> 52) & 0xF)
}

// SetInstance sets the instance part of the SteamID and returns the new SteamID.
func (s SteamID) SetInstance(i int32) SteamID {
	s &= ^SteamID(0xFFFFF << 32)  // Clear the instance part
	s |= SteamID(uint64(i) << 32) // Set the new instance
	return s
}

// SetAccountID sets the account ID part of the SteamID and returns the new SteamID.
func (s SteamID) SetAccountID(a uint32) SteamID {
	s &= ^SteamID(0xFFFFFFFF) // Clear the account ID part
	s |= SteamID(a)           // Set the new account ID
	return s
}

// AccountID returns the account ID part of the SteamID.
func (s SteamID) AccountID() uint32 {
	return uint32(s & 0xFFFFFFFF)
}

// FromSteam2ID returns a new SteamID based on the Steam2 ID format ("STEAM_X:Y:Z").
// Example: STEAM_1:1:278391449
func FromSteam2ID(id string) SteamID {
	// TODO: Error handling and validation
	var universe, mod, accountID uint32
	_, _ = fmt.Sscanf(id, "STEAM_%d:%d:%d", &universe, &mod, &accountID)

	if universe == 0 { // EUniverse_Invalid
		universe = 1 // EUniverse_Public
	}

	return SteamID(uint64(universe)<<56 | uint64(1)<<52 | uint64(1)<<32 | uint64(accountID*2+mod))
}

// FromSteam3ID returns a new SteamID based on the Steam3 ID format ("[U:1:Z]").
// Example: [U:1:556782899]
func FromSteam3ID(steam3ID string) SteamID {
	// TODO: Error handling and validation
	parts := strings.Split(strings.Trim(steam3ID, "[]"), ":")
	if len(parts) == 3 {
		z, _ := strconv.Atoi(parts[2])

		// Assuming public universe and individual accounts, return the new SteamID
		// TODO: Support other account types
		return SteamID(uint64(1)<<56 | uint64(1)<<52 | uint64(1)<<32 | uint64(z))
	}
	return 0 // Return 0 if the format is incorrect
}

// FromSteamID64 returns a new SteamID based on the SteamID64 format.
func FromSteamID64(steamID64 uint64) SteamID {
	return SteamID(steamID64)
}

// FromString takes a string ("765611...") and returns a new SteamID.
func FromString(str string) (SteamID, error) {
	num, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0, err // Return an error if parsing fails
	}
	return SteamID(num), nil // Return the parsed number as a SteamID
}

// ToSteam2ID returns the SteamID in Steam2 ID format ("STEAM_X:Y:Z").
func (s SteamID) ToSteam2ID() string {
	universe := s >> 56
	accountID := uint32(s & 0xFFFFFFFF)
	y := accountID % 2
	z := accountID / 2
	return fmt.Sprintf("STEAM_%d:%d:%d", universe, y, z)
}

// ToSteam3ID returns the SteamID in Steam3 ID format ("[U:1:Z]").
func (s SteamID) ToSteam3ID() string {
	accountID := uint32(s & 0xFFFFFFFF)
	return fmt.Sprintf("[U:1:%d]", accountID)
}

// ToSteamID64 returns the SteamID in SteamID64 format. Ex. 76561197960287930.
func (s SteamID) ToSteamID64() uint64 {
	return uint64(s)
}

// ToAccountID return the last part of Steam3ID. This can be used in trade offers.
// Example: 386798732
func (s SteamID) ToAccountID() uint64 {
	return uint64(s & 0xFFFFFFFF)
}

// String returns the SteamID as a string. Ex. "76561197960287930".
func (s SteamID) String() string {
	return strconv.FormatUint(uint64(s), 10)
}
