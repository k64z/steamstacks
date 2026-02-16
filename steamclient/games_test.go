package steamclient

import (
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

func TestGamesPlayedMarshalRoundTrip(t *testing.T) {
	msg := &protocol.CMsgClientGamesPlayed{
		GamesPlayed: []*protocol.CMsgClientGamesPlayed_GamePlayed{
			{GameId: proto.Uint64(730)},
			{GameId: proto.Uint64(440)},
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got protocol.CMsgClientGamesPlayed
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.GetGamesPlayed()) != 2 {
		t.Fatalf("got %d games, want 2", len(got.GetGamesPlayed()))
	}

	if got.GetGamesPlayed()[0].GetGameId() != 730 {
		t.Errorf("game[0] = %d, want 730", got.GetGamesPlayed()[0].GetGameId())
	}
	if got.GetGamesPlayed()[1].GetGameId() != 440 {
		t.Errorf("game[1] = %d, want 440", got.GetGamesPlayed()[1].GetGameId())
	}
}

func TestGamesPlayedMarshalEmpty(t *testing.T) {
	msg := &protocol.CMsgClientGamesPlayed{}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got protocol.CMsgClientGamesPlayed
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.GetGamesPlayed()) != 0 {
		t.Fatalf("got %d games, want 0", len(got.GetGamesPlayed()))
	}
}
