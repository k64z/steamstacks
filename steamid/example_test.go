package steamid_test

import (
	"fmt"

	"github.com/k64z/steamstacks/steamid"
)

func ExampleFromSteamID64() {
	sid := steamid.FromSteamID64(76561198000000000)
	fmt.Println(sid.ToSteam2ID())
	fmt.Println(sid.ToSteam3ID())
	fmt.Println(sid.AccountID())
	// Output:
	// STEAM_1:0:19867136
	// [U:1:39734272]
	// 39734272
}

func ExampleFromSteam2ID() {
	// The three textual forms all describe the same account.
	fmt.Println(steamid.FromSteam2ID("STEAM_1:0:19867136").ToSteamID64())
	fmt.Println(steamid.FromSteam3ID("[U:1:39734272]").ToSteamID64())
	// Output:
	// 76561198000000000
	// 76561198000000000
}
