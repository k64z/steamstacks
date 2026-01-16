package steamstore

import (
	"encoding/json"
	"os"
	"testing"
)

func TestParseUserData(t *testing.T) {
	data, err := os.ReadFile("testdata/userdata.json")
	if err != nil {
		t.Skipf("testdata/userdata.json not found: %v", err)
	}

	var userData UserData
	err = json.Unmarshal(data, &userData)
	if err != nil {
		t.Fatalf("failed to unmarshal userdata: %v", err)
	}

	// Verify basic fields are populated
	if len(userData.OwnedApps) == 0 {
		t.Error("expected OwnedApps to be non-empty")
	}

	if len(userData.OwnedPackages) == 0 {
		t.Error("expected OwnedPackages to be non-empty")
	}

	if len(userData.WishlistedApps) == 0 {
		t.Error("expected WishlistedApps to be non-empty")
	}

	if len(userData.RecommendedTags) == 0 {
		t.Error("expected RecommendedTags to be non-empty")
	}

	// Verify tag structure
	firstTag := userData.RecommendedTags[0]
	if firstTag.TagID == 0 {
		t.Error("expected first tag to have TagID")
	}
	if firstTag.Name == "" {
		t.Error("expected first tag to have Name")
	}

	t.Logf("Parsed %d owned apps, %d packages, %d wishlisted, %d tags",
		len(userData.OwnedApps),
		len(userData.OwnedPackages),
		len(userData.WishlistedApps),
		len(userData.RecommendedTags))
}
