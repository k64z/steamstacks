package steamclient

import (
	"sync"
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

func makePersonaStatePacket(t *testing.T, flags uint32, friends []*protocol.CMsgClientPersonaState_Friend) *Packet {
	t.Helper()
	body, err := proto.Marshal(&protocol.CMsgClientPersonaState{
		StatusFlags: proto.Uint32(flags),
		Friends:     friends,
	})
	if err != nil {
		t.Fatalf("marshal PersonaState: %v", err)
	}
	return &Packet{EMsg: EMsgClientPersonaState, IsProto: true, Body: body}
}

func TestHandlePersonaState(t *testing.T) {
	var mu sync.Mutex
	var events []PersonaStateEvent

	c := New(WithPersonaStateHandler(func(e *PersonaStateEvent) {
		mu.Lock()
		events = append(events, *e)
		mu.Unlock()
	}))

	pkt := makePersonaStatePacket(t, 339, []*protocol.CMsgClientPersonaState_Friend{
		{
			Friendid:        proto.Uint64(76561198012345678),
			PersonaState:    proto.Uint32(1), // Online
			PlayerName:      proto.String("Alice"),
			GamePlayedAppId: proto.Uint32(730),
			GameName:        proto.String("Counter-Strike 2"),
			LastLogoff:      proto.Uint32(1700000000),
			LastLogon:       proto.Uint32(1700000100),
		},
		{
			Friendid:     proto.Uint64(76561198087654321),
			PersonaState: proto.Uint32(3), // Away
			PlayerName:   proto.String("Bob"),
		},
	})

	c.handlePacket(pkt)

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	// First friend
	if events[0].SteamID != steamid.FromSteamID64(76561198012345678) {
		t.Errorf("events[0].SteamID = %v, want 76561198012345678", events[0].SteamID)
	}
	if events[0].State != PersonaStateOnline {
		t.Errorf("events[0].State = %d, want %d (Online)", events[0].State, PersonaStateOnline)
	}
	if events[0].PlayerName != "Alice" {
		t.Errorf("events[0].PlayerName = %q, want %q", events[0].PlayerName, "Alice")
	}
	if events[0].GameAppID != 730 {
		t.Errorf("events[0].GameAppID = %d, want 730", events[0].GameAppID)
	}
	if events[0].GameName != "Counter-Strike 2" {
		t.Errorf("events[0].GameName = %q, want %q", events[0].GameName, "Counter-Strike 2")
	}
	if events[0].LastLogoff != 1700000000 {
		t.Errorf("events[0].LastLogoff = %d, want 1700000000", events[0].LastLogoff)
	}
	if events[0].LastLogon != 1700000100 {
		t.Errorf("events[0].LastLogon = %d, want 1700000100", events[0].LastLogon)
	}
	if events[0].StatusFlags != 339 {
		t.Errorf("events[0].StatusFlags = %d, want 339", events[0].StatusFlags)
	}

	// Second friend
	if events[1].SteamID != steamid.FromSteamID64(76561198087654321) {
		t.Errorf("events[1].SteamID = %v, want 76561198087654321", events[1].SteamID)
	}
	if events[1].State != PersonaStateAway {
		t.Errorf("events[1].State = %d, want %d (Away)", events[1].State, PersonaStateAway)
	}
	if events[1].PlayerName != "Bob" {
		t.Errorf("events[1].PlayerName = %q, want %q", events[1].PlayerName, "Bob")
	}
}

func TestHandlePersonaStateNilHandler(t *testing.T) {
	c := New() // no handlers set

	pkt := makePersonaStatePacket(t, 339, []*protocol.CMsgClientPersonaState_Friend{
		{
			Friendid:     proto.Uint64(76561198012345678),
			PersonaState: proto.Uint32(1),
			PlayerName:   proto.String("Alice"),
		},
	})

	// Should not panic
	c.handlePacket(pkt)
}

func TestPersonaStateOnPacketPassthrough(t *testing.T) {
	var personaCalled, pktCalled bool

	c := New(
		WithPersonaStateHandler(func(e *PersonaStateEvent) { personaCalled = true }),
		WithPacketHandler(func(p *Packet) { pktCalled = true }),
	)

	pkt := makePersonaStatePacket(t, 339, []*protocol.CMsgClientPersonaState_Friend{
		{
			Friendid:     proto.Uint64(76561198012345678),
			PersonaState: proto.Uint32(1),
		},
	})

	c.handlePacket(pkt)

	if !personaCalled {
		t.Error("OnPersonaState was not called")
	}
	if !pktCalled {
		t.Error("OnPacket was not called for EMsgClientPersonaState")
	}
}

func TestPersonaStateString(t *testing.T) {
	tests := []struct {
		state PersonaState
		want  string
	}{
		{PersonaStateOffline, "Offline"},
		{PersonaStateOnline, "Online"},
		{PersonaStateBusy, "Busy"},
		{PersonaStateAway, "Away"},
		{PersonaStateSnooze, "Snooze"},
		{PersonaStateLookingToTrade, "LookingToTrade"},
		{PersonaStateLookingToPlay, "LookingToPlay"},
		{PersonaStateInvisible, "Invisible"},
		{PersonaState(99), "PersonaState(99)"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("PersonaState(%d).String() = %q, want %q", uint32(tt.state), got, tt.want)
		}
	}
}
