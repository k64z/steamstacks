// Command cmclient logs into Steam's Connection Manager over WebSocket —
// the session the desktop client maintains — goes online, and prints
// friend messages, persona changes, and wallet pushes until Ctrl+C.
//
// Set STEAM_USERNAME / STEAM_PASSWORD / STEAM_SHARED_SECRET. The CM
// session is saved to steam_session_client.json for reuse; it is separate
// from the web session the browser-facing examples use, because Steam
// issues platform-specific tokens.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/k64z/steamstacks/steamclient"
	"github.com/k64z/steamstacks/steamsession"
	"github.com/k64z/steamstacks/steamtotp"
)

func main() {
	ctx := context.Background()

	session := loadOrLogin(ctx)

	client := steamclient.New(
		steamclient.WithTransport(steamclient.TransportWebSocket),
		steamclient.WithFriendMessageHandler(func(msg *steamclient.FriendMessage) {
			log.Printf("message from %s: %s", msg.Sender, msg.Message)
		}),
		steamclient.WithPersonaStateHandler(func(ev *steamclient.PersonaStateEvent) {
			log.Printf("persona: %s (%s) state=%d", ev.SteamID, ev.PlayerName, ev.State)
		}),
		steamclient.WithWalletInfoHandler(func(w *steamclient.WalletInfo) {
			log.Printf("wallet: balance=%d currency=%d", w.Balance, w.Currency)
		}),
		steamclient.WithDisconnectHandler(func(ev *steamclient.DisconnectEvent) {
			log.Printf("disconnected: %+v", ev)
		}),
	)

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := client.Login(ctx, os.Getenv("STEAM_USERNAME"), session.RefreshToken, session.SteamID); err != nil {
		log.Fatalf("login: %v", err)
	}
	if err := client.SetPersonaState(ctx, steamclient.PersonaStateOnline); err != nil {
		log.Fatalf("set persona state: %v", err)
	}

	log.Printf("online as %s — Ctrl+C to quit", client.SteamID())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh

	if err := client.Disconnect(); err != nil {
		log.Fatalf("disconnect: %v", err)
	}
}

// loadOrLogin reuses steam_session_client.json when it holds a valid
// session and falls back to a fresh device-code login otherwise. The CM
// protocol needs a SteamClient-platform session.
func loadOrLogin(ctx context.Context) *steamsession.Session {
	const sessionFile = "steam_session_client.json"

	session, err := steamsession.New(
		steamsession.WithPlatformType(steamsession.PlatformTypeSteamClient),
	)
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
	return session
}
