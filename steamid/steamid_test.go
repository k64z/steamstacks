package steamid_test

import (
	"testing"

	"github.com/k64z/steamstacks/steamid"
)

func TestFromSteam2ID(t *testing.T) {
	tests := map[string]struct {
		id   string
		want steamid.SteamID
	}{
		"valid": {
			id:   "STEAM_0:0:11101",
			want: 76561197960287930,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := steamid.FromSteam2ID(tt.id)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFromSteam3ID(t *testing.T) {
	testCases := map[string]struct {
		id   string
		want steamid.SteamID
	}{
		"valid": {
			id:   "[U:1:22202]",
			want: 76561197960287930,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := steamid.FromSteam3ID(tc.id)
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFromSteamID64(t *testing.T) {
	testCases := map[string]struct {
		id   uint64
		want steamid.SteamID
	}{
		"valid": {
			id:   76561197960287930,
			want: 76561197960287930,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := steamid.FromSteamID64(tc.id)
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	testCases := map[string]struct {
		id   string
		want steamid.SteamID
	}{
		"valid": {
			id:   "76561197960287930",
			want: 76561197960287930,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := steamid.FromString(tc.id)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestToSteam2ID(t *testing.T) {
	testCases := map[string]struct {
		id            uint64
		wantSteam2ID  string
		wantSteam3ID  string
		wantSteamID64 uint64
		wantAccountID uint64
		wantString    string
	}{
		"valid": {
			id:            76561197960287930,
			wantSteam2ID:  "STEAM_1:0:11101",
			wantSteam3ID:  "[U:1:22202]",
			wantSteamID64: 76561197960287930,
			wantAccountID: 22202,
			wantString:    "76561197960287930",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sid := steamid.SteamID(tc.id)

			steam2ID := sid.ToSteam2ID()
			if steam2ID != tc.wantSteam2ID {
				t.Errorf("got %s, want %s", steam2ID, tc.wantSteam2ID)
			}

			steam3ID := sid.ToSteam3ID()
			if steam3ID != tc.wantSteam3ID {
				t.Errorf("got %s, want %s", steam3ID, tc.wantSteam3ID)
			}

			steamID64 := sid.ToSteamID64()
			if steamID64 != tc.wantSteamID64 {
				t.Errorf("got %d, want %d", steamID64, tc.wantSteamID64)
			}

			str := sid.String()
			if str != tc.wantString {
				t.Errorf("got %s, want %s", str, tc.wantString)
			}

		})
	}
}
