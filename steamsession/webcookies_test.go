package steamsession

import (
	"regexp"
	"testing"
)

func TestMustGenerateSessionID(t *testing.T) {
	t.Run("validate format", func(t *testing.T) {
		sessionID := mustGenerateSessionID()

		if len(sessionID) != 24 {
			t.Errorf("want sessionID length of 24, got %d", len(sessionID))
		}

		hexPattern := regexp.MustCompile("^[0-9a-f]+$")
		if !hexPattern.MatchString(sessionID) {
			t.Errorf("sessionID contains non-hexadecimal characters: %s", sessionID)
		}
	})

	t.Run("check uniqueness", func(t *testing.T) {
		sessionIDs := make(map[string]bool)
		for range 1000 {
			id := mustGenerateSessionID()
			if sessionIDs[id] {
				t.Errorf("duplicate sessionID generated: %s", id)
			}
			sessionIDs[id] = true
		}
	})
}
