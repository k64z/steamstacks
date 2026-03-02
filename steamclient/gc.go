package steamclient

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// GCMessage represents a decoded Game Coordinator message.
type GCMessage struct {
	AppID   uint32
	MsgType uint32 // GC msg type, proto mask stripped
	IsProto bool
	Body    []byte // protobuf or binary body (no GC header)
}

// WithGCMessageHandler sets a callback for Game Coordinator messages.
func WithGCMessageHandler(fn func(*GCMessage)) Option {
	return func(c *config) { c.onGCMessage = fn }
}

// SendGCMessage sends a message to a Game Coordinator.
func (c *Client) SendGCMessage(ctx context.Context, appID, msgType uint32, isProto bool, body []byte) error {
	payload, err := encodeGCPayload(&GCMessage{
		AppID:   appID,
		MsgType: msgType,
		IsProto: isProto,
		Body:    body,
	})
	if err != nil {
		return fmt.Errorf("encode GC payload: %w", err)
	}

	gcBody, err := proto.Marshal(&protocol.CMsgGCClient{
		Appid:   &appID,
		Msgtype: proto.Uint32(msgType | ProtoMask*boolToUint32(isProto)),
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("marshal CMsgGCClient: %w", err)
	}

	hdr := &protocol.CMsgProtoBufHeader{
		RoutingAppid: &appID,
	}
	return c.sendPacket(ctx, EMsgClientToGC, hdr, gcBody)
}

func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// handleGCMessage processes an EMsgClientFromGC packet.
func (c *Client) handleGCMessage(pkt *Packet) {
	if c.OnGCMessage == nil {
		return
	}

	var msg protocol.CMsgGCClient
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal CMsgGCClient", "err", err)
		return
	}

	gcMsg, err := decodeGCPayload(msg.GetAppid(), msg.GetMsgtype(), msg.GetPayload())
	if err != nil {
		c.logger.Error("decode GC payload", "err", err, "appid", msg.GetAppid())
		return
	}

	c.OnGCMessage(gcMsg)
}

// encodeGCPayload encodes a GC message body with the appropriate GC header.
//
// Protobuf format:
//
//	[msgType|0x80000000 : u32 LE][hdrLen : u32 LE][CMsgProtoBufHeader][body]
//
// Binary format:
//
//	[version=1 : u16 LE][targetJob : u64 LE][sourceJob : u64 LE][body]
func encodeGCPayload(msg *GCMessage) ([]byte, error) {
	if msg.IsProto {
		return encodeGCProtoPayload(msg)
	}
	return encodeGCBinaryPayload(msg), nil
}

func encodeGCProtoPayload(msg *GCMessage) ([]byte, error) {
	hdr := &protocol.CMsgProtoBufHeader{}
	hdrBytes, err := proto.Marshal(hdr)
	if err != nil {
		return nil, fmt.Errorf("marshal GC proto header: %w", err)
	}

	buf := make([]byte, 4+4+len(hdrBytes)+len(msg.Body))
	binary.LittleEndian.PutUint32(buf[0:4], msg.MsgType|ProtoMask)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(hdrBytes)))
	copy(buf[8:], hdrBytes)
	copy(buf[8+len(hdrBytes):], msg.Body)
	return buf, nil
}

func encodeGCBinaryPayload(msg *GCMessage) []byte {
	// header: version(2) + targetJob(8) + sourceJob(8) = 18 bytes
	// Note: unlike proto payloads, binary payloads do NOT include the msgType
	// in the inner header. The msgType is carried only in CMsgGCClient.Msgtype.
	buf := make([]byte, 18+len(msg.Body))
	binary.LittleEndian.PutUint16(buf[0:2], 1) // version
	binary.LittleEndian.PutUint64(buf[2:10], 0xFFFFFFFFFFFFFFFF)  // targetJob
	binary.LittleEndian.PutUint64(buf[10:18], 0xFFFFFFFFFFFFFFFF) // sourceJob
	copy(buf[18:], msg.Body)
	return buf
}

// decodeGCPayload parses a GC payload, strips the GC header, and returns the body.
// The rawMsgType from CMsgGCClient.Msgtype determines whether the inner payload
// uses proto or binary framing: proto payloads include the msgType in the inner
// header, while binary payloads do not.
func decodeGCPayload(appID, rawMsgType uint32, payload []byte) (*GCMessage, error) {
	isProto := rawMsgType&ProtoMask != 0
	msgType := rawMsgType &^ ProtoMask

	if isProto {
		return decodeGCProtoPayload(appID, msgType, payload)
	}
	return decodeGCBinaryPayload(appID, msgType, payload)
}

func decodeGCProtoPayload(appID, msgType uint32, payload []byte) (*GCMessage, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("GC proto payload too short for header length: %d bytes", len(payload))
	}

	hdrLen := binary.LittleEndian.Uint32(payload[4:8])
	bodyOffset := 8 + hdrLen
	if uint32(len(payload)) < bodyOffset {
		return nil, fmt.Errorf("GC proto payload truncated: need %d bytes, have %d", bodyOffset, len(payload))
	}

	return &GCMessage{
		AppID:   appID,
		MsgType: msgType,
		IsProto: true,
		Body:    payload[bodyOffset:],
	}, nil
}

func decodeGCBinaryPayload(appID, msgType uint32, payload []byte) (*GCMessage, error) {
	// binary header: version(2) + targetJob(8) + sourceJob(8) = 18 bytes
	const hdrSize = 18
	if len(payload) < hdrSize {
		return nil, fmt.Errorf("GC binary payload too short: %d bytes", len(payload))
	}

	return &GCMessage{
		AppID:   appID,
		MsgType: msgType,
		IsProto: false,
		Body:    payload[hdrSize:],
	}, nil
}
