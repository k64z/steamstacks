package steamclient

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

func TestFireDisconnectCallsHandler(t *testing.T) {
	got := make(chan *DisconnectEvent, 1)
	c := New(WithDisconnectHandler(func(evt *DisconnectEvent) {
		got <- evt
	}))
	c.done = make(chan struct{})

	want := &DisconnectEvent{Err: context.Canceled}
	c.fireDisconnect(want)

	select {
	case evt := <-got:
		if evt.Err != context.Canceled {
			t.Errorf("Err = %v, want %v", evt.Err, context.Canceled)
		}
		if evt.ServerInitiated {
			t.Error("ServerInitiated should be false")
		}
	case <-time.After(time.Second):
		t.Fatal("OnDisconnect was not called within 1s")
	}
}

func TestFireDisconnectOnlyOnce(t *testing.T) {
	var count int
	var mu sync.Mutex
	done := make(chan struct{})

	c := New(WithDisconnectHandler(func(evt *DisconnectEvent) {
		mu.Lock()
		count++
		mu.Unlock()
		// Signal after first invocation.
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	c.done = make(chan struct{})

	c.fireDisconnect(&DisconnectEvent{Err: context.Canceled})
	c.fireDisconnect(&DisconnectEvent{Err: context.DeadlineExceeded})
	c.fireDisconnect(&DisconnectEvent{ServerInitiated: true})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("OnDisconnect was not called within 1s")
	}

	// Give a little time for any extra goroutines to fire (they shouldn't).
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("OnDisconnect called %d times, want 1", count)
	}
}

func TestFireDisconnectNilHandler(t *testing.T) {
	c := New() // no disconnect handler
	c.done = make(chan struct{})

	// Should not panic.
	c.fireDisconnect(&DisconnectEvent{Err: context.Canceled})
}

func TestLoggedOffFiresDisconnect(t *testing.T) {
	got := make(chan *DisconnectEvent, 1)
	c := New(WithDisconnectHandler(func(evt *DisconnectEvent) {
		got <- evt
	}))
	c.done = make(chan struct{})

	body, err := proto.Marshal(&protocol.CMsgClientLoggedOff{
		Eresult: proto.Int32(6),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pkt := &Packet{
		EMsg:    EMsgClientLoggedOff,
		IsProto: true,
		Header:  &protocol.CMsgProtoBufHeader{},
		Body:    body,
	}
	c.handlePacket(pkt)

	select {
	case evt := <-got:
		if !evt.ServerInitiated {
			t.Error("ServerInitiated should be true")
		}
		if evt.EResult != 6 {
			t.Errorf("EResult = %d, want 6", evt.EResult)
		}
		if evt.Err != nil {
			t.Errorf("Err = %v, want nil", evt.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("OnDisconnect was not called within 1s")
	}
}

func TestAwaitPacketReturnsOnDone(t *testing.T) {
	c := New()
	c.done = make(chan struct{})

	ch := make(chan *Packet, 1)
	close(c.done) // simulate disconnect

	pkt, err := c.awaitPacket(context.Background(), ch)
	if pkt != nil {
		t.Errorf("pkt = %v, want nil", pkt)
	}
	if err != ErrDisconnected {
		t.Errorf("err = %v, want %v", err, ErrDisconnected)
	}
}
