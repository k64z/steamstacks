package steamclient

import (
	"github.com/k64z/steamstacks/protocol"
	"google.golang.org/protobuf/proto"
)

// userNotificationType values from the Steam CM protocol.
const (
	userNotificationTypeTradeOffer uint32 = 1
)

// TradeNotification is fired when the number of pending trade offers changes.
type TradeNotification struct {
	TradeOffersCount uint32 // number of pending trade offers (0 = none pending)
}

// ItemNotification is fired when new inventory items arrive.
type ItemNotification struct {
	NewItemCount uint32 // number of new items (0 = none)
}

// WithTradeNotificationHandler sets a callback for trade offer notifications.
func WithTradeNotificationHandler(fn func(*TradeNotification)) Option {
	return func(c *config) { c.onTradeNotification = fn }
}

// WithItemNotificationHandler sets a callback for new inventory item notifications.
func WithItemNotificationHandler(fn func(*ItemNotification)) Option {
	return func(c *config) { c.onItemNotification = fn }
}

// handleUserNotifications processes an EMsgClientUserNotifications packet.
func (c *Client) handleUserNotifications(pkt *Packet) {
	var msg protocol.CMsgClientUserNotifications
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal UserNotifications", "err", err)
		return
	}

	if c.OnTradeNotification == nil {
		return
	}

	for _, n := range msg.GetNotifications() {
		if n.GetUserNotificationType() == userNotificationTypeTradeOffer {
			c.OnTradeNotification(&TradeNotification{
				TradeOffersCount: n.GetCount(),
			})
		}
	}
}

// handleItemAnnouncements processes an EMsgClientItemAnnouncements packet.
func (c *Client) handleItemAnnouncements(pkt *Packet) {
	var msg protocol.CMsgClientItemAnnouncements
	if err := proto.Unmarshal(pkt.Body, &msg); err != nil {
		c.logger.Error("unmarshal ItemAnnouncements", "err", err)
		return
	}

	if c.OnItemNotification == nil {
		return
	}

	c.OnItemNotification(&ItemNotification{
		NewItemCount: msg.GetCountNewItems(),
	})
}
