package steamcommunity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetMarketPriceOverview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/market/priceoverview/" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("appid"); got != "440" {
			t.Errorf("appid=%q, want 440", got)
		}
		if got := r.URL.Query().Get("currency"); got != "1" {
			t.Errorf("currency=%q, want 1", got)
		}
		if got := r.URL.Query().Get("market_hash_name"); got != "Earbuds" {
			t.Errorf("market_hash_name=%q, want Earbuds", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"lowest_price":"$1.50","median_price":"$1.75","volume":"42"}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	o, err := c.GetMarketPriceOverview(context.Background(), 440, 1, "Earbuds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !o.Success || o.MedianPrice != "$1.75" || o.LowestPrice != "$1.50" || o.Volume != "42" {
		t.Errorf("unexpected overview: %+v", o)
	}
}

func TestSellMarketItemSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/market/sellitem/" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = r.ParseForm()
		if got := r.PostFormValue("sessionid"); got != "test-session-id" {
			t.Errorf("sessionid=%q, want test-session-id", got)
		}
		if got := r.PostFormValue("appid"); got != "440" {
			t.Errorf("appid=%q, want 440", got)
		}
		if got := r.PostFormValue("price"); got != "87" {
			t.Errorf("price=%q, want 87", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"requires_confirmation":1,"needs_mobile_confirmation":true}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	res, err := c.SellMarketItem(context.Background(), 440, 2, 1234, 1, 87)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !res.MobileConfirmationRequired {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestSellMarketItemTypedErrors(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    error
	}{
		{"item server down", "The game's item server may be down", ErrMarketItemServerDown},
		{"pending", "You already have a listing for this item pending confirmation", ErrMarketPendingConfirmation},
		{"not in inventory", "The item specified is no longer in your inventory", ErrMarketItemNotInInventory},
		{"wallet full", "You must spend some Steam Wallet funds or remove some", ErrMarketWalletTooMuchMoney},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.message
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":false,"message":` + jsonQuote(msg) + `}`))
			}))
			defer srv.Close()

			c := newTestCommunity(t, srv.URL)
			c.httpClient.Transport = rewriteHostTransport(srv)

			_, err := c.SellMarketItem(context.Background(), 440, 2, 1, 1, 50)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestGetMyMarketListingIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>
<div class="market_recent_listing_row listing_1000000000001"><span>item A</span></div>
<div class="market_recent_listing_row listing_1000000000002"><span>item B</span></div>
<div class="market_recent_listing_row listing_1000000000001"><span>dup</span></div>
</html>`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	ids, err := c.GetMyMarketListingIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "1000000000001" || ids[1] != "1000000000002" {
		t.Errorf("unexpected ids: %v", ids)
	}
}

func TestCancelMarketListing(t *testing.T) {
	var gotPath, gotSession string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = r.ParseForm()
		gotSession = r.PostFormValue("sessionid")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	if err := c.CancelMarketListing(context.Background(), "1000000000001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/market/removelisting/1000000000001") {
		t.Errorf("path=%q, want suffix /market/removelisting/1000000000001", gotPath)
	}
	if gotSession != "test-session-id" {
		t.Errorf("sessionid=%q, want test-session-id", gotSession)
	}
}

// jsonQuote produces a JSON-quoted string literal — enough for simple
// test fixtures that don't need full JSON marshaling.
func jsonQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
