package steamapi

import "testing"

func TestConvertDescriptions(t *testing.T) {
	raw := []rawAssetDescription{
		{
			AppID:          440,
			ClassID:        "101",
			InstanceID:     "0",
			Name:           "The Sydney Sleeper",
			MarketHashName: "The Sydney Sleeper",
			Type:           "Level 1 Sniper Rifle",
			Tradable:       1,
			Marketable:     0,
			Commodity:      0,
			IconURL:        "icon_101",
			IconURLLarge:   "icon_101_large",
			Descriptions: []DescriptionLine{
				{Value: "+25% charge rate", Color: "7ea9d1"},
			},
			Tags: []Tag{
				{Category: "Quality", InternalName: "Unique", LocalizedCategoryName: "Quality", LocalizedTagName: "Unique", Color: "7D6D00"},
			},
			Actions: []Action{
				{Link: "http://example.com", Name: "Wiki Page"},
			},
			FraudWarnings: []string{"renamed"},
		},
		{
			AppID:      730,
			ClassID:    "200",
			InstanceID: "55",
			Name:       "AK-47 | Redline",
			Type:       "Classified Rifle",
			Tradable:   1,
			Marketable: 1,
			Commodity:  1,
		},
	}

	m := convertDescriptions(raw)

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

func TestConvertDescriptions_Empty(t *testing.T) {
	if m := convertDescriptions(nil); m != nil {
		t.Errorf("convertDescriptions(nil) = %v; want nil", m)
	}
	if m := convertDescriptions([]rawAssetDescription{}); m != nil {
		t.Errorf("convertDescriptions([]) = %v; want nil", m)
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
