package main

import (
	"context"
	"log"
	"os"

	"github.com/k64z/steamstacks/steamsession"
	"github.com/k64z/steamstacks/steamtotp"
)

// loadOrLogin reuses steam_session_web.json when it holds a valid session
// and falls back to a fresh device-code login otherwise.
func loadOrLogin(ctx context.Context) *steamsession.Session {
	const sessionFile = "steam_session_web.json"

	session, err := steamsession.New()
	if err != nil {
		log.Fatalf("create session: %v", err)
	}

	if err := session.LoadFromFile(sessionFile); err != nil || !session.IsValidToken(ctx) {
		username := os.Getenv("STEAM_USERNAME")
		password := os.Getenv("STEAM_PASSWORD")
		sharedSecret := os.Getenv("STEAM_SHARED_SECRET")
		if username == "" || password == "" || sharedSecret == "" {
			log.Fatal("no saved session; set STEAM_USERNAME, STEAM_PASSWORD and STEAM_SHARED_SECRET")
		}

		code, err := steamtotp.GenerateAuthCode(sharedSecret, 0)
		if err != nil {
			log.Fatalf("generate auth code: %v", err)
		}
		if err := session.LoginWithDeviceCode(ctx, username, password, code); err != nil {
			log.Fatalf("login: %v", err)
		}
		if err := session.SaveToFile(sessionFile); err != nil {
			log.Printf("warning: save session: %v", err)
		}
	}

	if err := session.GetWebCookies(ctx); err != nil {
		log.Fatalf("get web cookies: %v", err)
	}
	return session
}
