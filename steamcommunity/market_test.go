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

func TestGetMyPendingMarketListings(t *testing.T) {
	// Fixture shape mirrors Steam's /market/ page: one pending row
	// (cancel-button href calls CancelMarketListingConfirmation) plus
	// two active rows (RemoveMarketListing). The pending scraper must
	// pick only the pending row despite identical row classes.
	const body = `<html>
<div id="tabContentsMyListings">
  <div class="market_listing_row market_recent_listing_row listing_1000000000001" id="mylisting_1000000000001">
    <div class="market_listing_cancel_button">
      <a href="javascript:CancelMarketListingConfirmation('mylisting', '1000000000001', 440, '2', '2000000001')">Cancel</a>
    </div>
  </div>
</div>
<div id="tabContentsMyActiveMarketListingsTable">
  <div class="market_listing_row market_recent_listing_row listing_1000000000002" id="mylisting_1000000000002">
    <div class="market_listing_cancel_button">
      <a href="javascript:RemoveMarketListing('mylisting', '1000000000002', 440, '2', '2000000002')">Remove</a>
    </div>
  </div>
  <div class="market_listing_row market_recent_listing_row listing_1000000000003" id="mylisting_1000000000003">
    <div class="market_listing_cancel_button">
      <a href="javascript:RemoveMarketListing('mylisting', '1000000000003', 440, '2', '2000000003')">Remove</a>
    </div>
  </div>
</div>
</html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	got, err := c.GetMyPendingMarketListings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d pending listings, want 1: %+v", len(got), got)
	}
	want := PendingListing{ListingID: "1000000000001", AssetID: "2000000001"}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}

func TestGetMyPendingMarketListingsDeduplicates(t *testing.T) {
	// Same pending listing rendered twice (Steam's HTML can duplicate
	// rows on slow re-renders). Scraper must dedupe by listing_id.
	const body = `<html>
<a href="javascript:CancelMarketListingConfirmation('mylisting', '111', 440, '2', '222')">Cancel</a>
<a href="javascript:CancelMarketListingConfirmation('mylisting', '111', 440, '2', '222')">Cancel dup</a>
<a href="javascript:CancelMarketListingConfirmation('mylisting', '333', 440, '2', '444')">Cancel</a>
</html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	got, err := c.GetMyPendingMarketListings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 after dedup: %+v", len(got), got)
	}
	if got[0].ListingID != "111" || got[1].ListingID != "333" {
		t.Errorf("unexpected order/ids: %+v", got)
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
// test fixtures that don't need full JSON marshaling. Escapes CR/LF
// in addition to backslash and quote so multiline HTML fixtures are
// embeddable as JSON string values.
func jsonQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + r.Replace(s) + `"`
}

// sampleListingRow builds one row of results_html matching Steam's
// real markup — nested price spans with title-attribute markers,
// a market_listing_listed_date_combined wrapper for the "Listed: …"
// line, the anchor-based item name link, and the RemoveMarketListing
// cancel-button call that carries (app, context, asset) IDs.
func sampleListingRow(id, name, buyerPrice, sellerPrice, listedDate, assetID string) string {
	return `<div class="market_listing_row market_recent_listing_row listing_` + id + `" id="mylisting_` + id + `">
    <img class="market_listing_item_img" alt="" />
    <div class="market_listing_right_cell market_listing_my_price">
        <span class="market_table_value">
            <span class="market_listing_price">
                <span style="display: inline-block">
                    <span title="This is the price the buyer pays.">
                        ` + buyerPrice + `
                    </span>
                    <br>
                    <span title="This is how much you will receive." style="color: #AFAFAF">
                        (` + sellerPrice + `)
                    </span>
                </span>
            </span>
        </span>
    </div>
    <div class="market_listing_right_cell market_listing_listed_date can_combine">
        ` + listedDate + `
    </div>
    <div class="market_listing_item_name_block">
        <span id="mylisting_` + id + `_name" class="market_listing_item_name">
            <a class="market_listing_item_name_link" href="https://steamcommunity.com/market/listings/440/` + name + `">` + name + `</a>
        </span>
        <br/>
        <span class="market_listing_game_name">Team Fortress 2</span>
        <div class="market_listing_listed_date_combined">
            Listed: ` + listedDate + `
        </div>
    </div>
    <div class="market_listing_cancel_button">
        <a href="javascript:RemoveMarketListing('mylisting', '` + id + `', 440, '2', '` + assetID + `')">Remove</a>
    </div>
</div>`
}

func TestParseMarketListingsHappyPath(t *testing.T) {
	html := strings.Join([]string{
		sampleListingRow("4000000000001", "Secret Saxton", "$0.08 USD", "$0.06 USD", "18 Apr", "999001"),
		sampleListingRow("4000000000002", "Mann Co. Supply Crate Key", "$2.50 USD", "$2.10 USD", "17 Apr", "999002"),
	}, "\n")

	got := parseMarketListings(html)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}

	if got[0].ID != "4000000000001" {
		t.Errorf("row0.ID = %q", got[0].ID)
	}
	if got[0].Asset == nil || got[0].Asset.MarketHashName != "Secret Saxton" {
		t.Errorf("row0.Asset.MarketHashName = %+v", got[0].Asset)
	}
	if got[0].PriceText != "$0.08 USD" {
		t.Errorf("row0.PriceText = %q", got[0].PriceText)
	}
	// The _combined wrapper includes the "Listed:" prefix.
	if got[0].CreatedText != "Listed: 18 Apr" {
		t.Errorf("row0.CreatedText = %q", got[0].CreatedText)
	}

	if got[1].ID != "4000000000002" || got[1].Asset == nil || got[1].Asset.MarketHashName != "Mann Co. Supply Crate Key" {
		t.Errorf("row1 = %+v asset=%+v", got[1], got[1].Asset)
	}
	if got[1].PriceText != "$2.50 USD" {
		t.Errorf("row1.PriceText = %q", got[1].PriceText)
	}
}

func TestFirstInnerText(t *testing.T) {
	cases := []struct {
		name   string
		html   string
		marker string
		want   string
	}{
		{
			name:   "direct child text",
			html:   `<a class="x">Hello</a>`,
			marker: `class="x"`,
			want:   "Hello",
		},
		{
			name:   "nested spans — innermost text wins",
			html:   `<span class="x"><span><span>Deep</span></span></span>`,
			marker: `class="x"`,
			want:   "Deep",
		},
		{
			name:   "leading whitespace and nested tags skipped",
			html:   "<span class=\"x\">\n\t<span>\n\t\tListed\n\t</span>\n</span>",
			marker: `class="x"`,
			want:   "Listed",
		},
		{
			name:   "multi-line text preserved as single line",
			html:   "<span class=\"x\">Listed:\n18 Apr</span>",
			marker: `class="x"`,
			want:   "Listed: 18 Apr",
		},
		{
			name:   "marker not found",
			html:   `<a class="y">Hello</a>`,
			marker: `class="x"`,
			want:   "",
		},
		{
			name:   "empty element",
			html:   `<span class="x"></span>`,
			marker: `class="x"`,
			want:   "",
		},
		{
			name:   "self-closing tag before text doesn't stop accumulation",
			html:   `<span class="x"><br/>Hello</span>`,
			marker: `class="x"`,
			want:   "Hello",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstInnerText(tc.html, tc.marker); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseMarketListingsLocaleIndependent(t *testing.T) {
	// A listing row as Steam would render it for a German-locale
	// account: title attributes in German, "Eingestellt:" prefix
	// on the date, price formatted with comma. Class names and
	// element structure are invariant, so the parser still works.
	row := `<div id="mylisting_8000000000001" class="market_listing_row">
    <div class="market_listing_right_cell market_listing_my_price">
        <span class="market_listing_price">
            <span style="display: inline-block">
                <span title="Dies ist der Preis, den der Käufer bezahlt.">
                    0,08 €
                </span>
                <br>
                <span title="So viel erhältst du." style="color: #AFAFAF">
                    (0,06 €)
                </span>
            </span>
        </span>
    </div>
    <div class="market_listing_item_name_block">
        <a class="market_listing_item_name_link" href="#">Secret Saxton</a>
        <div class="market_listing_listed_date_combined">
            Eingestellt: 18. Apr.
        </div>
    </div>
</div>`
	got := parseMarketListings(row)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].PriceText != "0,08 €" {
		t.Errorf("German PriceText = %q, want 0,08 €", got[0].PriceText)
	}
	if got[0].CreatedText != "Eingestellt: 18. Apr." {
		t.Errorf("German CreatedText = %q", got[0].CreatedText)
	}
}

func TestParseExtractsAssetRefFromCancelButton(t *testing.T) {
	// The RemoveMarketListing call lives AFTER the inner
	// mylisting_<id>_name span. A naive prefix scan would clip
	// the row chunk before the cancel button; the parser's
	// rowSplitRE (digits + literal `"`) avoids that.
	html := sampleListingRow("4000000000001", "Secret Saxton", "$0.08 USD", "$0.06 USD", "18 Apr", "999001")
	got := parseMarketListings(html)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	a := got[0].Asset
	if a == nil {
		t.Fatal("Asset is nil")
	}
	if a.AppID != 440 || a.ContextID != "2" || a.AssetID != "999001" {
		t.Errorf("asset ref = %+v", a)
	}
}

func TestParseMarketListingsDateFallback(t *testing.T) {
	// Older responses sometimes omit the _combined wrapper — the
	// parser should fall back to the short date field. No cancel
	// button here, so Asset legitimately stays nil.
	html := `<div class="market_listing_row" id="mylisting_5000000000001">
    <div class="market_listing_right_cell market_listing_listed_date can_combine">
        19 Apr
    </div>
    <a class="market_listing_item_name_link">Thing</a>
</div>`
	got := parseMarketListings(html)
	if len(got) != 1 || got[0].CreatedText != "19 Apr" {
		t.Errorf("fallback date = %+v", got)
	}
	if got[0].Asset != nil {
		t.Errorf("Asset should be nil without cancel button, got %+v", got[0].Asset)
	}
}

func TestParseMarketListingsEmpty(t *testing.T) {
	if got := parseMarketListings(""); got != nil {
		t.Errorf("empty html returned %v, want nil", got)
	}
	if got := parseMarketListings("<html><body>nothing here</body></html>"); got != nil {
		t.Errorf("no-rows html returned %v, want nil", got)
	}
}

func TestParseMarketListingsDegradesOnMissingFields(t *testing.T) {
	// A row with the outer id but no item-name link or price span.
	// Parser should still return the row with just the ID filled in
	// — a partial match beats dropping the whole entry.
	html := `<div id="mylisting_9000000000001" class="market_listing_row"></div>`
	got := parseMarketListings(html)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0].ID != "9000000000001" {
		t.Errorf("ID = %q", got[0].ID)
	}
	if got[0].PriceText != "" || got[0].CreatedText != "" || got[0].Asset != nil {
		t.Errorf("partial row should leave missing fields empty: %+v", got[0])
	}
}

func TestGetMarketListingsRoundTrip(t *testing.T) {
	var gotStart, gotCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/market/mylistings/render/" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		gotStart = r.URL.Query().Get("start")
		gotCount = r.URL.Query().Get("count")

		resultsHTML := jsonQuote(strings.Join([]string{
			sampleListingRow("4000000000001", "Secret Saxton", "$0.08 USD", "$0.06 USD", "18 Apr", "999001"),
			sampleListingRow("4000000000002", "Mann Co. Supply Crate Key", "$2.50 USD", "$2.10 USD", "17 Apr", "999002"),
		}, ""))
		// Steam's real response shape: `assets` keyed by
		// app_id → context_id → asset_id. No `listinginfo` —
		// that was removed by Steam at some point.
		assets := `"assets":{
			"440":{"2":{
				"999001":{"appid":440,"contextid":"2","id":"999001","classid":"c1","instanceid":"i1","amount":"1","market_hash_name":"Secret Saxton","name":"Secret Saxton","icon_url":"icon1"},
				"999002":{"appid":440,"contextid":"2","id":"999002","classid":"c2","instanceid":"i2","amount":"1","market_hash_name":"Mann Co. Supply Crate Key","name":"Mann Co. Supply Crate Key","icon_url":"icon2"}
			}}
		}`
		body := `{"success":true,"pagesize":2,"total_count":47,"results_html":` + resultsHTML + `,` + assets + `}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	page, err := c.GetMarketListings(context.Background(), 5, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStart != "5" || gotCount != "2" {
		t.Errorf("query params start=%q count=%q, want 5/2", gotStart, gotCount)
	}
	if page.Total != 47 {
		t.Errorf("Total = %d, want 47", page.Total)
	}
	if len(page.Listings) != 2 {
		t.Fatalf("len(Listings) = %d, want 2", len(page.Listings))
	}

	// Row 0 should be enriched with asset metadata joined via
	// the HTML-embedded RemoveMarketListing asset_id.
	row0a := page.Listings[0].Asset
	if row0a == nil {
		t.Fatal("row0.Asset is nil")
	}
	if row0a.AssetID != "999001" || row0a.AppID != 440 || row0a.ContextID != "2" {
		t.Errorf("row0 asset ref = %+v", row0a)
	}
	if row0a.ClassID != "c1" || row0a.InstanceID != "i1" || row0a.IconURL != "icon1" {
		t.Errorf("row0 asset meta = %+v", row0a)
	}
	// Asset's market_hash_name wins over HTML-parsed name.
	if row0a.MarketHashName != "Secret Saxton" {
		t.Errorf("row0.Asset.MarketHashName = %q", row0a.MarketHashName)
	}

	// Row 1 should follow the same pattern.
	row1a := page.Listings[1].Asset
	if row1a == nil || row1a.AssetID != "999002" || row1a.IconURL != "icon2" {
		t.Errorf("row1.Asset = %+v", row1a)
	}
}

func TestGetMarketListingsAssetsMissingLeavesFieldsEmpty(t *testing.T) {
	// When assets sidecar is absent or doesn't contain the
	// referenced ID, the asset fields stay empty but the listing
	// still renders from HTML-parsed data.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resultsHTML := jsonQuote(sampleListingRow("4000000000003", "Thing", "$1.00 USD", "$0.85 USD", "17 Apr", "999003"))
		body := `{"success":true,"pagesize":1,"total_count":1,"results_html":` + resultsHTML + `}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	page, err := c.GetMarketListings(context.Background(), 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Listings) != 1 {
		t.Fatalf("len(Listings) = %d", len(page.Listings))
	}
	a := page.Listings[0].Asset
	if a == nil {
		t.Fatal("Asset should be populated from cancel-button even without assets map")
	}
	// asset_id is still extracted from the HTML cancel call,
	// even when the assets map lookup fails.
	if a.AssetID != "999003" || a.AppID != 440 || a.ContextID != "2" {
		t.Errorf("asset ref = %+v", a)
	}
	// But the per-asset metadata (class, icon, …) stays empty.
	if a.ClassID != "" || a.IconURL != "" || a.InstanceID != "" {
		t.Errorf("expected asset lookup to miss: %+v", a)
	}
}

func TestGetMarketListingsClampsCount(t *testing.T) {
	var gotCount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCount = r.URL.Query().Get("count")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"pagesize":100,"total_count":0,"results_html":""}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	// Requesting 500 gets capped to 100 (Steam's hard ceiling).
	if _, err := c.GetMarketListings(context.Background(), 0, 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCount != "100" {
		t.Errorf("count=%q, want clamped to 100", gotCount)
	}

	// Requesting 0 defaults to 100.
	if _, err := c.GetMarketListings(context.Background(), 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCount != "100" {
		t.Errorf("count=%q with input 0, want default 100", gotCount)
	}
}

func TestGetMarketListingsSurfacesSteamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()

	c := newTestCommunity(t, srv.URL)
	c.httpClient.Transport = rewriteHostTransport(srv)

	_, err := c.GetMarketListings(context.Background(), 0, 10)
	if err == nil {
		t.Fatal("expected error when render endpoint returns success=false")
	}
}
