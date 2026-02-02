package steamcommunity

import (
	"encoding/base64"
	"testing"
)

func Test_getConfirmationKey(t *testing.T) {
	// Test vectors verified against node-steamcommunity/steam-totp implementations.
	// The identity secret is a known test value.
	identitySecret, err := base64.StdEncoding.DecodeString("SGVsbG9Xb3JsZFRlc3RTZWNyZXQh")
	if err != nil {
		t.Fatalf("decode identity secret: %v", err)
	}

	tests := []struct {
		name      string
		timestamp int64
		tag       string
		expected  string
	}{
		{
			name:      "list tag",
			timestamp: 1706889600,
			tag:       "list",
			expected:  "Nz4pGHHZ9Eqs1vkEKxisyzjpTcs=",
		},
		{
			name:      "accept tag",
			timestamp: 1706889600,
			tag:       "accept",
			expected:  "6POLFuEeetQjWwqECs//LROSa7w=",
		},
		{
			name:      "reject tag",
			timestamp: 1706889600,
			tag:       "reject",
			expected:  "PFeZ6/f7PrTbUC1uLPsmQT6VVAA=",
		},
		{
			name:      "empty tag",
			timestamp: 1706889600,
			tag:       "",
			expected:  "ihrP4qEavQZZmllRD2GtWS7x0CQ=",
		},
		{
			name:      "different timestamp",
			timestamp: 1700000000,
			tag:       "list",
			expected:  "tsxOja9kxppXR4vjyiOR82WpQG8=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getConfirmationKey(identitySecret, tt.timestamp, tt.tag)
			if got != tt.expected {
				t.Errorf("getConfirmationKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func Test_getDeviceID(t *testing.T) {
	tests := []struct {
		name      string
		steamID64 uint64
		expected  string
	}{
		{
			name:      "typical steamid64",
			steamID64: 76561198012345678,
			expected:  "android:ab17d684-7c3f-7758-8af3-1836e87daac5",
		},
		{
			name:      "another steamid64",
			steamID64: 76561198000000000,
			expected:  "android:5c9df5a2-d7de-1e2c-8fc8-766523ca130f",
		},
		{
			name:      "minimum valid steamid64",
			steamID64: 76561197960265728,
			expected:  "android:63e01aa8-e99c-42c4-ef4c-e78bd041f129",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDeviceID(tt.steamID64)
			if got != tt.expected {
				t.Errorf("getDeviceID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestConfirmationType_String(t *testing.T) {
	tests := []struct {
		typ      ConfirmationType
		expected string
	}{
		{ConfirmationTypeUnknown, "Unknown"},
		{ConfirmationTypeTrade, "Trade"},
		{ConfirmationTypeMarketListing, "Market Listing"},
		{ConfirmationType(999), "Unknown"}, // Unknown type
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.typ.String()
			if got != tt.expected {
				t.Errorf("ConfirmationType(%d).String() = %q, want %q", tt.typ, got, tt.expected)
			}
		})
	}
}
