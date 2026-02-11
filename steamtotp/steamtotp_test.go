package steamtotp

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestGenerateAuthCode(t *testing.T) {
	// Test vectors generated using the same algorithm as node-steam-totp.
	// Shared secret (base64): "t9MKLkm2D2GIG7bABTxjH7JIF/k="
	// Shared secret (hex): "b7d30a2e49b60f61881bb6c0053c631fb24817f9"

	base64Secret := "t9MKLkm2D2GIG7bABTxjH7JIF/k="
	hexSecret := "b7d30a2e49b60f61881bb6c0053c631fb24817f9"

	tests := []struct {
		name     string
		secret   string
		time     int64
		expected string
	}{
		{
			name:     "base64 secret, timestamp 1706889600",
			secret:   base64Secret,
			time:     1706889600,
			expected: "274WN",
		},
		{
			name:     "base64 secret, timestamp 1700000000",
			secret:   base64Secret,
			time:     1700000000,
			expected: "5GH26",
		},
		{
			name:     "base64 secret, timestamp 0",
			secret:   base64Secret,
			time:     0,
			expected: "GWQQ8",
		},
		{
			name:     "hex secret, timestamp 1706889600",
			secret:   hexSecret,
			time:     1706889600,
			expected: "274WN",
		},
		{
			name:     "hex secret, timestamp 1700000000",
			secret:   hexSecret,
			time:     1700000000,
			expected: "5GH26",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a fixed time by computing the offset from Now.
			offset := tt.time - time.Now().Unix()
			got, err := GenerateAuthCode(tt.secret, offset)
			if err != nil {
				t.Fatalf("GenerateAuthCode() error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("GenerateAuthCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGenerateAuthCode_InvalidSecret(t *testing.T) {
	_, err := GenerateAuthCode("not-valid-base64!!!", 0)
	if err == nil {
		t.Error("GenerateAuthCode() expected error for invalid secret, got nil")
	}
}

func TestGenerateConfirmationKey(t *testing.T) {
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
			got := GenerateConfirmationKey(identitySecret, tt.timestamp, tt.tag)
			if got != tt.expected {
				t.Errorf("GenerateConfirmationKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetDeviceID(t *testing.T) {
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
			got := GetDeviceID(tt.steamID64)
			if got != tt.expected {
				t.Errorf("GetDeviceID() = %q, want %q", got, tt.expected)
			}
		})
	}
}
