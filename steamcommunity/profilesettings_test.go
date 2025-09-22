package steamcommunity

import (
	"os"
	"testing"
)

func TestParseProfileData(t *testing.T) {
	f, err := os.Open("testdata/steam_profile_sample.html")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	defer f.Close()

	data, err := parseProfileData(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := data.EditConfig.PersonaName, "SampleUser"; got != want {
		t.Errorf("PersonaName = %q; want %q", got, want)
	}
	if got, want := data.EditConfig.RealName, "John Doe"; got != want {
		t.Errorf("RealName = %q; want %q", got, want)
	}
	if got := data.EditConfig.Privacy.PrivacySettings.PrivacyProfile; got != PrivacyOptionPublic {
		t.Errorf("PrivacyProfile = %d, want 3", got)
	}
}
