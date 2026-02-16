package steamclient

import (
	"context"
	"fmt"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

// PersonaState represents a Steam user's online status.
type PersonaState uint32

const (
	PersonaStateOffline        PersonaState = 0
	PersonaStateOnline         PersonaState = 1
	PersonaStateBusy           PersonaState = 2
	PersonaStateAway           PersonaState = 3
	PersonaStateSnooze         PersonaState = 4
	PersonaStateLookingToTrade PersonaState = 5
	PersonaStateLookingToPlay  PersonaState = 6
	PersonaStateInvisible      PersonaState = 7
)

var personaStateNames = map[PersonaState]string{
	PersonaStateOffline:        "Offline",
	PersonaStateOnline:         "Online",
	PersonaStateBusy:           "Busy",
	PersonaStateAway:           "Away",
	PersonaStateSnooze:         "Snooze",
	PersonaStateLookingToTrade: "LookingToTrade",
	PersonaStateLookingToPlay:  "LookingToPlay",
	PersonaStateInvisible:      "Invisible",
}

func (s PersonaState) String() string {
	if name, ok := personaStateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("PersonaState(%d)", uint32(s))
}

// PersonaStateEvent represents a persona state update for a Steam user.
type PersonaStateEvent struct {
	SteamID     steamid.SteamID
	StatusFlags uint32 // bitmask of what changed (EClientPersonaStateFlag)
	State       PersonaState
	PlayerName  string
	GameAppID   uint32
	GameName    string
	LastLogoff  uint32
	LastLogon   uint32
}

// handlePersonaState processes an EMsgClientPersonaState packet and dispatches PersonaStateEvents.
func (c *Client) handlePersonaState(pkt *Packet) {
	var msg protocol.CMsgClientPersonaState
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal PersonaState", "err", err)
		return
	}

	if c.OnPersonaState == nil {
		return
	}

	for _, f := range msg.GetFriends() {
		c.OnPersonaState(&PersonaStateEvent{
			SteamID:     steamid.FromSteamID64(f.GetFriendid()),
			StatusFlags: msg.GetStatusFlags(),
			State:       PersonaState(f.GetPersonaState()),
			PlayerName:  f.GetPlayerName(),
			GameAppID:   f.GetGamePlayedAppId(),
			GameName:    f.GetGameName(),
			LastLogoff:  f.GetLastLogoff(),
			LastLogon:   f.GetLastLogon(),
		})
	}
}

// SetPersonaState sets the logged-in user's online status (fire-and-forget).
func (c *Client) SetPersonaState(ctx context.Context, state PersonaState) error {
	body, err := proto.Marshal(&protocol.CMsgClientChangeStatus{
		PersonaState:     proto.Uint32(uint32(state)),
		PersonaSetByUser: proto.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("marshal ChangeStatus: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientChangeStatus, nil, body); err != nil {
		return fmt.Errorf("send ChangeStatus: %w", err)
	}

	return nil
}

// RequestFriendData requests persona data for the given Steam users (fire-and-forget).
// The server responds with EMsgClientPersonaState packets.
func (c *Client) RequestFriendData(ctx context.Context, friends []steamid.SteamID) error {
	ids := make([]uint64, len(friends))
	for i, f := range friends {
		ids[i] = f.ToSteamID64()
	}

	// Flags: Status(1) | PlayerName(2) | Presence(16) | LastSeen(64) | GameExtraInfo(256) = 339
	body, err := proto.Marshal(&protocol.CMsgClientRequestFriendData{
		PersonaStateRequested: proto.Uint32(339),
		Friends:               ids,
	})
	if err != nil {
		return fmt.Errorf("marshal RequestFriendData: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientRequestFriendData, nil, body); err != nil {
		return fmt.Errorf("send RequestFriendData: %w", err)
	}

	return nil
}
