package tf2

import (
	"context"
	"testing"
	"time"

	"github.com/k64z/steamstacks/steamclient"
	"google.golang.org/protobuf/encoding/protowire"
)

// --- helpers to build protowire-encoded SO messages ---

func buildItemBytes(id uint64, defIndex, inventory uint32) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, id)
	b = protowire.AppendTag(b, 4, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(defIndex))
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(inventory))
	b = protowire.AppendTag(b, 5, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)
	return b
}

func buildAccountBytes(additionalSlots uint32, trial bool) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(additionalSlots))
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	if trial {
		b = protowire.AppendVarint(b, 1)
	} else {
		b = protowire.AppendVarint(b, 0)
	}
	return b
}

func buildSubscribedType(typeID int32, objectData ...[]byte) []byte {
	var st []byte
	st = protowire.AppendTag(st, 1, protowire.VarintType)
	st = protowire.AppendVarint(st, uint64(typeID))
	for _, od := range objectData {
		st = protowire.AppendTag(st, 2, protowire.BytesType)
		st = protowire.AppendBytes(st, od)
	}
	return st
}

func buildCacheSubscribed(types ...[]byte) []byte {
	var msg []byte
	// field 1: owner (fixed64)
	msg = protowire.AppendTag(msg, 1, protowire.Fixed64Type)
	msg = protowire.AppendFixed64(msg, 76561198012345678)
	for _, t := range types {
		msg = protowire.AppendTag(msg, 2, protowire.BytesType)
		msg = protowire.AppendBytes(msg, t)
	}
	return msg
}

func buildSingleObject(typeID int32, objectData []byte) []byte {
	var msg []byte
	msg = protowire.AppendTag(msg, 2, protowire.VarintType)
	msg = protowire.AppendVarint(msg, uint64(typeID))
	msg = protowire.AppendTag(msg, 3, protowire.BytesType)
	msg = protowire.AppendBytes(msg, objectData)
	return msg
}

func buildMultipleObjects(objects ...singleObject) []byte {
	var msg []byte
	for _, so := range objects {
		var entry []byte
		entry = protowire.AppendTag(entry, 1, protowire.VarintType)
		entry = protowire.AppendVarint(entry, uint64(so.typeID))
		entry = protowire.AppendTag(entry, 2, protowire.BytesType)
		entry = protowire.AppendBytes(entry, so.objectData)
		msg = protowire.AppendTag(msg, 2, protowire.BytesType)
		msg = protowire.AppendBytes(msg, entry)
	}
	return msg
}

func buildWelcomeBody(version uint32, countryCode string) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(version))
	if countryCode != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, countryCode)
	}
	return b
}

func buildGoodbyeBody(reason uint32) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(reason))
	return b
}

func setupTestClient(opts ...Option) (*Client, *steamclient.Client, *mockConn) {
	mc := &mockConn{writeCh: make(chan []byte, 10)}
	cm := steamclient.New()
	cm.SetConn(mc)
	tc := New(cm, opts...)
	return tc, cm, mc
}

// --- Tests ---

func TestCacheSubscribedLoadsBackpack(t *testing.T) {
	var loadedItems []*Item
	tc, cm, _ := setupTestClient(
		WithBackpackLoadedHandler(func(items []*Item) {
			loadedItems = items
		}),
	)

	// Build SO cache subscribed with 2 items.
	item1 := buildItemBytes(1001, 5021, 100)
	item2 := buildItemBytes(1002, 5022, 200)
	st := buildSubscribedType(SOTypeItem, item1, item2)
	body := buildCacheSubscribed(st)

	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	if loadedItems == nil {
		t.Fatal("OnBackpackLoaded was not called")
	}
	if len(loadedItems) != 2 {
		t.Fatalf("got %d items, want 2", len(loadedItems))
	}

	bp := tc.Backpack()
	if len(bp) != 2 {
		t.Fatalf("Backpack() returned %d items, want 2", len(bp))
	}

	it := tc.BackpackItem(1001)
	if it == nil {
		t.Fatal("BackpackItem(1001) returned nil")
	}
	if it.DefIndex != 5021 {
		t.Errorf("item 1001 DefIndex = %d, want 5021", it.DefIndex)
	}
}

func TestCacheSubscribedLoadsAccount(t *testing.T) {
	var loadedAcc *Account
	tc, cm, _ := setupTestClient(
		WithAccountLoadedHandler(func(acc *Account) {
			loadedAcc = acc
		}),
	)

	accData := buildAccountBytes(100, false)
	st := buildSubscribedType(SOTypeAccount, accData)
	body := buildCacheSubscribed(st)

	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	if loadedAcc == nil {
		t.Fatal("OnAccountLoaded was not called")
	}
	if !loadedAcc.Premium {
		t.Error("Premium = false, want true")
	}
	if loadedAcc.BackpackSlots != 400 {
		t.Errorf("BackpackSlots = %d, want 400", loadedAcc.BackpackSlots)
	}

	info := tc.AccountInfo()
	if info == nil {
		t.Fatal("AccountInfo() returned nil")
	}
}

func TestSOCreateAddsItem(t *testing.T) {
	var acquired *Item
	tc, cm, _ := setupTestClient(
		WithItemAcquiredHandler(func(item *Item) {
			acquired = item
		}),
	)

	// First load the backpack so SO create works.
	item1 := buildItemBytes(1001, 5021, 100)
	st := buildSubscribedType(SOTypeItem, item1)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	// Now send SO create.
	newItem := buildItemBytes(2001, 5050, 300)
	soBody := buildSingleObject(SOTypeItem, newItem)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCreate, IsProto: true, Body: soBody,
	})

	if acquired == nil {
		t.Fatal("OnItemAcquired was not called")
	}
	if acquired.ID != 2001 {
		t.Errorf("acquired.ID = %d, want 2001", acquired.ID)
	}

	bp := tc.Backpack()
	if len(bp) != 2 {
		t.Fatalf("Backpack() has %d items, want 2", len(bp))
	}
}

func TestSOCreateBeforeBackpackIgnored(t *testing.T) {
	acquiredCalled := false
	_, cm, _ := setupTestClient(
		WithItemAcquiredHandler(func(item *Item) {
			acquiredCalled = true
		}),
	)

	// Send SO create without loading backpack first.
	newItem := buildItemBytes(2001, 5050, 300)
	soBody := buildSingleObject(SOTypeItem, newItem)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCreate, IsProto: true, Body: soBody,
	})

	if acquiredCalled {
		t.Error("OnItemAcquired should not be called before backpack is loaded")
	}
}

func TestSOUpdateChangesItem(t *testing.T) {
	var oldItem, newItem *Item
	_, cm, _ := setupTestClient(
		WithItemChangedHandler(func(old, new_ *Item) {
			oldItem = old
			newItem = new_
		}),
	)

	// Load backpack with one item.
	item1 := buildItemBytes(1001, 5021, 100)
	st := buildSubscribedType(SOTypeItem, item1)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	// Update item: change def_index.
	updated := buildItemBytes(1001, 9999, 200)
	soBody := buildSingleObject(SOTypeItem, updated)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOUpdate, IsProto: true, Body: soBody,
	})

	if oldItem == nil || newItem == nil {
		t.Fatal("OnItemChanged was not called")
	}
	if oldItem.DefIndex != 5021 {
		t.Errorf("old.DefIndex = %d, want 5021", oldItem.DefIndex)
	}
	if newItem.DefIndex != 9999 {
		t.Errorf("new.DefIndex = %d, want 9999", newItem.DefIndex)
	}
}

func TestSOUpdateAccount(t *testing.T) {
	var updatedAcc *Account
	_, cm, _ := setupTestClient(
		WithAccountUpdateHandler(func(acc *Account) {
			updatedAcc = acc
		}),
	)

	// Load account first.
	accData := buildAccountBytes(100, false)
	st := buildSubscribedType(SOTypeAccount, accData)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	// Update account via SO update.
	newAccData := buildAccountBytes(200, false)
	soBody := buildSingleObject(SOTypeAccount, newAccData)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOUpdate, IsProto: true, Body: soBody,
	})

	if updatedAcc == nil {
		t.Fatal("OnAccountUpdate was not called")
	}
	if updatedAcc.BackpackSlots != 500 {
		t.Errorf("BackpackSlots = %d, want 500 (300+200)", updatedAcc.BackpackSlots)
	}
}

func TestSODestroyRemovesItem(t *testing.T) {
	var removed *Item
	tc, cm, _ := setupTestClient(
		WithItemRemovedHandler(func(item *Item) {
			removed = item
		}),
	)

	// Load backpack with two items.
	item1 := buildItemBytes(1001, 5021, 100)
	item2 := buildItemBytes(1002, 5022, 200)
	st := buildSubscribedType(SOTypeItem, item1, item2)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	// Destroy item 1001.
	destroyData := buildItemBytes(1001, 0, 0)
	soBody := buildSingleObject(SOTypeItem, destroyData)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSODestroy, IsProto: true, Body: soBody,
	})

	if removed == nil {
		t.Fatal("OnItemRemoved was not called")
	}
	if removed.ID != 1001 {
		t.Errorf("removed.ID = %d, want 1001", removed.ID)
	}
	if removed.DefIndex != 5021 {
		t.Errorf("removed.DefIndex = %d, want 5021 (original item data)", removed.DefIndex)
	}

	bp := tc.Backpack()
	if len(bp) != 1 {
		t.Fatalf("Backpack() has %d items, want 1", len(bp))
	}
	if tc.BackpackItem(1001) != nil {
		t.Error("item 1001 still in backpack after destroy")
	}
}

func TestSOUpdateMultiple(t *testing.T) {
	changedCount := 0
	_, cm, _ := setupTestClient(
		WithItemChangedHandler(func(old, new_ *Item) {
			changedCount++
		}),
	)

	// Load backpack.
	item1 := buildItemBytes(1001, 5021, 100)
	item2 := buildItemBytes(1002, 5022, 200)
	st := buildSubscribedType(SOTypeItem, item1, item2)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	// Update multiple: both items.
	updated1 := buildItemBytes(1001, 9991, 101)
	updated2 := buildItemBytes(1002, 9992, 201)
	multiBody := buildMultipleObjects(
		singleObject{typeID: SOTypeItem, objectData: updated1},
		singleObject{typeID: SOTypeItem, objectData: updated2},
	)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOUpdateMultiple, IsProto: true, Body: multiBody,
	})

	if changedCount != 2 {
		t.Errorf("OnItemChanged called %d times, want 2", changedCount)
	}
}

func TestCacheResetOnWelcome(t *testing.T) {
	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	tc, cm, mc := setupTestClient()

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-mc.writeCh

	// First welcome.
	welcomeBody := buildWelcomeBody(1000, "US")
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientWelcome, IsProto: true, Body: welcomeBody,
	})

	// Load backpack.
	item1 := buildItemBytes(1001, 5021, 100)
	st := buildSubscribedType(SOTypeItem, item1)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	if len(tc.Backpack()) != 1 {
		t.Fatal("expected 1 item in backpack before reconnect")
	}

	// Simulate reconnect: goodbye then welcome clears cache.
	tc.Disconnect()

	// Re-connect and welcome.
	tc.mu.Lock()
	tc.helloStop = nil // allow reconnect
	tc.mu.Unlock()

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	<-mc.writeCh

	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientWelcome, IsProto: true, Body: welcomeBody,
	})

	if len(tc.Backpack()) != 0 {
		t.Error("backpack should be empty after reconnect welcome")
	}
}

func TestCacheResetOnGoodbye(t *testing.T) {
	tc, cm, _ := setupTestClient()

	// Load backpack.
	item1 := buildItemBytes(1001, 5021, 100)
	st := buildSubscribedType(SOTypeItem, item1)
	body := buildCacheSubscribed(st)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgSOCacheSubscribed, IsProto: true, Body: body,
	})

	if len(tc.Backpack()) != 1 {
		t.Fatal("expected 1 item before goodbye")
	}

	// Goodbye clears cache.
	goodbyeBody := buildGoodbyeBody(1)
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientGoodbye, IsProto: true, Body: goodbyeBody,
	})

	if len(tc.Backpack()) != 0 {
		t.Error("backpack should be empty after goodbye")
	}
}

func TestWelcomeEventParsed(t *testing.T) {
	origTicker := newTicker
	newTicker = func(d time.Duration) ticker { return &fakeTicker{ch: make(chan time.Time)} }
	defer func() { newTicker = origTicker }()

	var ev *WelcomeEvent
	tc, cm, mc := setupTestClient(
		WithConnectedHandler(func(e *WelcomeEvent) {
			ev = e
		}),
	)

	if err := tc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-mc.writeCh

	welcomeBody := buildWelcomeBody(1234, "SE")
	cm.OnGCMessage(&steamclient.GCMessage{
		AppID: AppID, MsgType: MsgClientWelcome, IsProto: true, Body: welcomeBody,
	})

	if ev == nil {
		t.Fatal("OnConnected was not called")
	}
	if ev.Version != 1234 {
		t.Errorf("Version = %d, want 1234", ev.Version)
	}
	if ev.TxnCountryCode != "SE" {
		t.Errorf("TxnCountryCode = %q, want %q", ev.TxnCountryCode, "SE")
	}
}
