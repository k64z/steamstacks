package steamclient

import (
	"context"
	"testing"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// mockConn implements Connection for unit tests.
type mockConn struct {
	writeCh chan []byte
}

func (m *mockConn) Write(_ context.Context, data []byte) error { m.writeCh <- data; return nil }
func (m *mockConn) Read(_ context.Context) ([]byte, error)     { select {} }
func (m *mockConn) Close() error                                { return nil }
func (m *mockConn) RemoteAddr() string                          { return "mock" }

func TestExpectJobIDMatches(t *testing.T) {
	c := New()
	c.done = make(chan struct{})

	ch := c.expectJobID(42)

	pkt := &Packet{
		EMsg:    EMsgServiceMethodSendToClient,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{JobidTarget: proto.Uint64(42), Eresult: proto.Int32(1)},
		Body:    []byte("ok"),
	}
	c.handlePacket(pkt)

	select {
	case got := <-ch:
		if got != pkt {
			t.Errorf("got different packet than expected")
		}
	case <-time.After(time.Second):
		t.Fatal("expectJobID did not deliver matching packet within 1s")
	}
}

func TestExpectJobIDIgnoresMismatch(t *testing.T) {
	c := New()
	c.done = make(chan struct{})

	ch := c.expectJobID(42)

	// Send a packet with a different JobidTarget — should NOT match.
	pkt := &Packet{
		EMsg:    EMsgServiceMethodSendToClient,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{JobidTarget: proto.Uint64(99)},
		Body:    []byte("wrong"),
	}
	c.handlePacket(pkt)

	select {
	case <-ch:
		t.Fatal("expectJobID matched a packet with wrong JobidTarget")
	case <-time.After(50 * time.Millisecond):
		// expected — no match
	}
}

func TestExpectJobIDOnPacketStillFires(t *testing.T) {
	var onPacketCalled bool
	c := New(WithPacketHandler(func(pkt *Packet) {
		onPacketCalled = true
	}))
	c.done = make(chan struct{})

	ch := c.expectJobID(7)

	// handlePacket should both deliver to the pending job channel
	// AND forward to OnPacket for user-level logging.
	pkt := &Packet{
		EMsg:    EMsgServiceMethodSendToClient,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{JobidTarget: proto.Uint64(7), Eresult: proto.Int32(1)},
		Body:    nil,
	}
	c.handlePacket(pkt)
	<-ch

	if !onPacketCalled {
		t.Error("OnPacket handler was not called for service method response")
	}
}

func TestCallServiceMethod(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 1)}
	c := New()
	c.conn = mc
	c.done = make(chan struct{})

	ctx := context.Background()
	respBody, _ := proto.Marshal(&protocol.CAuthentication_AccessToken_GenerateForApp_Response{
		AccessToken: proto.String("new-at"),
	})

	resultCh := make(chan struct {
		pkt *Packet
		err error
	}, 1)

	go func() {
		pkt, err := c.callServiceMethod(ctx, "Authentication.GenerateAccessTokenForApp#1", []byte{})
		resultCh <- struct {
			pkt *Packet
			err error
		}{pkt, err}
	}()

	// Wait for the sent packet.
	sentData := <-mc.writeCh

	// Decode and verify the sent packet.
	sentPkt, err := decodePacket(sentData)
	if err != nil {
		t.Fatalf("decode sent packet: %v", err)
	}
	if sentPkt.EMsg != EMsgServiceMethodCallFromClient {
		t.Errorf("sent EMsg = %v, want %v", sentPkt.EMsg, EMsgServiceMethodCallFromClient)
	}
	if sentPkt.Header.GetTargetJobName() != "Authentication.GenerateAccessTokenForApp#1" {
		t.Errorf("TargetJobName = %q, want %q", sentPkt.Header.GetTargetJobName(), "Authentication.GenerateAccessTokenForApp#1")
	}
	jobID := sentPkt.Header.GetJobidSource()
	if jobID == 0 {
		t.Fatal("JobidSource should be non-zero")
	}

	// Inject matching response via handlePacket.
	c.handlePacket(&Packet{
		EMsg:    EMsgServiceMethodSendToClient,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{JobidTarget: proto.Uint64(jobID), Eresult: proto.Int32(1)},
		Body:    respBody,
	})

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("callServiceMethod returned error: %v", r.err)
		}
		if r.pkt == nil {
			t.Fatal("callServiceMethod returned nil packet")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callServiceMethod did not return within 2s")
	}
}

func TestCallServiceMethodEresultError(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 1)}
	c := New()
	c.conn = mc
	c.done = make(chan struct{})

	ctx := context.Background()

	resultCh := make(chan error, 1)
	go func() {
		_, err := c.callServiceMethod(ctx, "SomeService.SomeMethod#1", []byte{})
		resultCh <- err
	}()

	sentData := <-mc.writeCh
	sentPkt, err := decodePacket(sentData)
	if err != nil {
		t.Fatalf("decode sent packet: %v", err)
	}
	jobID := sentPkt.Header.GetJobidSource()

	// Respond with eresult=2 (Fail).
	c.handlePacket(&Packet{
		EMsg:    EMsgServiceMethodSendToClient,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{JobidTarget: proto.Uint64(jobID), Eresult: proto.Int32(2)},
		Body:    nil,
	})

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("expected error for eresult != 1")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callServiceMethod did not return within 2s")
	}
}
