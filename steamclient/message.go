package steamclient

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// Packet represents a decoded Steam CM message.
type Packet struct {
	EMsg    EMsg
	IsProto bool
	Header  *protocol.CMsgProtoBufHeader
	Body    []byte // raw serialized protobuf body
}

// encodePacket serializes a Packet to the CM wire format.
//
// Protobuf wire format:
//
//	[EMsg | 0x80000000 : uint32 LE][header_len : uint32 LE][CMsgProtoBufHeader][body]
//
// Non-protobuf wire format (encryption handshake only):
//
//	[EMsg : uint32 LE][header_size=36 : byte][header_version=2 : uint16 LE]
//	[target_job_id : uint64 LE][source_job_id : uint64 LE]
//	[canary=0xEF : byte][steam_id : uint64 LE][session_id : int32 LE][body]
func encodePacket(p *Packet) ([]byte, error) {
	if p.IsProto {
		return encodeProtoPacket(p)
	}
	return encodeNonProtoPacket(p)
}

func encodeProtoPacket(p *Packet) ([]byte, error) {
	hdr := p.Header
	if hdr == nil {
		hdr = &protocol.CMsgProtoBufHeader{}
	}

	hdrBytes, err := proto.Marshal(hdr)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}

	buf := make([]byte, 4+4+len(hdrBytes)+len(p.Body))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(p.EMsg)|ProtoMask)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(hdrBytes)))
	copy(buf[8:], hdrBytes)
	copy(buf[8+len(hdrBytes):], p.Body)
	return buf, nil
}

func encodeNonProtoPacket(p *Packet) ([]byte, error) {
	// headerSize includes the 4-byte EMsg prefix
	const headerSize = 36

	var steamID uint64
	var sessionID int32
	if p.Header != nil {
		steamID = p.Header.GetSteamid()
		sessionID = p.Header.GetClientSessionid()
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(p.EMsg))
	buf.WriteByte(headerSize)
	binary.Write(buf, binary.LittleEndian, uint16(2)) // header version
	binary.Write(buf, binary.LittleEndian, uint64(0xFFFFFFFFFFFFFFFF)) // target job id
	binary.Write(buf, binary.LittleEndian, uint64(0xFFFFFFFFFFFFFFFF)) // source job id
	buf.WriteByte(0xEF) // canary
	binary.Write(buf, binary.LittleEndian, steamID)
	binary.Write(buf, binary.LittleEndian, sessionID)
	buf.Write(p.Body)
	return buf.Bytes(), nil
}

// decodePacket deserializes raw CM wire bytes into a Packet.
func decodePacket(data []byte) (*Packet, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	rawEMsg := binary.LittleEndian.Uint32(data[0:4])
	isProto := rawEMsg&ProtoMask != 0
	emsg := EMsg(rawEMsg &^ ProtoMask)

	if isProto {
		return decodeProtoPacket(emsg, data)
	}
	return decodeNonProtoPacket(emsg, data)
}

func decodeProtoPacket(emsg EMsg, data []byte) (*Packet, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("proto packet too short for header length: %d bytes", len(data))
	}

	hdrLen := binary.LittleEndian.Uint32(data[4:8])
	if uint32(len(data)) < 8+hdrLen {
		return nil, fmt.Errorf("proto packet truncated: need %d header bytes, have %d", hdrLen, len(data)-8)
	}

	hdr := &protocol.CMsgProtoBufHeader{}
	if err := proto.Unmarshal(data[8:8+hdrLen], hdr); err != nil {
		return nil, fmt.Errorf("unmarshal header: %w", err)
	}

	return &Packet{
		EMsg:    emsg,
		IsProto: true,
		Header:  hdr,
		Body:    data[8+hdrLen:],
	}, nil
}

func decodeNonProtoPacket(emsg EMsg, data []byte) (*Packet, error) {
	const minLen = 36
	if len(data) < minLen {
		return nil, fmt.Errorf("non-proto packet too short: %d bytes", len(data))
	}

	steamID := binary.LittleEndian.Uint64(data[24:32])
	sessionID := int32(binary.LittleEndian.Uint32(data[32:36]))

	hdr := &protocol.CMsgProtoBufHeader{
		Steamid:         &steamID,
		ClientSessionid: &sessionID,
	}

	return &Packet{
		EMsg:    emsg,
		IsProto: false,
		Header:  hdr,
		Body:    data[36:],
	}, nil
}

// decodeMulti handles EMsgMulti: optionally gzip-decompresses the body,
// then splits the concatenated [uint32 LE size][message] entries.
func decodeMulti(body []byte, sizeUnzipped uint32) ([]*Packet, error) {
	var reader io.Reader = bytes.NewReader(body)

	if sizeUnzipped > 0 {
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip open: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	var packets []*Packet
	var sizeBuf [4]byte

	for {
		_, err := io.ReadFull(reader, sizeBuf[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read sub-message size: %w", err)
		}

		subSize := binary.LittleEndian.Uint32(sizeBuf[:])
		subData := make([]byte, subSize)
		if _, err := io.ReadFull(reader, subData); err != nil {
			return nil, fmt.Errorf("read sub-message body: %w", err)
		}

		pkt, err := decodePacket(subData)
		if err != nil {
			return nil, fmt.Errorf("decode sub-message: %w", err)
		}
		packets = append(packets, pkt)
	}

	return packets, nil
}
