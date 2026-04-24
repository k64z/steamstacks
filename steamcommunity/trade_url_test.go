package steamcommunity

import (
	"errors"
	"testing"
)

func TestParseTradeOfferURLRealSteamSnippet(t *testing.T) {
	html := `
<div class="trade_offer_access_url_ctn">
	<input size="45" type="text" class="trade_offer_access_url" id="trade_offer_access_url" value="https://steamcommunity.com/tradeoffer/new/?partner=1265728223&amp;token=wjf4zUqn" readonly="">
</div>
`
	got, err := parseTradeOfferURL(html)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "https://steamcommunity.com/tradeoffer/new/?partner=1265728223&token=wjf4zUqn"
	if got != want {
		t.Errorf("url = %q; want %q", got, want)
	}
}

func TestParseTradeOfferURLClassOnly(t *testing.T) {
	html := `<input type="text" class="trade_offer_access_url" value="https://steamcommunity.com/tradeoffer/new/?partner=1&amp;token=abc" readonly>`
	got, err := parseTradeOfferURL(html)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := "https://steamcommunity.com/tradeoffer/new/?partner=1&token=abc"; got != want {
		t.Errorf("url = %q; want %q", got, want)
	}
}

func TestParseTradeOfferURLValueBeforeID(t *testing.T) {
	html := `<input value="https://steamcommunity.com/tradeoffer/new/?partner=9&amp;token=xy" id="trade_offer_access_url">`
	got, err := parseTradeOfferURL(html)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := "https://steamcommunity.com/tradeoffer/new/?partner=9&token=xy"; got != want {
		t.Errorf("url = %q; want %q", got, want)
	}
}

func TestParseTradeOfferURLSingleQuotes(t *testing.T) {
	html := `<input type='text' id='trade_offer_access_url' value='https://steamcommunity.com/tradeoffer/new/?partner=5&amp;token=qq' readonly>`
	got, err := parseTradeOfferURL(html)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := "https://steamcommunity.com/tradeoffer/new/?partner=5&token=qq"; got != want {
		t.Errorf("url = %q; want %q", got, want)
	}
}

func TestParseTradeOfferURLMissing(t *testing.T) {
	html := `<html><body><p>You don't have a trade URL yet.</p></body></html>`
	_, err := parseTradeOfferURL(html)
	if !errors.Is(err, ErrNoTradeToken) {
		t.Errorf("want ErrNoTradeToken, got %v", err)
	}
}

func TestParseTradeOfferURLEmptyValue(t *testing.T) {
	html := `<input id="trade_offer_access_url" value="" readonly>`
	_, err := parseTradeOfferURL(html)
	if !errors.Is(err, ErrNoTradeToken) {
		t.Errorf("want ErrNoTradeToken for empty value, got %v", err)
	}
}
