package steamclient

import (
	"bytes"
	"context"
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// makeGCPacket builds an EMsgClientFromGC packet wrapping a GC payload.
func makeGCPacket(t *testing.T, appID, msgType uint32, isProto bool, body []byte) *Packet {
	t.Helper()

	payload, err := encodeGCPayload(&GCMessage{
		AppID:   appID,
		MsgType: msgType,
		IsProto: isProto,
		Body:    body,
	})
	if err != nil {
		t.Fatalf("encodeGCPayload: %v", err)
	}

	gcBody, err := proto.Marshal(&protocol.CMsgGCClient{
		Appid:   &appID,
		Msgtype: proto.Uint32(msgType | ProtoMask*boolToUint32(isProto)),
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("marshal CMsgGCClient: %v", err)
	}

	return &Packet{
		EMsg:    EMsgClientFromGC,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{},
		Body:    gcBody,
	}
}

func TestGCEncodeDecodeProtoPayload(t *testing.T) {
	original := &GCMessage{
		AppID:   440,
		MsgType: 4004,
		IsProto: true,
		Body:    []byte{0x08, 0x01}, // some proto bytes
	}

	payload, err := encodeGCPayload(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeGCPayload(original.AppID, original.MsgType|ProtoMask, payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.AppID != original.AppID {
		t.Errorf("AppID = %d, want %d", decoded.AppID, original.AppID)
	}
	if decoded.MsgType != original.MsgType {
		t.Errorf("MsgType = %d, want %d", decoded.MsgType, original.MsgType)
	}
	if !decoded.IsProto {
		t.Error("expected IsProto=true")
	}
	if !bytes.Equal(decoded.Body, original.Body) {
		t.Errorf("Body = %x, want %x", decoded.Body, original.Body)
	}
}

func TestGCEncodeDecodeBinaryPayload(t *testing.T) {
	original := &GCMessage{
		AppID:   440,
		MsgType: 1001,
		IsProto: false,
		Body:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	payload, err := encodeGCPayload(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeGCPayload(original.AppID, original.MsgType, payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.AppID != original.AppID {
		t.Errorf("AppID = %d, want %d", decoded.AppID, original.AppID)
	}
	if decoded.MsgType != original.MsgType {
		t.Errorf("MsgType = %d, want %d", decoded.MsgType, original.MsgType)
	}
	if decoded.IsProto {
		t.Error("expected IsProto=false")
	}
	if !bytes.Equal(decoded.Body, original.Body) {
		t.Errorf("Body = %x, want %x", decoded.Body, original.Body)
	}
}

func TestGCDecodePayloadTooShort(t *testing.T) {
	_, err := decodeGCPayload(440, 4004, []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error for payload under 4 bytes")
	}
}

func TestGCHandleGCMessage(t *testing.T) {
	body := []byte{0x08, 0x42}
	pkt := makeGCPacket(t, 440, 4004, true, body)

	var got *GCMessage
	c := New(WithGCMessageHandler(func(msg *GCMessage) {
		got = msg
	}))

	c.handleGCMessage(pkt)

	if got == nil {
		t.Fatal("OnGCMessage was not called")
	}
	if got.AppID != 440 {
		t.Errorf("AppID = %d, want 440", got.AppID)
	}
	if got.MsgType != 4004 {
		t.Errorf("MsgType = %d, want 4004", got.MsgType)
	}
	if !got.IsProto {
		t.Error("expected IsProto=true")
	}
	if !bytes.Equal(got.Body, body) {
		t.Errorf("Body = %x, want %x", got.Body, body)
	}
}

func TestGCHandleGCMessageNilHandler(t *testing.T) {
	pkt := makeGCPacket(t, 440, 4004, true, []byte{0x08, 0x01})

	c := New() // no handler set
	// Should not panic.
	c.handleGCMessage(pkt)
}

func TestGCHandleGCMessageOnPacketPassthrough(t *testing.T) {
	pkt := makeGCPacket(t, 440, 4004, true, []byte{0x08, 0x01})

	var gcCalled, pktCalled bool
	c := New(
		WithGCMessageHandler(func(msg *GCMessage) { gcCalled = true }),
		WithPacketHandler(func(p *Packet) { pktCalled = true }),
	)

	// handlePacket dispatches to both handleGCMessage and OnPacket.
	c.handlePacket(pkt)

	if !gcCalled {
		t.Error("OnGCMessage was not called")
	}
	if !pktCalled {
		t.Error("OnPacket was not called")
	}
}

func TestGCSendGCMessage(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 1)}
	c := New()
	c.conn = mc
	c.done = make(chan struct{})

	body := []byte{0x08, 0x01}
	if err := c.SendGCMessage(context.Background(), 440, 4006, true, body); err != nil {
		t.Fatalf("SendGCMessage: %v", err)
	}

	sentData := <-mc.writeCh

	// Decode the outer CM packet.
	sentPkt, err := decodePacket(sentData)
	if err != nil {
		t.Fatalf("decode sent packet: %v", err)
	}
	if sentPkt.EMsg != EMsgClientToGC {
		t.Errorf("EMsg = %s, want ClientToGC", sentPkt.EMsg)
	}
	if sentPkt.Header.GetRoutingAppid() != 440 {
		t.Errorf("RoutingAppid = %d, want 440", sentPkt.Header.GetRoutingAppid())
	}

	// Unmarshal CMsgGCClient.
	var gcClient protocol.CMsgGCClient
	if err := proto.Unmarshal(sentPkt.Body, &gcClient); err != nil {
		t.Fatalf("unmarshal CMsgGCClient: %v", err)
	}
	if gcClient.GetAppid() != 440 {
		t.Errorf("CMsgGCClient.Appid = %d, want 440", gcClient.GetAppid())
	}
	wantMsgType := uint32(4006) | ProtoMask
	if gcClient.GetMsgtype() != wantMsgType {
		t.Errorf("CMsgGCClient.Msgtype = %d, want %d", gcClient.GetMsgtype(), wantMsgType)
	}

	// Decode the inner GC payload and verify the body.
	gcMsg, err := decodeGCPayload(gcClient.GetAppid(), gcClient.GetMsgtype(), gcClient.GetPayload())
	if err != nil {
		t.Fatalf("decode inner GC payload: %v", err)
	}
	if gcMsg.MsgType != 4006 {
		t.Errorf("inner MsgType = %d, want 4006", gcMsg.MsgType)
	}
	if !gcMsg.IsProto {
		t.Error("inner IsProto should be true")
	}
	if !bytes.Equal(gcMsg.Body, body) {
		t.Errorf("inner Body = %x, want %x", gcMsg.Body, body)
	}
}
