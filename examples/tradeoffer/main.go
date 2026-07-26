// Command tradeoffer sends a one-item trade offer and, if Steam asks for
// mobile confirmation, accepts it with the authenticator's
// identity_secret.
//
// Run the login example first to create steam_session_web.json (or set
// STEAM_USERNAME / STEAM_PASSWORD / STEAM_SHARED_SECRET), then:
//
//	TRADE_PARTNER=76561198000000000 \
//	TRADE_TOKEN=AbCdEfGh \            # only needed when not friends
//	TRADE_ASSET_ID=1234567890 \       # asset from your TF2 backpack
//	STEAM_IDENTITY_SECRET=... \       # base64, for the confirmation
//	go run .
package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"

	"github.com/k64z/steamstacks/steamapi"
	"github.com/k64z/steamstacks/steamcommunity"
	"github.com/k64z/steamstacks/steamid"
)

func main() {
	ctx := context.Background()

	partner, err := steamid.FromString(os.Getenv("TRADE_PARTNER"))
	if err != nil {
		log.Fatalf("TRADE_PARTNER: %v", err)
	}
	assetID := os.Getenv("TRADE_ASSET_ID")
	if assetID == "" {
		log.Fatal("set TRADE_ASSET_ID to an asset from your TF2 backpack (see the inventory example)")
	}

	session := loadOrLogin(ctx)

	community, err := steamcommunity.New(
		steamcommunity.WithHTTPClient(session.HTTPClient()),
	)
	if err != nil {
		log.Fatalf("create community client: %v", err)
	}

	resp, err := community.SendTradeOffer(ctx, steamcommunity.SendTradeOfferOptions{
		Partner: partner,
		Token:   os.Getenv("TRADE_TOKEN"),
		Message: "steamstacks example offer",
		ItemsToGive: []steamapi.TradeAsset{
			{AppID: 440, ContextID: "2", AssetID: assetID, Amount: "1"},
		},
	})
	if err != nil {
		log.Fatalf("send trade offer: %v", err)
	}
	log.Printf("offer %s sent", resp.TradeOfferID)

	if !resp.NeedsConfirmation {
		return
	}

	identitySecret, err := base64.StdEncoding.DecodeString(os.Getenv("STEAM_IDENTITY_SECRET"))
	if err != nil || len(identitySecret) == 0 {
		log.Fatal("offer needs mobile confirmation; set STEAM_IDENTITY_SECRET (base64)")
	}

	if err := community.AcceptConfirmationByCreatorID(ctx, identitySecret, resp.TradeOfferID); err != nil {
		log.Fatalf("accept confirmation: %v", err)
	}
	log.Println("offer confirmed")
}
