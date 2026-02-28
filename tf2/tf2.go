package tf2

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/k64z/steamstacks/steamclient"
	"google.golang.org/protobuf/encoding/protowire"
)

const AppID = 440

// GC message types for the TF2 Game Coordinator.
const (
	MsgClientWelcome  = 4004
	MsgClientHello    = 4006
	MsgClientGoodbye  = 4008
	MsgUseItemRequest = 1025
)

// WelcomeEvent is fired when the TF2 GC accepts our session.
type WelcomeEvent struct {
	Version        uint32
	TxnCountryCode string
}

// GoodbyeEvent is fired when the TF2 GC ends our session.
type GoodbyeEvent struct {
	Reason uint32
}

// Client manages a session with the TF2 Game Coordinator.
type Client struct {
	cm     *steamclient.Client
	logger *slog.Logger

	OnConnected    func(*WelcomeEvent)
	OnDisconnected func(*GoodbyeEvent)
	OnGCMessage    func(*steamclient.GCMessage)

	OnBackpackLoaded func([]*Item)
	OnAccountLoaded  func(*Account)
	OnItemAcquired   func(*Item)
	OnItemChanged    func(old, new_ *Item)
	OnItemRemoved    func(*Item)
	OnAccountUpdate  func(*Account)

	mu        sync.Mutex
	connected bool
	helloStop chan struct{}
	cache     *soCache
}

type config struct {
	logger           *slog.Logger
	onConnected      func(*WelcomeEvent)
	onDisconnected   func(*GoodbyeEvent)
	onGCMessage      func(*steamclient.GCMessage)
	onBackpackLoaded func([]*Item)
	onAccountLoaded  func(*Account)
	onItemAcquired   func(*Item)
	onItemChanged    func(old, new_ *Item)
	onItemRemoved    func(*Item)
	onAccountUpdate  func(*Account)
}

// Option configures a TF2 Client.
type Option func(*config)

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithConnectedHandler sets a callback for when the TF2 GC session is established.
func WithConnectedHandler(fn func(*WelcomeEvent)) Option {
	return func(c *config) { c.onConnected = fn }
}

// WithDisconnectedHandler sets a callback for when the TF2 GC session ends.
func WithDisconnectedHandler(fn func(*GoodbyeEvent)) Option {
	return func(c *config) { c.onDisconnected = fn }
}

// WithGCMessageHandler sets a callback for TF2 GC messages not handled internally.
func WithGCMessageHandler(fn func(*steamclient.GCMessage)) Option {
	return func(c *config) { c.onGCMessage = fn }
}

// WithBackpackLoadedHandler sets a callback for when the backpack is loaded from the SO cache.
func WithBackpackLoadedHandler(fn func([]*Item)) Option {
	return func(c *config) { c.onBackpackLoaded = fn }
}

// WithAccountLoadedHandler sets a callback for when account metadata is loaded from the SO cache.
func WithAccountLoadedHandler(fn func(*Account)) Option {
	return func(c *config) { c.onAccountLoaded = fn }
}

// WithItemAcquiredHandler sets a callback for when a new item is added to the backpack.
func WithItemAcquiredHandler(fn func(*Item)) Option {
	return func(c *config) { c.onItemAcquired = fn }
}

// WithItemChangedHandler sets a callback for when an existing item is updated.
func WithItemChangedHandler(fn func(old, new_ *Item)) Option {
	return func(c *config) { c.onItemChanged = fn }
}

// WithItemRemovedHandler sets a callback for when an item is removed from the backpack.
func WithItemRemovedHandler(fn func(*Item)) Option {
	return func(c *config) { c.onItemRemoved = fn }
}

// WithAccountUpdateHandler sets a callback for when account metadata is updated.
func WithAccountUpdateHandler(fn func(*Account)) Option {
	return func(c *config) { c.onAccountUpdate = fn }
}

// New creates a new TF2 GC client. It chains onto the CM client's OnGCMessage
// callback, filtering for AppID 440 and forwarding non-TF2 messages to any
// previously installed handler.
func New(cm *steamclient.Client, opts ...Option) *Client {
	cfg := config{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &Client{
		cm:               cm,
		logger:           cfg.logger,
		OnConnected:      cfg.onConnected,
		OnDisconnected:   cfg.onDisconnected,
		OnGCMessage:      cfg.onGCMessage,
		OnBackpackLoaded: cfg.onBackpackLoaded,
		OnAccountLoaded:  cfg.onAccountLoaded,
		OnItemAcquired:   cfg.onItemAcquired,
		OnItemChanged:    cfg.onItemChanged,
		OnItemRemoved:    cfg.onItemRemoved,
		OnAccountUpdate:  cfg.onAccountUpdate,
		cache:            newSOCache(),
	}

	prev := cm.OnGCMessage
	cm.OnGCMessage = func(msg *steamclient.GCMessage) {
		if msg.AppID == AppID {
			c.handleGCMessage(msg)
			return
		}
		if prev != nil {
			prev(msg)
		}
	}

	return c
}

// Connect starts the TF2 GC session by sending CMsgClientHello in a loop
// until the GC responds with CMsgClientWelcome.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.helloStop != nil {
		c.mu.Unlock()
		return fmt.Errorf("tf2: already connecting")
	}
	c.helloStop = make(chan struct{})
	c.mu.Unlock()

	return c.sendHello(ctx)
}

// Disconnect stops the hello loop and marks the session as disconnected.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	c.cache.reset()
	if c.helloStop != nil {
		close(c.helloStop)
		c.helloStop = nil
	}
}

// IsConnected reports whether the TF2 GC session is active.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// SendMessage sends a protobuf message to the TF2 GC.
func (c *Client) SendMessage(ctx context.Context, msgType uint32, body []byte) error {
	return c.cm.SendGCMessage(ctx, AppID, msgType, true, body)
}

// UseItem sends a CMsgUseItem to the TF2 GC, which triggers use of the
// specified item (opening crates, consuming items, applying tools, etc.).
func (c *Client) UseItem(ctx context.Context, itemID uint64) error {
	var body []byte
	body = protowire.AppendTag(body, 1, protowire.VarintType)
	body = protowire.AppendVarint(body, itemID)
	return c.SendMessage(ctx, MsgUseItemRequest, body)
}

// Backpack returns a snapshot copy of all backpack items.
func (c *Client) Backpack() []*Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	items := make([]*Item, 0, len(c.cache.items))
	for _, it := range c.cache.items {
		items = append(items, it)
	}
	return items
}

// BackpackItem returns a single item by ID, or nil if not found.
func (c *Client) BackpackItem(id uint64) *Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.items[id]
}

// AccountInfo returns the current account metadata, or nil if not yet loaded.
func (c *Client) AccountInfo() *Account {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.account
}

func (c *Client) handleGCMessage(msg *steamclient.GCMessage) {
	switch msg.MsgType {
	case MsgClientWelcome:
		ev := parseWelcome(msg.Body)

		c.mu.Lock()
		c.cache.reset()
		c.connected = true
		if c.helloStop != nil {
			close(c.helloStop)
			c.helloStop = nil
		}
		c.mu.Unlock()

		c.logger.Info("tf2 GC session established", "version", ev.Version)
		if c.OnConnected != nil {
			c.OnConnected(ev)
		}

	case MsgClientGoodbye:
		ev := parseGoodbye(msg.Body)

		c.mu.Lock()
		c.connected = false
		c.cache.reset()
		c.mu.Unlock()

		c.logger.Info("tf2 GC session ended", "reason", ev.Reason)
		if c.OnDisconnected != nil {
			c.OnDisconnected(ev)
		}

	case MsgSOCacheSubscriptionCheck:
		c.handleSOCacheSubscriptionCheck()
	case MsgSOCacheSubscribed:
		c.handleSOCacheSubscribed(msg.Body)
	case MsgSOCreate:
		c.handleSOCreate(msg.Body)
	case MsgSOUpdate:
		c.handleSOUpdate(msg.Body)
	case MsgSODestroy:
		c.handleSODestroy(msg.Body)
	case MsgSOUpdateMultiple:
		c.handleSOUpdateMultiple(msg.Body)

	default:
		if c.OnGCMessage != nil {
			c.OnGCMessage(msg)
		}
	}
}

func parseWelcome(b []byte) *WelcomeEvent {
	ev := &WelcomeEvent{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ev
			}
			b = b[n:]
			if num == 1 {
				ev.Version = uint32(v)
			}
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return ev
			}
			b = b[n:]
			if num == 3 {
				ev.TxnCountryCode = string(v)
			}
		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return ev
			}
			b = b[n:]
		}
	}
	return ev
}

func parseGoodbye(b []byte) *GoodbyeEvent {
	ev := &GoodbyeEvent{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]

		switch wtype {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return ev
			}
			b = b[n:]
			if num == 1 {
				ev.Reason = uint32(v)
			}
		default:
			n := protowire.ConsumeFieldValue(num, wtype, b)
			if n < 0 {
				return ev
			}
			b = b[n:]
		}
	}
	return ev
}

func (c *Client) sendHello(ctx context.Context) error {
	// Encode CMsgClientHello with protowire to avoid name conflicts with
	// protocol.CMsgClientHello. The GC hello has a single field:
	// field 1 (uint32): client_launcher = 0
	var body []byte
	body = protowire.AppendTag(body, 1, protowire.VarintType)
	body = protowire.AppendVarint(body, 0)

	if err := c.cm.SendGCMessage(ctx, AppID, MsgClientHello, true, body); err != nil {
		return fmt.Errorf("tf2: send hello: %w", err)
	}

	c.mu.Lock()
	stop := c.helloStop
	c.mu.Unlock()

	// Start background hello loop — resends every 5 seconds until welcome or stop.
	go c.helloLoop(stop, body)

	return nil
}

func (c *Client) helloLoop(stop <-chan struct{}, helloBody []byte) {
	ticker := newTicker(helloInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C():
			if err := c.cm.SendGCMessage(context.Background(), AppID, MsgClientHello, true, helloBody); err != nil {
				c.logger.Error("tf2: resend hello failed", "err", err)
				return
			}
			c.logger.Debug("tf2: hello resent")
		}
	}
}
