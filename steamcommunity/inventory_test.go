package steamcommunity

import (
	"os"
	"testing"
)

func TestParseInventoryResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/inventory_sample.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	items, hasMore, lastAssetID, err := parseInventoryResponse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Check pagination fields
	if hasMore {
		t.Error("hasMore = true; want false")
	}
	if lastAssetID != "1003" {
		t.Errorf("lastAssetID = %q; want %q", lastAssetID, "1003")
	}

	// Check item count
	if got, want := len(items), 3; got != want {
		t.Fatalf("len(items) = %d; want %d", got, want)
	}

	// Item 1: Sydney Sleeper - tradable, not marketable
	item1 := items[0]
	if got, want := item1.AssetID, "1001"; got != want {
		t.Errorf("item1.AssetID = %q; want %q", got, want)
	}
	if got, want := item1.ClassID, "101"; got != want {
		t.Errorf("item1.ClassID = %q; want %q", got, want)
	}
	if got, want := item1.Name, "The Sydney Sleeper"; got != want {
		t.Errorf("item1.Name = %q; want %q", got, want)
	}
	if got, want := item1.Type, "Level 1 Sniper Rifle"; got != want {
		t.Errorf("item1.Type = %q; want %q", got, want)
	}
	if !item1.Tradable {
		t.Error("item1.Tradable = false; want true")
	}
	if item1.Marketable {
		t.Error("item1.Marketable = true; want false")
	}
	if item1.Commodity {
		t.Error("item1.Commodity = true; want false")
	}
	if got, want := len(item1.Descriptions), 2; got != want {
		t.Errorf("len(item1.Descriptions) = %d; want %d", got, want)
	}
	if got, want := len(item1.Tags), 2; got != want {
		t.Errorf("len(item1.Tags) = %d; want %d", got, want)
	}
	if got, want := len(item1.Actions), 1; got != want {
		t.Errorf("len(item1.Actions) = %d; want %d", got, want)
	}

	// Item 2: Key - tradable, marketable, commodity, has instanceid
	item2 := items[1]
	if got, want := item2.AssetID, "1002"; got != want {
		t.Errorf("item2.AssetID = %q; want %q", got, want)
	}
	if got, want := item2.InstanceID, "201"; got != want {
		t.Errorf("item2.InstanceID = %q; want %q", got, want)
	}
	if got, want := item2.Name, "Mann Co. Supply Crate Key"; got != want {
		t.Errorf("item2.Name = %q; want %q", got, want)
	}
	if !item2.Tradable {
		t.Error("item2.Tradable = false; want true")
	}
	if !item2.Marketable {
		t.Error("item2.Marketable = false; want true")
	}
	if !item2.Commodity {
		t.Error("item2.Commodity = false; want true")
	}

	// Item 3: Refined Metal - not tradable, has fraudwarnings, amount=2
	item3 := items[2]
	if got, want := item3.AssetID, "1003"; got != want {
		t.Errorf("item3.AssetID = %q; want %q", got, want)
	}
	if got, want := item3.Name, "Refined Metal"; got != want {
		t.Errorf("item3.Name = %q; want %q", got, want)
	}
	if got, want := item3.Amount, "2"; got != want {
		t.Errorf("item3.Amount = %q; want %q", got, want)
	}
	if item3.Tradable {
		t.Error("item3.Tradable = true; want false")
	}
	if item3.Marketable {
		t.Error("item3.Marketable = true; want false")
	}
	if got, want := len(item3.FraudWarnings), 1; got != want {
		t.Errorf("len(item3.FraudWarnings) = %d; want %d", got, want)
	}
}

func TestParseInventoryResponse_InvalidJSON(t *testing.T) {
	_, _, _, err := parseInventoryResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseInventoryResponse_FailedRequest(t *testing.T) {
	data := []byte(`{"success": 0}`)
	_, _, _, err := parseInventoryResponse(data)
	if err == nil {
		t.Error("expected error for success=0")
	}
}
