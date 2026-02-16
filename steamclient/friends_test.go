package steamclient

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

func TestDecodeFriendsList(t *testing.T) {
	want := &protocol.CMsgClientFriendsList{
		Bincremental:   proto.Bool(false),
		MaxFriendCount: proto.Uint32(250),
		Friends: []*protocol.CMsgClientFriendsList_Friend{
			{
				Ulfriendid:          proto.Uint64(76561198012345678),
				Efriendrelationship: proto.Uint32(3), // Friend
			},
			{
				Ulfriendid:          proto.Uint64(76561198087654321),
				Efriendrelationship: proto.Uint32(2), // RequestRecipient
			},
		},
	}

	body, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("marshal test FriendsList: %v", err)
	}

	pkt := &Packet{
		EMsg:    EMsgClientFriendsList,
		IsProto: true,
		Body:    body,
	}

	got, err := DecodeFriendsList(pkt)
	if err != nil {
		t.Fatalf("DecodeFriendsList: %v", err)
	}

	if got.GetBincremental() != false {
		t.Errorf("Bincremental = %v, want false", got.GetBincremental())
	}
	if got.GetMaxFriendCount() != 250 {
		t.Errorf("MaxFriendCount = %d, want 250", got.GetMaxFriendCount())
	}
	if len(got.GetFriends()) != 2 {
		t.Fatalf("len(Friends) = %d, want 2", len(got.GetFriends()))
	}
	if got.GetFriends()[0].GetUlfriendid() != 76561198012345678 {
		t.Errorf("Friends[0].Ulfriendid = %d, want 76561198012345678", got.GetFriends()[0].GetUlfriendid())
	}
	if got.GetFriends()[0].GetEfriendrelationship() != 3 {
		t.Errorf("Friends[0].Efriendrelationship = %d, want 3", got.GetFriends()[0].GetEfriendrelationship())
	}
	if got.GetFriends()[1].GetUlfriendid() != 76561198087654321 {
		t.Errorf("Friends[1].Ulfriendid = %d, want 76561198087654321", got.GetFriends()[1].GetUlfriendid())
	}
}

func TestIgnoreFriendBodyEncoding(t *testing.T) {
	self := steamid.FromSteamID64(76561198000000001)
	friend := steamid.FromSteamID64(76561198000000002)

	body := encodeIgnoreFriendBody(self, friend, true)

	if len(body) != 17 {
		t.Fatalf("body length = %d, want 17", len(body))
	}

	gotSelf := binary.LittleEndian.Uint64(body[0:8])
	if gotSelf != 76561198000000001 {
		t.Errorf("self steamid = %d, want 76561198000000001", gotSelf)
	}

	gotFriend := binary.LittleEndian.Uint64(body[8:16])
	if gotFriend != 76561198000000002 {
		t.Errorf("friend steamid = %d, want 76561198000000002", gotFriend)
	}

	if body[16] != 1 {
		t.Errorf("ignore byte = %d, want 1", body[16])
	}

	// Test unblock (ignore=false)
	body2 := encodeIgnoreFriendBody(self, friend, false)
	if body2[16] != 0 {
		t.Errorf("ignore byte (unblock) = %d, want 0", body2[16])
	}
}

func TestIgnoreFriendResponseDecoding(t *testing.T) {
	// Build a 12-byte response: [FriendId: uint64 LE][Result: uint32 LE]
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint64(buf[0:8], 76561198000000002)
	binary.LittleEndian.PutUint32(buf[8:12], 1) // EResult OK

	result, err := decodeIgnoreFriendResponse(buf)
	if err != nil {
		t.Fatalf("decodeIgnoreFriendResponse: %v", err)
	}
	if result != 1 {
		t.Errorf("result = %d, want 1", result)
	}

	// Test failure result
	binary.LittleEndian.PutUint32(buf[8:12], 2) // EResult Fail
	result, err = decodeIgnoreFriendResponse(buf)
	if err != nil {
		t.Fatalf("decodeIgnoreFriendResponse: %v", err)
	}
	if result != 2 {
		t.Errorf("result = %d, want 2", result)
	}

	// Test too-short body
	_, err = decodeIgnoreFriendResponse(buf[:5])
	if err == nil {
		t.Error("expected error for short body, got nil")
	}
}

func makeFriendsListPacket(t *testing.T, incremental bool, friends []*protocol.CMsgClientFriendsList_Friend) *Packet {
	t.Helper()
	body, err := proto.Marshal(&protocol.CMsgClientFriendsList{
		Bincremental: proto.Bool(incremental),
		Friends:      friends,
	})
	if err != nil {
		t.Fatalf("marshal FriendsList: %v", err)
	}
	return &Packet{EMsg: EMsgClientFriendsList, IsProto: true, Body: body}
}

func makeFriendMsgIncomingPacket(t *testing.T, emsg EMsg, from uint64, entryType int32, msg string, limited bool, ts uint32) *Packet {
	t.Helper()
	body, err := proto.Marshal(&protocol.CMsgClientFriendMsgIncoming{
		SteamidFrom:          proto.Uint64(from),
		ChatEntryType:        proto.Int32(entryType),
		FromLimitedAccount:   proto.Bool(limited),
		Message:              []byte(msg),
		Rtime32ServerTimestamp: proto.Uint32(ts),
	})
	if err != nil {
		t.Fatalf("marshal FriendMsgIncoming: %v", err)
	}
	return &Packet{EMsg: emsg, IsProto: true, Body: body}
}

func TestHandleFriendsList(t *testing.T) {
	var mu sync.Mutex
	var events []RelationshipEvent

	c := New(WithRelationshipHandler(func(e *RelationshipEvent) {
		mu.Lock()
		events = append(events, *e)
		mu.Unlock()
	}))

	pkt := makeFriendsListPacket(t, false, []*protocol.CMsgClientFriendsList_Friend{
		{Ulfriendid: proto.Uint64(76561198012345678), Efriendrelationship: proto.Uint32(3)},
		{Ulfriendid: proto.Uint64(76561198087654321), Efriendrelationship: proto.Uint32(2)},
	})

	c.handlePacket(pkt)

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].SteamID != steamid.FromSteamID64(76561198012345678) {
		t.Errorf("events[0].SteamID = %v, want 76561198012345678", events[0].SteamID)
	}
	if events[0].Relationship != RelationshipFriend {
		t.Errorf("events[0].Relationship = %d, want %d", events[0].Relationship, RelationshipFriend)
	}
	if events[0].Incremental != false {
		t.Errorf("events[0].Incremental = %v, want false", events[0].Incremental)
	}
	if events[1].SteamID != steamid.FromSteamID64(76561198087654321) {
		t.Errorf("events[1].SteamID = %v, want 76561198087654321", events[1].SteamID)
	}
	if events[1].Relationship != RelationshipRequestRecipient {
		t.Errorf("events[1].Relationship = %d, want %d", events[1].Relationship, RelationshipRequestRecipient)
	}
}

func TestHandleFriendsListIncremental(t *testing.T) {
	var event RelationshipEvent
	c := New(WithRelationshipHandler(func(e *RelationshipEvent) {
		event = *e
	}))

	pkt := makeFriendsListPacket(t, true, []*protocol.CMsgClientFriendsList_Friend{
		{Ulfriendid: proto.Uint64(76561198012345678), Efriendrelationship: proto.Uint32(0)},
	})

	c.handlePacket(pkt)

	if event.Incremental != true {
		t.Errorf("Incremental = %v, want true", event.Incremental)
	}
	if event.Relationship != RelationshipNone {
		t.Errorf("Relationship = %d, want %d", event.Relationship, RelationshipNone)
	}
}

func TestHandleFriendMsgIncoming(t *testing.T) {
	var got FriendMessage
	c := New(WithFriendMessageHandler(func(m *FriendMessage) {
		got = *m
	}))

	pkt := makeFriendMsgIncomingPacket(t, EMsgClientFriendMsgIncoming, 76561198012345678, 1, "hello\x00", false, 1700000000)
	c.handlePacket(pkt)

	if got.Sender != steamid.FromSteamID64(76561198012345678) {
		t.Errorf("Sender = %v, want 76561198012345678", got.Sender)
	}
	if got.EntryType != ChatEntryTypeChatMsg {
		t.Errorf("EntryType = %d, want %d", got.EntryType, ChatEntryTypeChatMsg)
	}
	if got.Message != "hello" {
		t.Errorf("Message = %q, want %q", got.Message, "hello")
	}
	if got.FromLimitedAccount != false {
		t.Errorf("FromLimitedAccount = %v, want false", got.FromLimitedAccount)
	}
	if got.ServerTimestamp != 1700000000 {
		t.Errorf("ServerTimestamp = %d, want 1700000000", got.ServerTimestamp)
	}
}

func TestHandleFriendMsgEcho(t *testing.T) {
	var got FriendMessage
	c := New(WithFriendMessageHandler(func(m *FriendMessage) {
		got = *m
	}))

	pkt := makeFriendMsgIncomingPacket(t, EMsgClientFriendMsgEchoToSender, 76561198012345678, 1, "echo\x00", false, 1700000001)
	c.handlePacket(pkt)

	if got.Message != "echo" {
		t.Errorf("Message = %q, want %q", got.Message, "echo")
	}
	if got.ServerTimestamp != 1700000001 {
		t.Errorf("ServerTimestamp = %d, want 1700000001", got.ServerTimestamp)
	}
	if got.Echo != true {
		t.Errorf("Echo = %v, want true", got.Echo)
	}
}

func TestHandleFriendMsgIncomingNotEcho(t *testing.T) {
	var got FriendMessage
	c := New(WithFriendMessageHandler(func(m *FriendMessage) {
		got = *m
	}))

	pkt := makeFriendMsgIncomingPacket(t, EMsgClientFriendMsgIncoming, 76561198012345678, 1, "hi\x00", false, 1700000000)
	c.handlePacket(pkt)

	if got.Echo != false {
		t.Errorf("Echo = %v, want false", got.Echo)
	}
}

func TestNilHandlerSafety(t *testing.T) {
	c := New() // no handlers set

	// Should not panic
	pkt1 := makeFriendsListPacket(t, false, []*protocol.CMsgClientFriendsList_Friend{
		{Ulfriendid: proto.Uint64(76561198012345678), Efriendrelationship: proto.Uint32(3)},
	})
	c.handlePacket(pkt1)

	pkt2 := makeFriendMsgIncomingPacket(t, EMsgClientFriendMsgIncoming, 76561198012345678, 1, "test\x00", false, 0)
	c.handlePacket(pkt2)
}

func TestOnPacketPassthrough(t *testing.T) {
	var relCalled, pktCalled bool

	c := New(
		WithRelationshipHandler(func(e *RelationshipEvent) { relCalled = true }),
		WithPacketHandler(func(p *Packet) { pktCalled = true }),
	)

	pkt := makeFriendsListPacket(t, false, []*protocol.CMsgClientFriendsList_Friend{
		{Ulfriendid: proto.Uint64(76561198012345678), Efriendrelationship: proto.Uint32(3)},
	})
	c.handlePacket(pkt)

	if !relCalled {
		t.Error("OnRelationship was not called")
	}
	if !pktCalled {
		t.Error("OnPacket was not called for EMsgClientFriendsList")
	}
}

func TestSendMessageBody(t *testing.T) {
	target := steamid.FromSteamID64(76561198012345678)
	sid := target.ToSteamID64()
	entryType := int32(ChatEntryTypeChatMsg)

	msg := &protocol.CMsgClientFriendMsg{
		Steamid:       &sid,
		ChatEntryType: &entryType,
		Message:       append([]byte("hi there"), 0x00),
	}

	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got protocol.CMsgClientFriendMsg
	if err := proto.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GetSteamid() != 76561198012345678 {
		t.Errorf("Steamid = %d, want 76561198012345678", got.GetSteamid())
	}
	if got.GetChatEntryType() != 1 {
		t.Errorf("ChatEntryType = %d, want 1", got.GetChatEntryType())
	}
	// Verify message includes null terminator
	rawMsg := got.GetMessage()
	if len(rawMsg) == 0 {
		t.Fatal("Message is empty")
	}
	if rawMsg[len(rawMsg)-1] != 0x00 {
		t.Errorf("Message missing null terminator, last byte = %#x", rawMsg[len(rawMsg)-1])
	}
	if string(rawMsg[:len(rawMsg)-1]) != "hi there" {
		t.Errorf("Message text = %q, want %q", string(rawMsg[:len(rawMsg)-1]), "hi there")
	}
}
