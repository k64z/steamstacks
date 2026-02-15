package steamclient

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
)

func TestTCPFramingWriteRead(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tc := &tcpConn{conn: client, addr: "test"}

	payload := []byte("hello steam")

	// Write in background
	go func() {
		err := tc.Write(context.Background(), payload)
		if err != nil {
			t.Errorf("write: %v", err)
		}
	}()

	// Read the raw frame from the server side
	var hdr [8]byte
	if _, err := server.Read(hdr[:]); err != nil {
		t.Fatalf("read header: %v", err)
	}

	gotLen := binary.LittleEndian.Uint32(hdr[0:4])
	gotMagic := binary.LittleEndian.Uint32(hdr[4:8])

	if gotLen != uint32(len(payload)) {
		t.Errorf("payload length: got %d, want %d", gotLen, len(payload))
	}
	if gotMagic != tcpMagic {
		t.Errorf("magic: got 0x%08X, want 0x%08X", gotMagic, tcpMagic)
	}

	buf := make([]byte, gotLen)
	if _, err := server.Read(buf); err != nil {
		t.Fatalf("read payload: %v", err)
	}

	if string(buf) != "hello steam" {
		t.Errorf("payload: got %q, want %q", string(buf), "hello steam")
	}
}

func TestTCPFramingReadVerifiesMagic(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tc := &tcpConn{conn: client, addr: "test"}

	// Write a frame with wrong magic
	go func() {
		hdr := make([]byte, 8)
		binary.LittleEndian.PutUint32(hdr[0:4], 4) // payload length
		binary.LittleEndian.PutUint32(hdr[4:8], 0xDEADBEEF) // wrong magic
		server.Write(hdr)
		server.Write([]byte("test"))
	}()

	_, err := tc.Read(context.Background())
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestTCPFramingRoundTrip(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	writer := &tcpConn{conn: client, addr: "test"}
	reader := &tcpConn{conn: server, addr: "test"}

	payload := []byte("round trip test data")

	go func() {
		if err := writer.Write(context.Background(), payload); err != nil {
			t.Errorf("write: %v", err)
		}
	}()

	got, err := reader.Read(context.Background())
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != string(payload) {
		t.Errorf("round-trip: got %q, want %q", string(got), string(payload))
	}
}
