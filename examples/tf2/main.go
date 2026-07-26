// Command tf2 connects to Steam, appears in-game in Team Fortress 2,
// establishes a Game Coordinator session, and prints the account's
// backpack once the shared-object cache has synced.
//
// Set STEAM_USERNAME / STEAM_PASSWORD / STEAM_SHARED_SECRET. Reuses
// steam_session_client.json (see the cmclient example).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/k64z/steamstacks/steamclient"
	"github.com/k64z/steamstacks/steamsession"
	"github.com/k64z/steamstacks/steamtotp"
	"github.com/k64z/steamstacks/tf2"
)

func main() {
	ctx := context.Background()

	session := loadOrLogin(ctx)

	client := steamclient.New(
		steamclient.WithTransport(steamclient.TransportWebSocket),
	)

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := client.Login(ctx, os.Getenv("STEAM_USERNAME"), session.RefreshToken, session.SteamID); err != nil {
		log.Fatalf("login: %v", err)
	}
	if err := client.SetGamesPlayed(ctx, []uint32{tf2.AppID}); err != nil {
		log.Fatalf("set games played: %v", err)
	}

	backpackLoaded := make(chan struct{})
	gc := tf2.New(client,
		tf2.WithConnectedHandler(func(ev *tf2.WelcomeEvent) {
			log.Printf("GC session established (version %d)", ev.Version)
		}),
		tf2.WithBackpackLoadedHandler(func(items []*tf2.Item) {
			close(backpackLoaded)
		}),
	)

	if err := gc.Connect(ctx); err != nil {
		log.Fatalf("GC connect: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	select {
	case <-backpackLoaded:
		items := gc.Backpack()
		fmt.Printf("%d items in backpack\n", len(items))
		for _, item := range items {
			fmt.Printf("  [%d] defindex=%d quality=%d level=%d\n",
				item.ID, item.DefIndex, item.Quality, item.Level)
		}
	case <-sigCh:
		log.Println("interrupted before backpack sync")
	}

	gc.Disconnect()
	if err := client.Disconnect(); err != nil {
		log.Fatalf("disconnect: %v", err)
	}
}

// loadOrLogin reuses steam_session_client.json when it holds a valid
// session and falls back to a fresh device-code login otherwise.
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
