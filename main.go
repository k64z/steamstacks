package main

import (
	"context"
	"log"
	"os"

	"github.com/k64z/steamstacks/steamsession"
)

func main() {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")

	session := steamsession.New(username, password)

	ctx := context.Background()

	err := session.StartWithCredentials(ctx)
	if err != nil {
		log.Fatalf("main error: %v", err)
	}
}
