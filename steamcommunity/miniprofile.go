package steamcommunity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamid"
)

// MiniProfile is the JSON shape served by steamcommunity.com/miniprofile/<accountid>/json.
// It is the cheapest cookie-authenticated source of persona + avatar +
// in-game data — useful where the Web API alternative (GetPlayerSummaries)
// is unavailable because the bot has no Steam Web API key.
type MiniProfile struct {
	SteamID     steamid.SteamID
	PersonaName string
	AvatarURL   string
	Level       int
	InGame      *MiniProfileGame
}

// MiniProfileGame is non-nil when the player is currently in a game.
// IsNonSteam is true for users in a non-Steam game; Steam reports such
// users with PersonaState=Offline in CM packets but they're actually
// online (visible on the profile as "In non-Steam game"). Name is empty
// for non-Steam games unless the user customised it.
type MiniProfileGame struct {
	Name         string
	RichPresence string
	LogoURL      string
	IsNonSteam   bool
}

type rawMiniProfile struct {
	PersonaName string `json:"persona_name"`
	AvatarURL   string `json:"avatar_url"`
	Level       int    `json:"level"`
	InGame      *struct {
		Name         string `json:"name"`
		RichPresence string `json:"rich_presence"`
		LogoURL      string `json:"logo"`
		IsNonSteam   bool   `json:"is_non_steam"`
	} `json:"in_game"`
}

// GetMiniProfile fetches the mini-profile blob for a single SteamID.
// Authentication comes from the cookie jar; no Web API key required.
func (c *Community) GetMiniProfile(ctx context.Context, sid steamid.SteamID) (MiniProfile, error) {
	accountID := sid.AccountID()
	endpoint := "https://steamcommunity.com/miniprofile/" + strconv.FormatUint(uint64(accountID), 10) + "/json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return MiniProfile{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MiniProfile{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MiniProfile{}, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return MiniProfile{}, steamapi.HTTPStatusError(resp.StatusCode, body)
	}

	var raw rawMiniProfile
	if err := json.Unmarshal(body, &raw); err != nil {
		return MiniProfile{}, fmt.Errorf("decode mini-profile: %w", err)
	}

	out := MiniProfile{
		SteamID:     sid,
		PersonaName: raw.PersonaName,
		AvatarURL:   raw.AvatarURL,
		Level:       raw.Level,
	}
	if raw.InGame != nil {
		out.InGame = &MiniProfileGame{
			Name:         raw.InGame.Name,
			RichPresence: raw.InGame.RichPresence,
			LogoURL:      raw.InGame.LogoURL,
			IsNonSteam:   raw.InGame.IsNonSteam,
		}
	}
	return out, nil
}

// GetMiniProfiles batches mini-profile lookups across the given SteamIDs
// with a small worker pool. Per-ID failures are dropped (caller diffs
// input vs. output to detect them) so a single private or deleted
// profile doesn't poison the whole batch. If ctx is cancelled before
// every ID is scheduled, the batch is abandoned and ctx.Err() is
// returned — rather than a silently truncated result padded with
// zero-value profiles.
func (c *Community) GetMiniProfiles(ctx context.Context, ids []steamid.SteamID) ([]MiniProfile, error) {
	const concurrency = 12
	if len(ids) == 0 {
		return nil, nil
	}

	type result struct {
		profile MiniProfile
		err     error
	}
	results := make([]result, len(ids))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	cancelled := false
loop:
	for i, id := range ids {
		select {
		case <-ctx.Done():
			// Caller cancelled — stop scheduling. In-flight goroutines
			// already past sem-acquire will exit via their own ctx
			// check inside GetMiniProfile.
			cancelled = true
			break loop
		case sem <- struct{}{}:
		}
		i, id := i, id
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := c.GetMiniProfile(ctx, id)
			results[i] = result{profile: p, err: err}
		}()
	}
	wg.Wait()

	// A cancelled scheduling loop leaves a zero-valued tail in results:
	// those entries have err==nil and would be appended as phantom
	// SteamID-0 profiles. Report the cancellation instead of returning a
	// partial batch the caller can't distinguish from a complete one.
	if cancelled {
		return nil, ctx.Err()
	}

	out := make([]MiniProfile, 0, len(ids))
	for _, r := range results {
		if r.err != nil {
			continue
		}
		out = append(out, r.profile)
	}
	return out, nil
}
