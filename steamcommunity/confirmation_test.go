package steamcommunity

import "testing"

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
