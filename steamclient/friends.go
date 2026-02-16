package steamclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

// ChatEntryType identifies the kind of chat message.
type ChatEntryType int32

const (
	ChatEntryTypeChatMsg ChatEntryType = 1
	ChatEntryTypeTyping  ChatEntryType = 2
)

// FriendRelationship represents the relationship state between two Steam users.
type FriendRelationship uint32

const (
	RelationshipNone             FriendRelationship = 0
	RelationshipBlocked          FriendRelationship = 1
	RelationshipRequestRecipient FriendRelationship = 2
	RelationshipFriend           FriendRelationship = 3
	RelationshipRequestInitiator FriendRelationship = 4
	RelationshipIgnored          FriendRelationship = 5
	RelationshipIgnoredFriend    FriendRelationship = 6
)

// FriendMessage represents an incoming chat message from a friend.
type FriendMessage struct {
	Sender             steamid.SteamID
	EntryType          ChatEntryType
	Message            string
	FromLimitedAccount bool
	ServerTimestamp    uint32
	Echo               bool // true when this is our own message echoed back (EMsgClientFriendMsgEchoToSender)
}

// RelationshipEvent represents a change in relationship state with a Steam user.
type RelationshipEvent struct {
	SteamID      steamid.SteamID
	Relationship FriendRelationship
	Incremental  bool // false = full list after login, true = live change
}

// AddFriend sends a friend request to the given Steam user.
// It returns the server's response containing the result and persona name.
func (c *Client) AddFriend(ctx context.Context, target steamid.SteamID) (*protocol.CMsgClientAddFriendResponse, error) {
	responseCh := c.expectEMsg(EMsgClientAddFriendResponse)

	sid := target.ToSteamID64()
	body, err := proto.Marshal(&protocol.CMsgClientAddFriend{
		SteamidToAdd: &sid,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal AddFriend: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientAddFriend, nil, body); err != nil {
		return nil, fmt.Errorf("send AddFriend: %w", err)
	}

	pkt, err := c.awaitPacket(ctx, responseCh)
	if err != nil {
		return nil, fmt.Errorf("wait for AddFriend response: %w", err)
	}

	var resp protocol.CMsgClientAddFriendResponse
	if err := proto.Unmarshal(pkt.Body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal AddFriend response: %w", err)
	}

	if resp.GetEresult() != 1 {
		return &resp, fmt.Errorf("AddFriend failed: eresult=%d", resp.GetEresult())
	}

	return &resp, nil
}

// RemoveFriend removes a friend from the logged-in user's friend list.
// This is fire-and-forget; the server sends an incremental FriendsList update instead of a direct response.
func (c *Client) RemoveFriend(ctx context.Context, target steamid.SteamID) error {
	sid := target.ToSteamID64()
	body, err := proto.Marshal(&protocol.CMsgClientRemoveFriend{
		Friendid: &sid,
	})
	if err != nil {
		return fmt.Errorf("marshal RemoveFriend: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientRemoveFriend, nil, body); err != nil {
		return fmt.Errorf("send RemoveFriend: %w", err)
	}

	return nil
}

// IgnoreFriend blocks or unblocks a Steam user. This uses the legacy non-protobuf
// wire format (MsgClientSetIgnoreFriend).
func (c *Client) IgnoreFriend(ctx context.Context, target steamid.SteamID, ignore bool) error {
	responseCh := c.expectEMsg(EMsgClientSetIgnoreFriendResponse)

	body := encodeIgnoreFriendBody(c.steamID, target, ignore)

	if err := c.sendNonProtoPacket(ctx, EMsgClientSetIgnoreFriend, body); err != nil {
		return fmt.Errorf("send SetIgnoreFriend: %w", err)
	}

	pkt, err := c.awaitPacket(ctx, responseCh)
	if err != nil {
		return fmt.Errorf("wait for SetIgnoreFriend response: %w", err)
	}

	result, err := decodeIgnoreFriendResponse(pkt.Body)
	if err != nil {
		return err
	}

	if result != 1 {
		return fmt.Errorf("SetIgnoreFriend failed: eresult=%d", result)
	}

	return nil
}

// encodeIgnoreFriendBody builds the 17-byte non-proto body for EMsgClientSetIgnoreFriend.
// Layout: [MySteamId: uint64 LE][SteamIdFriend: uint64 LE][Ignore: byte]
func encodeIgnoreFriendBody(self steamid.SteamID, friend steamid.SteamID, ignore bool) []byte {
	buf := make([]byte, 17)
	binary.LittleEndian.PutUint64(buf[0:8], self.ToSteamID64())
	binary.LittleEndian.PutUint64(buf[8:16], friend.ToSteamID64())
	if ignore {
		buf[16] = 1
	}
	return buf
}

// decodeIgnoreFriendResponse parses the 12-byte non-proto response body for EMsgClientSetIgnoreFriendResponse.
// Layout: [FriendId: uint64 LE][Result: uint32 LE]
func decodeIgnoreFriendResponse(body []byte) (uint32, error) {
	if len(body) < 12 {
		return 0, fmt.Errorf("SetIgnoreFriendResponse too short: %d bytes", len(body))
	}
	result := binary.LittleEndian.Uint32(body[8:12])
	return result, nil
}

// sendNonProtoPacket sends a non-protobuf CM message with the extended header.
func (c *Client) sendNonProtoPacket(ctx context.Context, emsg EMsg, body []byte) error {
	c.mu.Lock()
	sid := c.steamID.ToSteamID64()
	sessionID := c.sessionID
	c.mu.Unlock()

	pkt := &Packet{
		EMsg:    emsg,
		IsProto: false,
		Header: &protocol.CMsgProtoBufHeader{
			Steamid:         &sid,
			ClientSessionid: &sessionID,
		},
		Body: body,
	}

	data, err := encodePacket(pkt)
	if err != nil {
		return fmt.Errorf("encode %s: %w", emsg, err)
	}

	return c.conn.Write(ctx, data)
}

// DecodeFriendsList unmarshals a CMsgClientFriendsList from an EMsgClientFriendsList packet.
// The server pushes this after login (full list) and on changes (incremental).
func DecodeFriendsList(pkt *Packet) (*protocol.CMsgClientFriendsList, error) {
	var msg protocol.CMsgClientFriendsList
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal FriendsList: %w", err)
	}
	return &msg, nil
}

// handleFriendsList processes an EMsgClientFriendsList packet and dispatches RelationshipEvents.
func (c *Client) handleFriendsList(pkt *Packet) {
	var msg protocol.CMsgClientFriendsList
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal FriendsList", "err", err)
		return
	}

	if c.OnRelationship == nil {
		return
	}

	incremental := msg.GetBincremental()
	for _, f := range msg.GetFriends() {
		c.OnRelationship(&RelationshipEvent{
			SteamID:      steamid.FromSteamID64(f.GetUlfriendid()),
			Relationship: FriendRelationship(f.GetEfriendrelationship()),
			Incremental:  incremental,
		})
	}
}

// handleFriendMsgIncoming processes an incoming friend chat message packet.
func (c *Client) handleFriendMsgIncoming(pkt *Packet) {
	var msg protocol.CMsgClientFriendMsgIncoming
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal FriendMsgIncoming", "err", err)
		return
	}

	if c.OnFriendMessage == nil {
		return
	}

	c.OnFriendMessage(&FriendMessage{
		Sender:             steamid.FromSteamID64(msg.GetSteamidFrom()),
		EntryType:          ChatEntryType(msg.GetChatEntryType()),
		Message:            strings.TrimRight(string(msg.GetMessage()), "\x00"),
		FromLimitedAccount: msg.GetFromLimitedAccount(),
		ServerTimestamp:    msg.GetRtime32ServerTimestamp(),
		Echo:               pkt.EMsg == EMsgClientFriendMsgEchoToSender,
	})
}

// SendMessage sends a chat message to the given Steam friend (fire-and-forget).
func (c *Client) SendMessage(ctx context.Context, target steamid.SteamID, message string) error {
	sid := target.ToSteamID64()
	entryType := int32(ChatEntryTypeChatMsg)
	body, err := proto.Marshal(&protocol.CMsgClientFriendMsg{
		Steamid:       &sid,
		ChatEntryType: &entryType,
		Message:       append([]byte(message), 0x00),
	})
	if err != nil {
		return fmt.Errorf("marshal SendMessage: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientFriendMsg, nil, body); err != nil {
		return fmt.Errorf("send FriendMsg: %w", err)
	}

	return nil
}
