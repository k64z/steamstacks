package steamapi

import (
	"encoding/json"
	"testing"
)

func TestAssetDescriptionUnmarshal(t *testing.T) {
	// Simulates the descriptions array from GetTradeOffers with Steam's
	// current wire format (bool for tradable/marketable/commodity).
	const payload = `[
		{
			"appid": 440,
			"classid": "101",
			"instanceid": "0",
			"name": "The Sydney Sleeper",
			"market_hash_name": "The Sydney Sleeper",
			"type": "Level 1 Sniper Rifle",
			"tradable": true,
			"marketable": false,
			"commodity": false,
			"icon_url": "icon_101",
			"icon_url_large": "icon_101_large",
			"descriptions": [
				{"value": "+25% charge rate", "color": "7ea9d1"}
			],
			"tags": [
				{"category": "Quality", "internal_name": "Unique", "localized_category_name": "Quality", "localized_tag_name": "Unique", "color": "7D6D00"}
			],
			"actions": [
				{"link": "http://example.com", "name": "Wiki Page"}
			],
			"fraudwarnings": ["renamed"]
		},
		{
			"appid": 730,
			"classid": "200",
			"instanceid": "55",
			"name": "AK-47 | Redline",
			"type": "Classified Rifle",
			"tradable": true,
			"marketable": true,
			"commodity": true,
			"icon_url": ""
		}
	]`

	var descs []AssetDescription
	if err := json.Unmarshal([]byte(payload), &descs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Build map the same way production code does.
	m := make(map[string]AssetDescription, len(descs))
	for _, d := range descs {
		m[AssetDescriptionKey(d.AppID, d.ClassID, d.InstanceID)] = d
	}

	if got, want := len(m), 2; got != want {
		t.Fatalf("len(m) = %d; want %d", got, want)
	}

	// First item: tradable, not marketable, not commodity
	d1 := m["440_101_0"]
	if d1.Name != "The Sydney Sleeper" {
		t.Errorf("d1.Name = %q; want %q", d1.Name, "The Sydney Sleeper")
	}
	if d1.MarketHashName != "The Sydney Sleeper" {
		t.Errorf("d1.MarketHashName = %q; want %q", d1.MarketHashName, "The Sydney Sleeper")
	}
	if d1.Type != "Level 1 Sniper Rifle" {
		t.Errorf("d1.Type = %q; want %q", d1.Type, "Level 1 Sniper Rifle")
	}
	if !d1.Tradable {
		t.Error("d1.Tradable = false; want true")
	}
	if d1.Marketable {
		t.Error("d1.Marketable = true; want false")
	}
	if d1.Commodity {
		t.Error("d1.Commodity = true; want false")
	}
	if d1.IconURL != "icon_101" {
		t.Errorf("d1.IconURL = %q; want %q", d1.IconURL, "icon_101")
	}
	if got, want := len(d1.Descriptions), 1; got != want {
		t.Errorf("len(d1.Descriptions) = %d; want %d", got, want)
	}
	if got, want := len(d1.Tags), 1; got != want {
		t.Errorf("len(d1.Tags) = %d; want %d", got, want)
	}
	if got, want := len(d1.Actions), 1; got != want {
		t.Errorf("len(d1.Actions) = %d; want %d", got, want)
	}
	if got, want := len(d1.FraudWarnings), 1; got != want {
		t.Errorf("len(d1.FraudWarnings) = %d; want %d", got, want)
	}

	// Second item: different app, all bools true
	d2 := m["730_200_55"]
	if d2.Name != "AK-47 | Redline" {
		t.Errorf("d2.Name = %q; want %q", d2.Name, "AK-47 | Redline")
	}
	if !d2.Tradable {
		t.Error("d2.Tradable = false; want true")
	}
	if !d2.Marketable {
		t.Error("d2.Marketable = false; want true")
	}
	if !d2.Commodity {
		t.Error("d2.Commodity = false; want true")
	}
}

func TestAssetDescriptionUnmarshal_Empty(t *testing.T) {
	var descs []AssetDescription
	if err := json.Unmarshal([]byte(`[]`), &descs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(descs) != 0 {
		t.Errorf("len(descs) = %d; want 0", len(descs))
	}
}

func TestAssetDescriptionKey(t *testing.T) {
	got := AssetDescriptionKey(440, "101", "0")
	if want := "440_101_0"; got != want {
		t.Errorf("AssetDescriptionKey(440, 101, 0) = %q; want %q", got, want)
	}
}

func TestTradeAssetDescriptionKey(t *testing.T) {
	asset := TradeAsset{AppID: 730, ClassID: "200", InstanceID: "55"}
	got := asset.DescriptionKey()
	if want := "730_200_55"; got != want {
		t.Errorf("DescriptionKey() = %q; want %q", got, want)
	}
}
