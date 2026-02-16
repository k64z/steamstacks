package steamclient

import (
	"testing"

	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

func TestUserNotificationsMarshalRoundTrip(t *testing.T) {
	msg := &protocol.CMsgClientUserNotifications{
		Notifications: []*protocol.CMsgClientUserNotifications_Notification{
			{
				UserNotificationType: proto.Uint32(1), // trade offers
				Count:                proto.Uint32(3),
			},
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got protocol.CMsgClientUserNotifications
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.GetNotifications()) != 1 {
		t.Fatalf("got %d notifications, want 1", len(got.GetNotifications()))
	}

	n := got.GetNotifications()[0]
	if n.GetUserNotificationType() != 1 {
		t.Errorf("UserNotificationType = %d, want 1", n.GetUserNotificationType())
	}
	if n.GetCount() != 3 {
		t.Errorf("Count = %d, want 3", n.GetCount())
	}
}

func TestItemAnnouncementsMarshalRoundTrip(t *testing.T) {
	msg := &protocol.CMsgClientItemAnnouncements{
		CountNewItems: proto.Uint32(5),
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got protocol.CMsgClientItemAnnouncements
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GetCountNewItems() != 5 {
		t.Errorf("CountNewItems = %d, want 5", got.GetCountNewItems())
	}
}

func makeUserNotificationsPacket(t *testing.T, notifications []*protocol.CMsgClientUserNotifications_Notification) *Packet {
	t.Helper()
	body, err := proto.Marshal(&protocol.CMsgClientUserNotifications{
		Notifications: notifications,
	})
	if err != nil {
		t.Fatalf("marshal UserNotifications: %v", err)
	}
	return &Packet{EMsg: EMsgClientUserNotifications, IsProto: true, Body: body}
}

func makeItemAnnouncementsPacket(t *testing.T, count uint32) *Packet {
	t.Helper()
	body, err := proto.Marshal(&protocol.CMsgClientItemAnnouncements{
		CountNewItems: proto.Uint32(count),
	})
	if err != nil {
		t.Fatalf("marshal ItemAnnouncements: %v", err)
	}
	return &Packet{EMsg: EMsgClientItemAnnouncements, IsProto: true, Body: body}
}

func TestHandleUserNotificationsTradeOffer(t *testing.T) {
	var got TradeNotification
	c := New(WithTradeNotificationHandler(func(n *TradeNotification) {
		got = *n
	}))

	pkt := makeUserNotificationsPacket(t, []*protocol.CMsgClientUserNotifications_Notification{
		{UserNotificationType: proto.Uint32(1), Count: proto.Uint32(7)},
	})

	c.handlePacket(pkt)

	if got.TradeOffersCount != 7 {
		t.Errorf("TradeOffersCount = %d, want 7", got.TradeOffersCount)
	}
}

func TestHandleUserNotificationsIgnoresNonTrade(t *testing.T) {
	var called bool
	c := New(WithTradeNotificationHandler(func(n *TradeNotification) {
		called = true
	}))

	// Type 2 is not trade offers — should not fire the callback
	pkt := makeUserNotificationsPacket(t, []*protocol.CMsgClientUserNotifications_Notification{
		{UserNotificationType: proto.Uint32(2), Count: proto.Uint32(10)},
	})

	c.handlePacket(pkt)

	if called {
		t.Error("OnTradeNotification was called for non-trade notification type")
	}
}

func TestHandleItemAnnouncements(t *testing.T) {
	var got ItemNotification
	c := New(WithItemNotificationHandler(func(n *ItemNotification) {
		got = *n
	}))

	pkt := makeItemAnnouncementsPacket(t, 12)
	c.handlePacket(pkt)

	if got.NewItemCount != 12 {
		t.Errorf("NewItemCount = %d, want 12", got.NewItemCount)
	}
}

func TestHandleNotificationsNilHandler(t *testing.T) {
	c := New() // no handlers set

	// Should not panic
	pkt1 := makeUserNotificationsPacket(t, []*protocol.CMsgClientUserNotifications_Notification{
		{UserNotificationType: proto.Uint32(1), Count: proto.Uint32(1)},
	})
	c.handlePacket(pkt1)

	pkt2 := makeItemAnnouncementsPacket(t, 3)
	c.handlePacket(pkt2)
}

func TestNotificationsOnPacketPassthrough(t *testing.T) {
	var tradeCalled, itemCalled, pktCount int

	c := New(
		WithTradeNotificationHandler(func(n *TradeNotification) { tradeCalled++ }),
		WithItemNotificationHandler(func(n *ItemNotification) { itemCalled++ }),
		WithPacketHandler(func(p *Packet) { pktCount++ }),
	)

	pkt1 := makeUserNotificationsPacket(t, []*protocol.CMsgClientUserNotifications_Notification{
		{UserNotificationType: proto.Uint32(1), Count: proto.Uint32(2)},
	})
	c.handlePacket(pkt1)

	pkt2 := makeItemAnnouncementsPacket(t, 1)
	c.handlePacket(pkt2)

	if tradeCalled != 1 {
		t.Errorf("OnTradeNotification called %d times, want 1", tradeCalled)
	}
	if itemCalled != 1 {
		t.Errorf("OnItemNotification called %d times, want 1", itemCalled)
	}
	if pktCount != 2 {
		t.Errorf("OnPacket called %d times, want 2", pktCount)
	}
}
