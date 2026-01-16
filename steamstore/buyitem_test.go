package steamstore

import (
	"os"
	"testing"
)

func TestParseBuyItemForm(t *testing.T) {
	html, err := os.ReadFile("testdata/buyitem_checkout.html")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	txnData, err := parseBuyItemForm(string(html))
	if err != nil {
		t.Fatalf("parseBuyItemForm: %v", err)
	}

	if txnData.transactionID != "123456789012345678" {
		t.Errorf("transactionID = %q, want %q", txnData.transactionID, "123456789012345678")
	}

	if txnData.sessionID != "testsessionid123" {
		t.Errorf("sessionID = %q, want %q", txnData.sessionID, "testsessionid123")
	}

	// returnURL should have &amp; decoded to &
	wantReturnURL := "https://store.steampowered.com/buyitem/440/finalize/987654321?canceledurl=https%3A%2F%2Fstore.steampowered.com%2F&returnhost=store.steampowered.com"
	if txnData.returnURL != wantReturnURL {
		t.Errorf("returnURL = %q, want %q", txnData.returnURL, wantReturnURL)
	}
}

func TestExtractOrderID(t *testing.T) {
	tests := []struct {
		returnURL string
		wantOrder string
	}{
		{
			returnURL: "https://store.steampowered.com/buyitem/440/finalize/987654321?canceledurl=https%3A%2F%2Fstore.steampowered.com%2F",
			wantOrder: "987654321",
		},
		{
			returnURL: "https://store.steampowered.com/buyitem/730/finalize/123456789",
			wantOrder: "123456789",
		},
		{
			returnURL: "https://store.steampowered.com/other/path",
			wantOrder: "",
		},
	}

	for _, tt := range tests {
		orderID := extractMatch(reOrderID, tt.returnURL)
		if orderID != tt.wantOrder {
			t.Errorf("extractMatch(reOrderID, %q) = %q, want %q", tt.returnURL, orderID, tt.wantOrder)
		}
	}
}

func TestParseBuyItemFormMissingFields(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{
			name: "missing transaction_id",
			html: `<input name="returnurl" value="url" /><input name="sessionid" value="sid" />`,
		},
		{
			name: "missing returnurl",
			html: `<input name="transaction_id" value="123" /><input name="sessionid" value="sid" />`,
		},
		{
			name: "missing sessionid",
			html: `<input name="transaction_id" value="123" /><input name="returnurl" value="url" />`,
		},
	}

	for _, tt := range tests {
		_, err := parseBuyItemForm(tt.html)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}
