package steamcommunity

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
)

// ErrNoTradeToken is returned from GetTradeOfferURL when the account's
// /tradeoffers/privacy page has no active trade offer access URL.
// The caller usually wants to surface this as "visit Steam and click
// Create New URL" rather than retrying automatically.
var ErrNoTradeToken = errors.New("steamcommunity: no trade offer access token (create one via Steam first)")

// GetTradeOfferURL returns the logged-in account's own Steam trade
// offer URL — the one you'd paste into a third-party trading site to
// let them send offers to you. Fetches
// https://steamcommunity.com/profiles/{steamid64}/tradeoffers/privacy
// and parses the full URL from the trade_offer_access_url input.
//
// Steam exposes no JSON endpoint for this; the only canonical source
// is the HTML form on the privacy page. Caching the result is the
// caller's job — this method hits Steam every call.
func (c *Community) GetTradeOfferURL(ctx context.Context) (string, error) {
	if err := c.ensureInit(); err != nil {
		return "", err
	}
	u := fmt.Sprintf("https://steamcommunity.com/profiles/%d/tradeoffers/privacy", c.SteamID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return parseTradeOfferURL(string(body))
}

// Steam renders the URL inside a readonly text input marked by either
// id="trade_offer_access_url" or class="trade_offer_access_url" — we
// accept both. Attribute order within the <input> varies across
// templates, so the two-step approach (find the tag, then pull value=)
// avoids combinatorial alternation.
var (
	reTradeURLInput = regexp.MustCompile(`<input\b[^>]*\b(?:id|class)\s*=\s*["'][^"']*\btrade_offer_access_url\b[^"']*["'][^>]*>`)
	reValueAttr     = regexp.MustCompile(`\bvalue\s*=\s*["']([^"']*)["']`)
)

// parseTradeOfferURL extracts the full Steam trade offer URL from a
// /tradeoffers/privacy HTML response. Returns ErrNoTradeToken when the
// input is absent — Steam renders the privacy page without the URL
// input when no token has been created yet.
func parseTradeOfferURL(body string) (string, error) {
	tag := reTradeURLInput.FindString(body)
	if tag == "" {
		return "", ErrNoTradeToken
	}
	m := reValueAttr.FindStringSubmatch(tag)
	if len(m) != 2 || m[1] == "" {
		return "", ErrNoTradeToken
	}
	return html.UnescapeString(m[1]), nil
}
