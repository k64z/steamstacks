package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// AssetClassKey identifies one (classid, instanceid) pair for a
// GetAssetClassInfo lookup. InstanceID may be empty; the API
// interprets that as "any instance" — Steam returns a single
// description that's valid for the class regardless of instance
// state.
type AssetClassKey struct {
	ClassID    string
	InstanceID string
}

// GetAssetClassInfo resolves descriptions for a list of
// (classid, instanceid) pairs via ISteamEconomy/GetAssetClassInfo/v1.
// It is the right primitive for backfilling descriptions Steam
// omitted from a paginated GetTradeOffers response — the trade-offer
// dashboard uses it lazily when the operator opens an offer whose
// items have unresolved descriptions.
//
// The returned map is keyed by AssetDescriptionKey(appID, classID,
// instanceID) so callers can look up by the same key shape used for
// trade-offer descriptions. Missing entries (Steam doesn't have a
// description for that class) are simply absent from the map; the
// caller should treat absence as "Steam can't resolve this."
//
// Steam caps the number of classes per request at 100. This wrapper
// transparently chunks larger inputs into multiple calls.
func (a *API) GetAssetClassInfo(ctx context.Context, appID int, keys []AssetClassKey) (map[string]AssetDescription, error) {
	out := make(map[string]AssetDescription, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	const chunkSize = 100
	for i := 0; i < len(keys); i += chunkSize {
		end := i + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := a.getAssetClassInfoChunk(ctx, appID, keys[i:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (a *API) getAssetClassInfoChunk(ctx context.Context, appID int, keys []AssetClassKey, out map[string]AssetDescription) error {
	params, err := a.getAuthParams()
	if err != nil {
		return err
	}
	params.Set("appid", strconv.Itoa(appID))
	params.Set("class_count", strconv.Itoa(len(keys)))
	params.Set("language", "en")
	for i, k := range keys {
		params.Set("classid"+strconv.Itoa(i), k.ClassID)
		if k.InstanceID != "" {
			params.Set("instanceid"+strconv.Itoa(i), k.InstanceID)
		}
	}

	reqURL := fmt.Sprintf("%s/ISteamEconomy/GetAssetClassInfo/v1/?%s", a.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkEconResponse(resp); err != nil {
		return err
	}

	// The response wraps a heterogeneous map: most entries are
	// per-class description objects keyed by `classid` or
	// `classid_instanceid`, but a `success` sentinel and occasional
	// error strings share the object. Decode as raw messages and
	// skip non-object entries.
	var envelope struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	keyHasInstance := make(map[string]string, len(keys))
	for _, k := range keys {
		keyHasInstance[k.ClassID] = k.InstanceID
		if k.InstanceID != "" {
			keyHasInstance[k.ClassID+"_"+k.InstanceID] = k.InstanceID
		}
	}

	for k, raw := range envelope.Result {
		if k == "success" || k == "error" {
			continue
		}
		var info assetClassInfoWire
		if err := json.Unmarshal(raw, &info); err != nil {
			continue
		}
		// Fall back to parsing the outer key if Steam omits classid/
		// instanceid from the inner object — older variants of this
		// endpoint shape inline only the description fields and rely
		// on the response key for identity.
		if info.ClassID == "" {
			before, after, _ := strings.Cut(k, "_")
			info.ClassID = before
			info.InstanceID = after
		}
		if info.InstanceID == "" {
			if inst, ok := keyHasInstance[k]; ok {
				info.InstanceID = inst
			}
		}
		if info.ClassID == "" {
			continue
		}
		desc := info.toAssetDescription(appID)
		out[AssetDescriptionKey(appID, desc.ClassID, desc.InstanceID)] = desc
	}
	return nil
}

// assetClassInfoWire mirrors the response shape for one description
// entry inside ISteamEconomy/GetAssetClassInfo/v1. Steam is
// inconsistent across appids about whether tradable/marketable/
// commodity arrive as ints, strings, or bools, so those fields use
// RawMessage and a tolerant decoder.
type assetClassInfoWire struct {
	AppID           int             `json:"-"`
	ClassID         string          `json:"classid"`
	InstanceID      string          `json:"instanceid"`
	Name            string          `json:"name"`
	MarketHashName  string          `json:"market_hash_name"`
	MarketName      string          `json:"market_name"`
	NameColor       string          `json:"name_color"`
	BackgroundColor string          `json:"background_color"`
	Type            string          `json:"type"`
	IconURL         string          `json:"icon_url"`
	IconURLLarge    string          `json:"icon_url_large"`
	Tradable        json.RawMessage `json:"tradable"`
	Marketable      json.RawMessage `json:"marketable"`
	Commodity       json.RawMessage `json:"commodity"`
	Tags            json.RawMessage `json:"tags,omitempty"`
	Descriptions    json.RawMessage `json:"descriptions,omitempty"`
	Actions         json.RawMessage `json:"actions,omitempty"`
	// Steam returns either an array `[...]` or an empty string `""`
	// for fraudwarnings; RawMessage absorbs both so unmarshal won't
	// fail on the empty-string case.
	FraudWarnings json.RawMessage `json:"fraudwarnings,omitempty"`
}

func (w assetClassInfoWire) toAssetDescription(appID int) AssetDescription {
	d := AssetDescription{
		AppID:          appID,
		ClassID:        w.ClassID,
		InstanceID:     w.InstanceID,
		Name:           w.Name,
		MarketHashName: w.MarketHashName,
		Type:           w.Type,
		IconURL:        w.IconURL,
		IconURLLarge:   w.IconURLLarge,
		Tradable:       flagToBool(w.Tradable),
		Marketable:     flagToBool(w.Marketable),
		Commodity:      flagToBool(w.Commodity),
	}
	// tags/descriptions/actions arrive as objects keyed by string
	// index ("0", "1", ...), not arrays — convert to slices that
	// match AssetDescription's array shape so callers see the same
	// data regardless of which endpoint returned it.
	d.Tags = decodeIndexedTags(w.Tags)
	d.Descriptions = decodeIndexedDescriptions(w.Descriptions)
	d.Actions = decodeIndexedActions(w.Actions)
	d.FraudWarnings = decodeIndexedStrings(w.FraudWarnings)
	return d
}

// decodeIndexed* convert Steam's "indexed-object" shape from
// GetAssetClassInfo into the array shape used elsewhere. Each helper
// accepts the array form too — callers that hit other endpoints
// where the field is already an array continue to work.

func decodeIndexedTags(raw json.RawMessage) []Tag {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var arr []Tag
		_ = json.Unmarshal(raw, &arr)
		return arr
	}
	if raw[0] == '{' {
		var m map[string]Tag
		if err := json.Unmarshal(raw, &m); err == nil {
			return mapByIndexKey(m)
		}
	}
	return nil
}

func decodeIndexedDescriptions(raw json.RawMessage) []DescriptionLine {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var arr []DescriptionLine
		_ = json.Unmarshal(raw, &arr)
		return arr
	}
	if raw[0] == '{' {
		var m map[string]DescriptionLine
		if err := json.Unmarshal(raw, &m); err == nil {
			return mapByIndexKey(m)
		}
	}
	return nil
}

func decodeIndexedActions(raw json.RawMessage) []Action {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var arr []Action
		_ = json.Unmarshal(raw, &arr)
		return arr
	}
	if raw[0] == '{' {
		var m map[string]Action
		if err := json.Unmarshal(raw, &m); err == nil {
			return mapByIndexKey(m)
		}
	}
	return nil
}

func decodeIndexedStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var arr []string
		_ = json.Unmarshal(raw, &arr)
		return arr
	}
	if raw[0] == '{' {
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err == nil {
			return mapByIndexKey(m)
		}
	}
	// "" or other non-array, non-object — empty list.
	return nil
}

// mapByIndexKey converts a map keyed by stringified indices ("0",
// "1", ...) into a slice in numeric-key order. Used by the indexed-
// object decoders. Generic so it works for Tag, DescriptionLine,
// Action, and string maps without per-type duplication.
func mapByIndexKey[V any](m map[string]V) []V {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int, 0, len(m))
	idx := make(map[int]string, len(m))
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		keys = append(keys, n)
		idx[n] = k
	}
	sort.Ints(keys)
	out := make([]V, 0, len(keys))
	for _, n := range keys {
		out = append(out, m[idx[n]])
	}
	return out
}

// flagToBool decodes a tradable/marketable/commodity flag whose wire
// shape varies across endpoints — Steam returns it as int (0/1), int
// string ("0"/"1"), or bool depending on the call. Empty/missing
// values map to false.
func flagToBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	switch raw[0] {
	case 't':
		return true
	case 'f':
		return false
	case '"':
		// Quoted: trim quotes and inspect the contents.
		s := strings.Trim(string(raw), `"`)
		return s != "" && s != "0" && s != "false"
	default:
		// Bare number.
		s := string(raw)
		return s != "" && s != "0"
	}
}

