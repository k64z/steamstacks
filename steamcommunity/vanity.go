package steamcommunity

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
)

// ErrVanityNotFound is returned by ResolveVanityURL when Steam responds
// without a steamID64 — typically because the vanity name doesn't exist or
// the profile is configured to be unreachable by vanity lookup.
var ErrVanityNotFound = errors.New("steamcommunity: vanity URL not found")

// ResolveVanityURL maps a profile vanity name (the slug used in
// steamcommunity.com/id/<vanity>) to its SteamID64.
//
// Uses the /id/<vanity>/?xml=1 page which works with cookie auth — no Web
// API key required. Returns ErrVanityNotFound if Steam responds without a
// <steamID64> element.
func (c *Community) ResolveVanityURL(ctx context.Context, vanity string) (steamid.SteamID, error) {
	vanity = strings.TrimSpace(vanity)
	if vanity == "" {
		return 0, errors.New("steamcommunity: empty vanity name")
	}

	endpoint := "https://steamcommunity.com/id/" + url.PathEscape(vanity) + "/?xml=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, steamapi.HTTPStatusError(resp.StatusCode, body)
	}

	var parsed struct {
		XMLName   xml.Name `xml:"profile"`
		SteamID64 string   `xml:"steamID64"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return 0, ErrVanityNotFound
	}
	if parsed.SteamID64 == "" {
		return 0, ErrVanityNotFound
	}

	sid, err := steamid.FromString(parsed.SteamID64)
	if err != nil {
		return 0, fmt.Errorf("parse steamID64 %q: %w", parsed.SteamID64, err)
	}
	return sid, nil
}

var (
	reProfileURL = regexp.MustCompile(`(?i)steamcommunity\.com/profiles/(\d{17})`)
	reVanityURL  = regexp.MustCompile(`(?i)steamcommunity\.com/id/([A-Za-z0-9_\-]+)`)
	reSteamID64  = regexp.MustCompile(`^7656\d{13}$`)
)

// ParseProfileInput classifies a user-entered profile reference. Exactly one
// of (sid, vanity) is set on a successful return:
//
//   - A 17-digit SteamID64           → sid=<parsed>, vanity=""
//   - steamcommunity.com/profiles/X  → sid=<parsed>, vanity=""
//   - steamcommunity.com/id/<name>   → sid=0,        vanity="<name>"
//   - a bare slug ([A-Za-z0-9_-]+)   → sid=0,        vanity="<input>"
//
// Callers should resolve a non-empty vanity via ResolveVanityURL.
func ParseProfileInput(s string) (sid steamid.SteamID, vanity string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", errors.New("steamcommunity: empty input")
	}

	if reSteamID64.MatchString(s) {
		parsed, perr := steamid.FromString(s)
		if perr != nil {
			return 0, "", fmt.Errorf("parse SteamID64: %w", perr)
		}
		return parsed, "", nil
	}

	if m := reProfileURL.FindStringSubmatch(s); len(m) == 2 {
		parsed, perr := steamid.FromString(m[1])
		if perr != nil {
			return 0, "", fmt.Errorf("parse SteamID64 from URL: %w", perr)
		}
		return parsed, "", nil
	}

	if m := reVanityURL.FindStringSubmatch(s); len(m) == 2 {
		return 0, m[1], nil
	}

	// Bare slug fallback — only accept characters Steam actually permits in vanity URLs.
	if regexp.MustCompile(`^[A-Za-z0-9_\-]+$`).MatchString(s) {
		return 0, s, nil
	}

	return 0, "", fmt.Errorf("steamcommunity: unrecognized profile input %q", s)
}
