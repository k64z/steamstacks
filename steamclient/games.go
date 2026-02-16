package steamclient

import (
	"context"
	"fmt"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// SetGamesPlayed tells Steam which games we are currently playing.
// Pass app IDs to appear in-game; pass an empty slice to stop playing.
func (c *Client) SetGamesPlayed(ctx context.Context, appIDs []uint32) error {
	games := make([]*protocol.CMsgClientGamesPlayed_GamePlayed, len(appIDs))
	for i, id := range appIDs {
		games[i] = &protocol.CMsgClientGamesPlayed_GamePlayed{
			GameId: proto.Uint64(uint64(id)),
		}
	}

	body, err := proto.Marshal(&protocol.CMsgClientGamesPlayed{
		GamesPlayed: games,
	})
	if err != nil {
		return fmt.Errorf("marshal GamesPlayed: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientGamesPlayed, nil, body); err != nil {
		return fmt.Errorf("send GamesPlayed: %w", err)
	}

	return nil
}
