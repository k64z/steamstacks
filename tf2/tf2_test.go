package tf2

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamclient"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
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
	var gotEvent *WelcomeEvent
	tc := New(cm, WithConnectedHandler(func(e *WelcomeEvent) {
		connected = true
		gotEvent = e
	}))

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Drain the initial hello send.
	<-mc.writeCh

	// Simulate welcome from GC with version + txn_country_code.
	var welcomeBody []byte
	welcomeBody = protowire.AppendTag(welcomeBody, 1, protowire.VarintType)
	welcomeBody = protowire.AppendVarint(welcomeBody, 1500)
	welcomeBody = protowire.AppendTag(welcomeBody, 3, protowire.BytesType)
	welcomeBody = protowire.AppendString(welcomeBody, "US")

	cm.OnGCMessage(&steamclient.GCMessage{
		AppID:   AppID,
		MsgType: MsgClientWelcome,
		IsProto: true,
		Body:    welcomeBody,
	})

	if !tc.IsConnected() {
		t.Error("expected IsConnected() == true after welcome")
	}
	if !connected {
		t.Error("OnConnected was not called")
	}
	if gotEvent == nil {
		t.Fatal("WelcomeEvent is nil")
	}
	if gotEvent.Version != 1500 {
		t.Errorf("WelcomeEvent.Version = %d, want 1500", gotEvent.Version)
	}
	if gotEvent.TxnCountryCode != "US" {
		t.Errorf("WelcomeEvent.TxnCountryCode = %q, want %q", gotEvent.TxnCountryCode, "US")
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

func TestUseItem(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	tc := New(cm)

	const itemID uint64 = 12345
	if err := tc.UseItem(context.Background(), itemID); err != nil {
		t.Fatalf("UseItem: %v", err)
	}

	raw := <-mc.writeCh

	// Decode outer CM packet: [uint32 EMsg|ProtoMask] [uint32 hdrLen] [header] [body]
	if len(raw) < 8 {
		t.Fatalf("packet too short: %d bytes", len(raw))
	}
	emsg := binary.LittleEndian.Uint32(raw[:4])
	if emsg != uint32(steamclient.EMsgClientToGC)|steamclient.ProtoMask {
		t.Fatalf("EMsg = %#x, want EMsgClientToGC|ProtoMask (%#x)", emsg, uint32(steamclient.EMsgClientToGC)|steamclient.ProtoMask)
	}
	hdrLen := binary.LittleEndian.Uint32(raw[4:8])
	cmBody := raw[8+hdrLen:]

	// Unmarshal CMsgGCClient from the CM body.
	var gcClient protocol.CMsgGCClient
	if err := proto.Unmarshal(cmBody, &gcClient); err != nil {
		t.Fatalf("unmarshal CMsgGCClient: %v", err)
	}
	if gcClient.GetAppid() != AppID {
		t.Errorf("AppID = %d, want %d", gcClient.GetAppid(), AppID)
	}
	wantMsgType := uint32(MsgUseItemRequest) | steamclient.ProtoMask
	if gcClient.GetMsgtype() != wantMsgType {
		t.Errorf("MsgType = %#x, want %#x", gcClient.GetMsgtype(), wantMsgType)
	}

	// Decode inner GC payload: [uint32 msgType|ProtoMask] [uint32 gcHdrLen] [gcHdr] [body]
	payload := gcClient.GetPayload()
	if len(payload) < 8 {
		t.Fatalf("GC payload too short: %d bytes", len(payload))
	}
	gcMsgType := binary.LittleEndian.Uint32(payload[:4])
	if gcMsgType != uint32(MsgUseItemRequest)|steamclient.ProtoMask {
		t.Errorf("GC inner MsgType = %#x, want %#x", gcMsgType, uint32(MsgUseItemRequest)|steamclient.ProtoMask)
	}
	gcHdrLen := binary.LittleEndian.Uint32(payload[4:8])
	gcBody := payload[8+gcHdrLen:]

	// Verify protowire-encoded item_id (field 1, varint) = 12345.
	fieldNum, wireType, n := protowire.ConsumeTag(gcBody)
	if n < 0 {
		t.Fatalf("protowire.ConsumeTag failed")
	}
	if fieldNum != 1 || wireType != protowire.VarintType {
		t.Fatalf("tag = field %d wire %d, want field 1 varint", fieldNum, wireType)
	}
	val, vn := protowire.ConsumeVarint(gcBody[n:])
	if vn < 0 {
		t.Fatalf("protowire.ConsumeVarint failed")
	}
	if val != itemID {
		t.Errorf("item_id = %d, want %d", val, itemID)
	}
}

func TestCraft(t *testing.T) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)

	tc := New(cm)

	items := []uint64{111, 222}
	var recipe int16 = -2
	if err := tc.Craft(context.Background(), items, recipe); err != nil {
		t.Fatalf("Craft: %v", err)
	}

	raw := <-mc.writeCh

	// Decode outer CM packet.
	if len(raw) < 8 {
		t.Fatalf("packet too short: %d bytes", len(raw))
	}
	emsg := binary.LittleEndian.Uint32(raw[:4])
	if emsg != uint32(steamclient.EMsgClientToGC)|steamclient.ProtoMask {
		t.Fatalf("EMsg = %#x, want EMsgClientToGC|ProtoMask", emsg)
	}
	hdrLen := binary.LittleEndian.Uint32(raw[4:8])
	cmBody := raw[8+hdrLen:]

	// Unmarshal CMsgGCClient.
	var gcClient protocol.CMsgGCClient
	if err := proto.Unmarshal(cmBody, &gcClient); err != nil {
		t.Fatalf("unmarshal CMsgGCClient: %v", err)
	}
	if gcClient.GetAppid() != AppID {
		t.Errorf("AppID = %d, want %d", gcClient.GetAppid(), AppID)
	}
	// Non-proto: msgtype should NOT have ProtoMask set.
	if gcClient.GetMsgtype() != MsgCraft {
		t.Errorf("MsgType = %#x, want %#x", gcClient.GetMsgtype(), uint32(MsgCraft))
	}

	// Decode inner GC payload (binary header: version(2) + targetJob(8) + sourceJob(8) = 18 bytes).
	payload := gcClient.GetPayload()
	if len(payload) < 18 {
		t.Fatalf("GC payload too short: %d bytes", len(payload))
	}
	gcVersion := binary.LittleEndian.Uint16(payload[:2])
	if gcVersion != 1 {
		t.Errorf("GC binary header version = %d, want 1", gcVersion)
	}
	gcBody := payload[18:]

	// Verify binary layout: int16le recipe, int16le count, uint64le[] items.
	if len(gcBody) < 4+8*len(items) {
		t.Fatalf("craft body too short: %d bytes", len(gcBody))
	}
	gotRecipe := int16(binary.LittleEndian.Uint16(gcBody[0:2]))
	if gotRecipe != recipe {
		t.Errorf("recipe = %d, want %d", gotRecipe, recipe)
	}
	gotCount := binary.LittleEndian.Uint16(gcBody[2:4])
	if gotCount != uint16(len(items)) {
		t.Errorf("item_count = %d, want %d", gotCount, len(items))
	}
	for i, want := range items {
		got := binary.LittleEndian.Uint64(gcBody[4+8*i:])
		if got != want {
			t.Errorf("item[%d] = %d, want %d", i, got, want)
		}
	}
}

func TestCraftResponse(t *testing.T) {
	cm := steamclient.New()

	var gotEvent *CraftEvent
	tc := New(cm, WithCraftCompletedHandler(func(e *CraftEvent) {
		gotEvent = e
	}))
	_ = tc

	// Build a raw binary CraftResponse body:
	// int16le blueprint, uint32le unknown, uint16le id_count, uint64le[] item_ids
	var body []byte
	body = binary.LittleEndian.AppendUint16(body, uint16(int16(5))) // recipe 5
	body = binary.LittleEndian.AppendUint32(body, 0)               // unknown
	body = binary.LittleEndian.AppendUint16(body, 2)               // 2 items
	body = binary.LittleEndian.AppendUint64(body, 9001)
	body = binary.LittleEndian.AppendUint64(body, 9002)

	cm.OnGCMessage(&steamclient.GCMessage{
		AppID:   AppID,
		MsgType: MsgCraftResponse,
		IsProto: false,
		Body:    body,
	})

	if gotEvent == nil {
		t.Fatal("OnCraftCompleted was not called")
	}
	if gotEvent.Recipe != 5 {
		t.Errorf("Recipe = %d, want 5", gotEvent.Recipe)
	}
	if len(gotEvent.ItemIDs) != 2 {
		t.Fatalf("ItemIDs len = %d, want 2", len(gotEvent.ItemIDs))
	}
	if gotEvent.ItemIDs[0] != 9001 {
		t.Errorf("ItemIDs[0] = %d, want 9001", gotEvent.ItemIDs[0])
	}
	if gotEvent.ItemIDs[1] != 9002 {
		t.Errorf("ItemIDs[1] = %d, want 9002", gotEvent.ItemIDs[1])
	}
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
