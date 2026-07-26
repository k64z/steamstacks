// Command login authenticates with Steam using account credentials and a
// Steam Guard mobile authenticator secret, then saves the session to
// steam_session_web.json so the other examples can reuse it without
// logging in again.
//
// Required environment variables:
//   - STEAM_USERNAME
//   - STEAM_PASSWORD
//   - STEAM_SHARED_SECRET (base64 shared_secret from your authenticator)
package main

import (
	"context"
	"log"
	"os"

	"github.com/k64z/steamstacks/steamsession"
	"github.com/k64z/steamstacks/steamtotp"
)

func main() {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")
	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")
	if username == "" || password == "" || sharedSecret == "" {
		log.Fatal("set STEAM_USERNAME, STEAM_PASSWORD and STEAM_SHARED_SECRET")
	}

	ctx := context.Background()

	session, err := steamsession.New()
	if err != nil {
		log.Fatalf("create session: %v", err)
	}

	code, err := steamtotp.GenerateAuthCode(sharedSecret, 0)
	if err != nil {
		log.Fatalf("generate auth code: %v", err)
	}

	if err := session.LoginWithDeviceCode(ctx, username, password, code); err != nil {
		log.Fatalf("login: %v", err)
	}

	if err := session.SaveToFile("steam_session_web.json"); err != nil {
		log.Fatalf("save session: %v", err)
	}

	log.Printf("logged in as %s, session saved to steam_session_web.json", session.SteamID)
}
