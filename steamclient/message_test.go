package steamclient

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

func TestEncodeDecodeProtoPacket(t *testing.T) {
	hdr := &protocol.CMsgProtoBufHeader{
		Steamid:         proto.Uint64(76561198012345678),
		ClientSessionid: proto.Int32(42),
	}

	body, err := proto.Marshal(&protocol.CMsgClientHeartBeat{})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	original := &Packet{
		EMsg:    EMsgClientHeartBeat,
		IsProto: true,
		Header:  hdr,
		Body:    body,
	}

	encoded, err := encodePacket(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Verify ProtoMask is set
	rawEMsg := binary.LittleEndian.Uint32(encoded[0:4])
	if rawEMsg&ProtoMask == 0 {
		t.Error("ProtoMask not set in encoded packet")
	}
	if EMsg(rawEMsg&^ProtoMask) != EMsgClientHeartBeat {
		t.Errorf("EMsg mismatch: got %d, want %d", rawEMsg&^ProtoMask, EMsgClientHeartBeat)
	}

	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.EMsg != original.EMsg {
		t.Errorf("EMsg: got %s, want %s", decoded.EMsg, original.EMsg)
	}
	if !decoded.IsProto {
		t.Error("expected IsProto=true")
	}
	if decoded.Header.GetSteamid() != 76561198012345678 {
		t.Errorf("steamid: got %d, want 76561198012345678", decoded.Header.GetSteamid())
	}
	if decoded.Header.GetClientSessionid() != 42 {
		t.Errorf("session_id: got %d, want 42", decoded.Header.GetClientSessionid())
	}
}

func TestEncodeDecodeNonProtoPacket(t *testing.T) {
	original := &Packet{
		EMsg:    EMsgChannelEncryptRequest,
		IsProto: false,
		Body:    []byte{0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00},
	}

	encoded, err := encodePacket(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Non-proto should NOT have ProtoMask
	rawEMsg := binary.LittleEndian.Uint32(encoded[0:4])
	if rawEMsg&ProtoMask != 0 {
		t.Error("ProtoMask unexpectedly set for non-proto packet")
	}

	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.EMsg != EMsgChannelEncryptRequest {
		t.Errorf("EMsg: got %s, want %s", decoded.EMsg, EMsgChannelEncryptRequest)
	}
	if decoded.IsProto {
		t.Error("expected IsProto=false")
	}
	if !bytes.Equal(decoded.Body, original.Body) {
		t.Error("body mismatch")
	}
}

func TestDecodeMultiUncompressed(t *testing.T) {
	// Build two sub-messages
	sub1 := buildProtoPacket(t, EMsgClientHeartBeat, nil)
	sub2 := buildProtoPacket(t, EMsgClientHeartBeat, nil)

	var payload bytes.Buffer
	writeSub(&payload, sub1)
	writeSub(&payload, sub2)

	packets, err := decodeMulti(payload.Bytes(), 0)
	if err != nil {
		t.Fatalf("decodeMulti: %v", err)
	}

	if len(packets) != 2 {
		t.Fatalf("expected 2 packets, got %d", len(packets))
	}

	for i, pkt := range packets {
		if pkt.EMsg != EMsgClientHeartBeat {
			t.Errorf("packet %d: EMsg=%s, want ClientHeartBeat", i, pkt.EMsg)
		}
	}
}

func TestDecodeMultiCompressed(t *testing.T) {
	sub1 := buildProtoPacket(t, EMsgClientHeartBeat, nil)

	var payload bytes.Buffer
	writeSub(&payload, sub1)

	// Gzip compress the payload
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	gz.Write(payload.Bytes())
	gz.Close()

	packets, err := decodeMulti(compressed.Bytes(), uint32(payload.Len()))
	if err != nil {
		t.Fatalf("decodeMulti compressed: %v", err)
	}

	if len(packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(packets))
	}
}

func buildProtoPacket(t *testing.T, emsg EMsg, hdr *protocol.CMsgProtoBufHeader) []byte {
	t.Helper()
	pkt := &Packet{
		EMsg:    emsg,
		IsProto: true,
		Header:  hdr,
		Body:    nil,
	}
	data, err := encodePacket(pkt)
	if err != nil {
		t.Fatalf("buildProtoPacket: %v", err)
	}
	return data
}

func writeSub(buf *bytes.Buffer, data []byte) {
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(data)))
	buf.Write(size[:])
	buf.Write(data)
}
