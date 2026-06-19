package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/k64z/steamstacks/steamid"
)

// EPersonaState mirrors the persona_state values returned by
// ISteamUser/GetPlayerSummaries.
type EPersonaState int

const (
	EPersonaStateOffline        EPersonaState = 0
	EPersonaStateOnline         EPersonaState = 1
	EPersonaStateBusy           EPersonaState = 2
	EPersonaStateAway           EPersonaState = 3
	EPersonaStateSnooze         EPersonaState = 4
	EPersonaStateLookingToTrade EPersonaState = 5
	EPersonaStateLookingToPlay  EPersonaState = 6
)

// PlayerSummary is the subset of ISteamUser/GetPlayerSummaries fields
// that callers in this repo actually need. Steam returns more fields
// (real name, primary clan, location, etc.) which can be added as needed.
type PlayerSummary struct {
	SteamID                  steamid.SteamID
	PersonaName              string
	ProfileURL               string
	AvatarSmall              string
	AvatarMedium             string
	AvatarLarge              string
	AvatarHash               string
	PersonaState             EPersonaState
	CommunityVisibilityState int
	ProfileState             int
	LastLogoff               int64
	TimeCreated              int64
	GameID                   string
	GameExtraInfo            string
	GameServerIP             string
}

const playerSummariesBatchSize = 100

// GetPlayerSummaries fetches public profile snapshots for the given SteamIDs
// from ISteamUser/GetPlayerSummaries/v2. The endpoint accepts at most 100 IDs
// per call; this helper transparently batches and concatenates results.
//
// Authentication uses the API's configured key or access token via
// getAuthParams. Missing players are silently omitted from the response —
// callers wanting per-ID error reporting should diff input vs. output.
func (a *API) GetPlayerSummaries(ctx context.Context, ids []steamid.SteamID) ([]PlayerSummary, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	out := make([]PlayerSummary, 0, len(ids))
	for start := 0; start < len(ids); start += playerSummariesBatchSize {
		end := start + playerSummariesBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		page, err := a.getPlayerSummariesBatch(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
	}
	return out, nil
}

func (a *API) getPlayerSummariesBatch(ctx context.Context, ids []steamid.SteamID) ([]PlayerSummary, error) {
	params, err := a.getAuthParams()
	if err != nil {
		return nil, err
	}

	csv := make([]string, len(ids))
	for i, id := range ids {
		csv[i] = strconv.FormatUint(id.ToSteamID64(), 10)
	}
	params.Set("steamids", strings.Join(csv, ","))

	reqURL := a.baseURL + "/ISteamUser/GetPlayerSummaries/v2/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, HTTPStatusErrorFromResponse(resp)
	}

	var decoded struct {
		Response struct {
			Players []rawPlayerSummary `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]PlayerSummary, 0, len(decoded.Response.Players))
	for _, p := range decoded.Response.Players {
		sid, err := steamid.FromString(p.SteamID)
		if err != nil {
			return nil, fmt.Errorf("parse SteamID %q: %w", p.SteamID, err)
		}
		out = append(out, PlayerSummary{
			SteamID:                  sid,
			PersonaName:              p.PersonaName,
			ProfileURL:               p.ProfileURL,
			AvatarSmall:              p.Avatar,
			AvatarMedium:             p.AvatarMedium,
			AvatarLarge:              p.AvatarFull,
			AvatarHash:               p.AvatarHash,
			PersonaState:             EPersonaState(p.PersonaState),
			CommunityVisibilityState: p.CommunityVisibilityState,
			ProfileState:             p.ProfileState,
			LastLogoff:               p.LastLogoff,
			TimeCreated:              p.TimeCreated,
			GameID:                   p.GameID,
			GameExtraInfo:            p.GameExtraInfo,
			GameServerIP:             p.GameServerIP,
		})
	}
	return out, nil
}

type rawPlayerSummary struct {
	SteamID                  string `json:"steamid"`
	CommunityVisibilityState int    `json:"communityvisibilitystate"`
	ProfileState             int    `json:"profilestate"`
	PersonaName              string `json:"personaname"`
	ProfileURL               string `json:"profileurl"`
	Avatar                   string `json:"avatar"`
	AvatarMedium             string `json:"avatarmedium"`
	AvatarFull               string `json:"avatarfull"`
	AvatarHash               string `json:"avatarhash"`
	LastLogoff               int64  `json:"lastlogoff"`
	PersonaState             int    `json:"personastate"`
	TimeCreated              int64  `json:"timecreated"`
	GameID                   string `json:"gameid"`
	GameExtraInfo            string `json:"gameextrainfo"`
	GameServerIP             string `json:"gameserverip"`
}
