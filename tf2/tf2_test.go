package tf2

import (
	"context"
	"testing"
	"time"

	"github.com/k64z/steamstacks/steamclient"
)

func TestWelcomeStopsHelloAndSetsConnected(t *testing.T) {
	// Use a fake ticker that never fires so helloLoop stays quiet.
	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	var connected bool
	tc := New(cm, WithConnectedHandler(func(e *WelcomeEvent) {
		connected = true
	}))

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Drain the initial hello send.
	<-mc.writeCh

	// Simulate welcome from GC.
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID:   AppID,
		MsgType: MsgClientWelcome,
		IsProto: true,
		Body:    nil,
	})

	if !tc.IsConnected() {
		t.Error("expected IsConnected() == true after welcome")
	}
	if !connected {
		t.Error("OnConnected was not called")
	}
}

func TestGoodbyeSetsDisconnected(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	var disconnected bool
	tc := New(cm, WithDisconnectedHandler(func(e *GoodbyeEvent) {
		disconnected = true
	}))

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-mc.writeCh

	// Establish connection first.
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientWelcome, IsProto: true,
	})

	if !tc.IsConnected() {
		t.Fatal("not connected after welcome")
	}

	// Simulate goodbye.
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientGoodbye, IsProto: true,
	})

	if tc.IsConnected() {
		t.Error("expected IsConnected() == false after goodbye")
	}
	if !disconnected {
		t.Error("OnDisconnected was not called")
	}
}

func TestDisconnectStopsHelloLoop(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	tc := New(cm)

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-mc.writeCh

	tc.Disconnect()

	if tc.IsConnected() {
		t.Error("expected IsConnected() == false after Disconnect()")
	}
}

func TestNonTF2MessagePassthrough(t *testing.T) {
	cm := steamclient.New()

	var prevCalled bool
	cm.OnGCMessage = func(msg *steamclient.GCMessage) {
		prevCalled = true
	}

	_ = New(cm) // installs TF2 filter

	// Send a non-TF2 GC message.
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: 730, MsgType: 9999, IsProto: true, Body: []byte{0x01},
	})

	if !prevCalled {
		t.Error("previous OnGCMessage handler was not called for non-TF2 message")
	}
}

func TestUnhandledTF2MessageForwarded(t *testing.T) {
	cm := steamclient.New()

	var got *steamclient.GCMessage
	tc := New(cm, WithGCMessageHandler(func(msg *steamclient.GCMessage) {
		got = msg
	}))
	_ = tc

	// Send a TF2 GC message with an unknown type.
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: 9999, IsProto: true, Body: []byte{0xAB},
	})

	if got == nil {
		t.Fatal("OnGCMessage was not called for unhandled TF2 message")
	}
	if got.MsgType != 9999 {
		t.Errorf("MsgType = %d, want 9999", got.MsgType)
	}
}

func TestHelloLoopSendsMessages(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	fakeCh := make(chan time.Time, 5)
	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: fakeCh} }
	defer func() { newTicker = origTicker }()

	tc := New(cm)

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Drain the initial hello.
	<-mc.writeCh

	// Fire two ticks.
	fakeCh <- time.Now()
	fakeCh <- time.Now()

	// Allow goroutine to process.
	time.Sleep(50 * time.Millisecond)

	count := len(mc.writeCh)
	if count < 2 {
		t.Errorf("hello loop sent %d messages after 2 ticks, want >= 2", count)
	}

	tc.Disconnect()
}

func TestConnectWhileConnecting(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	tc := New(cm)

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	<-mc.writeCh

	err := tc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error on second Connect()")
	}

	tc.Disconnect()
}

// --- test helpers ---

type mockConn struct {
	writeCh chan []byte
}

func (m *mockConn) Write(_ context.Context, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.writeCh <- cp
	return nil
}
func (m *mockConn) Read(_ context.Context) ([]byte, error) { select {} }
func (m *mockConn) Close() error                           { return nil }
func (m *mockConn) RemoteAddr() string                     { return "mock" }

type fakeTicker struct {
	ch chan time.Time
}

func (f *fakeTicker) C() <-chan time.Time { return f.ch }
func (f *fakeTicker) Stop()              {}
