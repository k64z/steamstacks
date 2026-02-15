package steamclient

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync"
)

const tcpMagic = 0x31305456 // "VT01"

// tcpConn implements Connection over raw TCP with VT01 framing.
type tcpConn struct {
	conn   net.Conn
	cipher *channelCipher
	mu     sync.Mutex // serializes writes
	addr   string
}

func dialTCP(ctx context.Context, addr string) (*tcpConn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
	}
	return &tcpConn{conn: conn, addr: addr}, nil
}

// Write sends data with VT01 framing. If encrypted, encrypts first.
// TCP frame: [payload_len : uint32 LE][magic "VT01" : uint32 LE][payload]
func (t *tcpConn) Write(ctx context.Context, data []byte) error {
	payload := data
	if t.cipher != nil {
		var err error
		payload, err = t.cipher.encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	binary.LittleEndian.PutUint32(hdr[4:8], tcpMagic)

	if _, err := t.conn.Write(hdr); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := t.conn.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// Read reads one VT01-framed message. If encrypted, decrypts.
func (t *tcpConn) Read(ctx context.Context) ([]byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(t.conn, hdr[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	payloadLen := binary.LittleEndian.Uint32(hdr[0:4])
	magic := binary.LittleEndian.Uint32(hdr[4:8])
	if magic != tcpMagic {
		return nil, fmt.Errorf("invalid magic: 0x%08X", magic)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(t.conn, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	if t.cipher != nil {
		decrypted, err := t.cipher.decrypt(payload)
		if err != nil {
			return nil, fmt.Errorf("decrypt: %w", err)
		}
		return decrypted, nil
	}

	return payload, nil
}

func (t *tcpConn) Close() error {
	return t.conn.Close()
}

func (t *tcpConn) RemoteAddr() string {
	return t.addr
}

// performEncryptionHandshake executes the TCP channel encryption handshake.
//
// Encryption handshake messages use MsgHdr (20 bytes), NOT ExtendedClientMsgHdr (36 bytes):
//
//	[EMsg : uint32 LE][target_job_id : uint64 LE][source_job_id : uint64 LE]
//
// 1. Receive ChannelEncryptRequest (1303) — protocol_version + universe + optional 16-byte challenge
// 2. Generate 32-byte random session key
// 3. RSA-encrypt (sessionKey + challenge) with Steam's public key
// 4. Send ChannelEncryptResponse (1304) — protocol_version + key_size + encrypted blob + CRC32
// 5. Receive ChannelEncryptResult (1305) — verify eresult == 1
func (t *tcpConn) performEncryptionHandshake(ctx context.Context) error {
	const msgHdrLen = 20 // EMsg(4) + TargetJobID(8) + SourceJobID(8)

	data, err := t.Read(ctx)
	if err != nil {
		return fmt.Errorf("read encrypt request: %w", err)
	}

	if len(data) < msgHdrLen+8 {
		return fmt.Errorf("encrypt request too short: %d bytes", len(data))
	}

	emsg := EMsg(binary.LittleEndian.Uint32(data[0:4]))
	if emsg != EMsgChannelEncryptRequest {
		return fmt.Errorf("expected ChannelEncryptRequest, got %s", emsg)
	}

	body := data[msgHdrLen:]

	var challenge []byte
	if len(body) >= 24 {
		challenge = body[8:24]
	}

	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		return fmt.Errorf("generate session key: %w", err)
	}

	encryptedBlob, err := rsaEncryptSessionKey(sessionKey, challenge)
	if err != nil {
		return fmt.Errorf("rsa encrypt: %w", err)
	}

	keyCRC := crc32.ChecksumIEEE(encryptedBlob)

	buf := make([]byte, 0, msgHdrLen+8+len(encryptedBlob)+8)
	resp := binary.LittleEndian.AppendUint32(buf, uint32(EMsgChannelEncryptResponse))
	resp = binary.LittleEndian.AppendUint64(resp, 0xFFFFFFFFFFFFFFFF) // target job id
	resp = binary.LittleEndian.AppendUint64(resp, 0xFFFFFFFFFFFFFFFF) // source job id
	resp = binary.LittleEndian.AppendUint32(resp, 1)                  // protocol version
	resp = binary.LittleEndian.AppendUint32(resp, 128)                // key size
	resp = append(resp, encryptedBlob...)
	resp = binary.LittleEndian.AppendUint32(resp, keyCRC)
	resp = binary.LittleEndian.AppendUint32(resp, 0) // trailing zero

	if err := t.Write(ctx, resp); err != nil {
		return fmt.Errorf("send encrypt response: %w", err)
	}

	resultData, err := t.Read(ctx)
	if err != nil {
		return fmt.Errorf("read encrypt result: %w", err)
	}

	if len(resultData) < msgHdrLen+4 {
		return fmt.Errorf("encrypt result too short: %d bytes", len(resultData))
	}

	resultEmsg := EMsg(binary.LittleEndian.Uint32(resultData[0:4]))
	if resultEmsg != EMsgChannelEncryptResult {
		return fmt.Errorf("expected ChannelEncryptResult, got %s", resultEmsg)
	}

	eresult := binary.LittleEndian.Uint32(resultData[msgHdrLen : msgHdrLen+4])
	if eresult != 1 {
		return fmt.Errorf("encryption handshake failed: eresult=%d", eresult)
	}

	// Use HMAC mode only when a challenge was present
	t.cipher, err = newChannelCipher(sessionKey, challenge != nil)
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}

	return nil
}
