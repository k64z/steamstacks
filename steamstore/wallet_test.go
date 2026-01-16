package steamstore

import (
	"os"
	"testing"
)

func TestParseWalletBalance(t *testing.T) {
	data, err := os.ReadFile("testdata/account.html")
	if err != nil {
		t.Skipf("testdata/account.html not found: %v", err)
	}

	balance, err := parseWalletBalance(string(data))
	if err != nil {
		t.Fatalf("failed to parse wallet balance: %v", err)
	}

	if !balance.HasWallet {
		t.Error("expected HasWallet to be true")
	}

	if balance.Balance == "" {
		t.Error("expected Balance to be non-empty")
	}

	t.Logf("Parsed wallet balance: %s", balance.Balance)
}

func TestParseWalletBalanceNoWallet(t *testing.T) {
	html := `<html><body>No wallet here</body></html>`

	balance, err := parseWalletBalance(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if balance.HasWallet {
		t.Error("expected HasWallet to be false")
	}

	if balance.Balance != "" {
		t.Errorf("expected Balance to be empty, got %q", balance.Balance)
	}
}

func TestParseWalletBalanceVariousFormats(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
		hasWallet bool
	}{
		{
			name:     "USD format",
			html:     `<a class="global_action_link" id="header_wallet_balance" href="/account/store_transactions/">$10.00</a>`,
			expected: "$10.00",
			hasWallet: true,
		},
		{
			name:     "EUR format",
			html:     `<a class="global_action_link" id="header_wallet_balance" href="/account/store_transactions/">10,00€</a>`,
			expected: "10,00€",
			hasWallet: true,
		},
		{
			name:     "KZT format with spaces",
			html:     `<a class="global_action_link" id="header_wallet_balance" href="https://store.steampowered.com/account/store_transactions/">48 291,94₸</a>`,
			expected: "48 291,94₸",
			hasWallet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, err := parseWalletBalance(tt.html)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if balance.HasWallet != tt.hasWallet {
				t.Errorf("expected HasWallet=%v, got %v", tt.hasWallet, balance.HasWallet)
			}

			if balance.Balance != tt.expected {
				t.Errorf("expected Balance=%q, got %q", tt.expected, balance.Balance)
			}
		})
	}
}
