package steamclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/k64z/steamstacks/protocol"
	"github.com/k64z/steamstacks/steamid"
	"google.golang.org/protobuf/proto"
)

// TransportType selects the CM transport layer.
type TransportType int

const (
	TransportWebSocket TransportType = iota
	TransportTCP
)

// Client manages a connection to a Steam CM server.
type Client struct {
	conn      Connection
	steamID   steamid.SteamID
	sessionID int32

	transport  TransportType
	httpClient *http.Client
	logger     *slog.Logger

	// OnPacket is called for every decoded packet not handled internally.
	OnPacket func(*Packet)

	// OnFriendMessage is called for incoming chat messages.
	OnFriendMessage func(*FriendMessage)

	// OnRelationship is called for friend list / relationship changes.
	OnRelationship func(*RelationshipEvent)

	// OnPersonaState is called when a friend's persona state changes.
	OnPersonaState func(*PersonaStateEvent)

	// OnTradeNotification is called when the pending trade offer count changes.
	OnTradeNotification func(*TradeNotification)

	// OnItemNotification is called when new inventory items arrive.
	OnItemNotification func(*ItemNotification)

	// OnDisconnect is called when the connection drops unexpectedly.
	OnDisconnect func(*DisconnectEvent)

	nextJobID   atomic.Uint64
	pendingJobs map[uint64]chan<- *Packet // protected by mu

	mu             sync.Mutex
	done           chan struct{} // closed on Disconnect
	wg             sync.WaitGroup
	loggedIn       bool
	closeOnce      sync.Once
	disconnectOnce sync.Once
}

type config struct {
	transport           TransportType
	httpClient          *http.Client
	logger              *slog.Logger
	onPacket            func(*Packet)
	onFriendMsg         func(*FriendMessage)
	onRelationship      func(*RelationshipEvent)
	onPersonaState      func(*PersonaStateEvent)
	onTradeNotification func(*TradeNotification)
	onItemNotification  func(*ItemNotification)
	onDisconnect        func(*DisconnectEvent)
}

// Option configures a Client.
type Option func(*config)

// WithTransport sets the transport type (WebSocket or TCP).
func WithTransport(t TransportType) Option {
	return func(c *config) { c.transport = t }
}

// WithHTTPClient sets the HTTP client used for server discovery.
func WithHTTPClient(h *http.Client) Option {
	return func(c *config) { c.httpClient = h }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithPacketHandler sets a callback for packets not handled internally.
func WithPacketHandler(fn func(*Packet)) Option {
	return func(c *config) { c.onPacket = fn }
}

// WithFriendMessageHandler sets a callback for incoming friend chat messages.
func WithFriendMessageHandler(fn func(*FriendMessage)) Option {
	return func(c *config) { c.onFriendMsg = fn }
}

// WithRelationshipHandler sets a callback for friend list / relationship changes.
func WithRelationshipHandler(fn func(*RelationshipEvent)) Option {
	return func(c *config) { c.onRelationship = fn }
}

// WithPersonaStateHandler sets a callback for persona state changes.
func WithPersonaStateHandler(fn func(*PersonaStateEvent)) Option {
	return func(c *config) { c.onPersonaState = fn }
}

// New creates a new Steam CM client.
func New(opts ...Option) *Client {
	cfg := config{
		transport:  TransportWebSocket,
		httpClient: http.DefaultClient,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Client{
		transport:           cfg.transport,
		httpClient:          cfg.httpClient,
		logger:              cfg.logger,
		OnPacket:            cfg.onPacket,
		OnFriendMessage:     cfg.onFriendMsg,
		OnRelationship:      cfg.onRelationship,
		OnPersonaState:      cfg.onPersonaState,
		OnTradeNotification: cfg.onTradeNotification,
		OnItemNotification:  cfg.onItemNotification,
		OnDisconnect:        cfg.onDisconnect,
	}
}

// Connect discovers CM servers, dials one, and prepares the connection.
// For TCP, this includes the encryption handshake.
func (c *Client) Connect(ctx context.Context) error {
	servers, err := DiscoverServers(ctx, c.httpClient)
	if err != nil {
		return fmt.Errorf("discover servers: %w", err)
	}

	targetType := "websockets"
	if c.transport == TransportTCP {
		targetType = "netfilter"
	}

	var candidates []CMServer
	for _, s := range servers {
		if s.Type == targetType {
			candidates = append(candidates, s)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no %s servers found", targetType)
	}

	server := candidates[rand.IntN(len(candidates))]
	c.logger.Info("connecting to CM server", "addr", server.Addr, "type", server.Type)

	switch c.transport {
	case TransportWebSocket:
		ws, err := dialWebSocket(ctx, server.Addr)
		if err != nil {
			return err
		}
		c.conn = ws

	case TransportTCP:
		tcp, err := dialTCP(ctx, server.Addr)
		if err != nil {
			return err
		}
		if err := tcp.performEncryptionHandshake(ctx); err != nil {
			tcp.Close()
			return fmt.Errorf("encryption handshake: %w", err)
		}
		c.conn = tcp
	}

	c.done = make(chan struct{})
	c.wg.Add(1)
	go c.readLoop()

	c.logger.Info("connected", "addr", c.conn.RemoteAddr())
	return nil
}

// Login authenticates with the CM server using an account name and refresh token.
func (c *Client) Login(ctx context.Context, accountName, refreshToken string, sid steamid.SteamID) error {
	loginSID := steamid.SteamID(0).
		SetUniverse(1).
		SetType(1).
		SetInstance(1).
		SetAccountID(sid.AccountID())

	sidU64 := loginSID.ToSteamID64()

	helloBody, err := proto.Marshal(&protocol.CMsgClientHello{
		ProtocolVersion: proto.Uint32(ProtoVersion),
	})
	if err != nil {
		return fmt.Errorf("marshal ClientHello: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientHello, nil, helloBody); err != nil {
		return fmt.Errorf("send ClientHello: %w", err)
	}

	// Install response handler BEFORE sending logon to avoid race with readLoop
	responseCh := c.expectEMsg(EMsgClientLogOnResponse)

	osType := uint32(20) // EOSType Windows 11
	lang := "english"

	logonBody, err := proto.Marshal(&protocol.CMsgClientLogon{
		AccountName:            &accountName,
		AccessToken:            &refreshToken,
		ShouldRememberPassword: proto.Bool(true),
		ProtocolVersion:        proto.Uint32(ProtoVersion),
		ClientOsType:           &osType,
		ClientLanguage:         &lang,
	})
	if err != nil {
		return fmt.Errorf("marshal ClientLogon: %w", err)
	}

	if err := c.sendPacket(ctx, EMsgClientLogon, &protocol.CMsgProtoBufHeader{
		Steamid:         &sidU64,
		ClientSessionid: proto.Int32(0),
	}, logonBody); err != nil {
		return fmt.Errorf("send ClientLogon: %w", err)
	}

	pkt, err := c.awaitPacket(ctx, responseCh)
	if err != nil {
		return fmt.Errorf("wait for logon response: %w", err)
	}

	var resp protocol.CMsgClientLogonResponse
	if err := proto.Unmarshal(pkt.Body, &resp); err != nil {
		return fmt.Errorf("unmarshal logon response: %w", err)
	}

	if resp.GetEresult() != 1 { // EResult.OK
		return fmt.Errorf("logon failed: eresult=%d", resp.GetEresult())
	}

	c.mu.Lock()
	c.steamID = steamid.FromSteamID64(pkt.Header.GetSteamid())
	c.sessionID = pkt.Header.GetClientSessionid()
	c.loggedIn = true
	c.mu.Unlock()

	heartbeatSec := resp.GetHeartbeatSeconds()
	if heartbeatSec <= 0 {
		heartbeatSec = 30 // fallback
	}

	c.wg.Add(1)
	go c.heartbeatLoop(time.Duration(heartbeatSec) * time.Second)

	c.logger.Info("logged in",
		"steamid", c.steamID.String(),
		"session_id", c.sessionID,
		"heartbeat_sec", heartbeatSec,
	)

	return nil
}

// Disconnect cleanly disconnects from the CM server.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	wasLoggedIn := c.loggedIn
	var sidU64 uint64
	var sessionID int32
	if wasLoggedIn {
		sidU64 = c.steamID.ToSteamID64()
		sessionID = c.sessionID
		c.loggedIn = false
	}
	c.mu.Unlock()

	// Send ClientLogOff best-effort (outside the lock to avoid deadlock
	// with sendPacket which also acquires c.mu).
	if wasLoggedIn {
		body, _ := proto.Marshal(&protocol.CMsgClientLogOff{})
		_ = c.sendPacket(context.Background(), EMsgClientLogOff, &protocol.CMsgProtoBufHeader{
			Steamid:         &sidU64,
			ClientSessionid: &sessionID,
		}, body)
	}

	c.closeOnce.Do(func() { close(c.done) })

	if c.conn != nil {
		c.conn.Close()
	}
	c.wg.Wait()

	// Reset sync primitives for potential reuse via Reconnect.
	c.closeOnce = sync.Once{}
	c.disconnectOnce = sync.Once{}

	c.logger.Info("disconnected")
	return nil
}

func (c *Client) sendPacket(ctx context.Context, emsg EMsg, hdr *protocol.CMsgProtoBufHeader, body []byte) error {
	if hdr == nil {
		hdr = &protocol.CMsgProtoBufHeader{}
	}

	c.mu.Lock()
	if c.loggedIn {
		sid := c.steamID.ToSteamID64()
		hdr.Steamid = &sid
		hdr.ClientSessionid = &c.sessionID
	}
	c.mu.Unlock()

	pkt := &Packet{
		EMsg:    emsg,
		IsProto: true,
		Header:  hdr,
		Body:    body,
	}

	data, err := encodePacket(pkt)
	if err != nil {
		return fmt.Errorf("encode %s: %w", emsg, err)
	}

	return c.conn.Write(ctx, data)
}

func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		data, err := c.conn.Read(context.Background())
		if err != nil {
			select {
			case <-c.done:
				return // expected disconnect
			default:
				if !errors.Is(err, context.Canceled) {
					c.logger.Error("read error", "err", err)
				}
				c.fireDisconnect(&DisconnectEvent{Err: err})
				return
			}
		}

		pkt, err := decodePacket(data)
		if err != nil {
			c.logger.Error("decode error", "err", err)
			continue
		}

		c.handlePacket(pkt)
	}
}

func (c *Client) handlePacket(pkt *Packet) {
	// EMsgMulti is handled recursively and never forwarded to OnPacket.
	if pkt.EMsg == EMsgMulti {
		var multi protocol.CMsgMulti
		if err := proto.Unmarshal(pkt.Body, &multi); err != nil {
			c.logger.Error("unmarshal Multi", "err", err)
			return
		}

		packets, err := decodeMulti(multi.GetMessageBody(), multi.GetSizeUnzipped())
		if err != nil {
			c.logger.Error("decode Multi", "err", err)
			return
		}

		for _, sub := range packets {
			c.handlePacket(sub)
		}
		return
	}

	// Dispatch pending service method responses by job ID.
	// The response EMsg varies (146, 147, 152) so we match all packets.
	c.mu.Lock()
	ch, ok := c.pendingJobs[pkt.Header.GetJobidTarget()]
	if ok {
		delete(c.pendingJobs, pkt.Header.GetJobidTarget())
	}
	c.mu.Unlock()
	if ok {
		select {
		case ch <- pkt:
		default:
		}
	}

	// Dispatch to type-specific handlers.
	switch pkt.EMsg {
	case EMsgClientLoggedOff:
		var logoff protocol.CMsgClientLoggedOff
		eresult := int32(2)
		if err := proto.Unmarshal(pkt.Body, &logoff); err == nil {
			eresult = logoff.GetEresult()
		}
		c.logger.Warn("logged off by server", "eresult", eresult)
		c.fireDisconnect(&DisconnectEvent{ServerInitiated: true, EResult: eresult})
		// Close connection — readLoop will exit cleanly on next Read().
		c.closeOnce.Do(func() { close(c.done) })
		if c.conn != nil {
			c.conn.Close()
		}

	case EMsgClientFriendsList:
		c.handleFriendsList(pkt)

	case EMsgClientPersonaState:
		c.handlePersonaState(pkt)

	case EMsgClientFriendMsgIncoming, EMsgClientFriendMsgEchoToSender:
		c.handleFriendMsgIncoming(pkt)

	case EMsgClientUserNotifications:
		c.handleUserNotifications(pkt)

	case EMsgClientItemAnnouncements:
		c.handleItemAnnouncements(pkt)
	}

	// Forward all non-Multi packets to the generic handler.
	if c.OnPacket != nil {
		c.OnPacket(pkt)
	}
}

// expectEMsg installs a one-shot packet listener for the given EMsg.
// Call this BEFORE sending the request to avoid a race with readLoop.
// Use awaitPacket to block until the response arrives.
func (c *Client) expectEMsg(target EMsg) <-chan *Packet {
	ch := make(chan *Packet, 1)

	prev := c.OnPacket
	c.OnPacket = func(pkt *Packet) {
		if pkt.EMsg == target {
			select {
			case ch <- pkt:
			default:
			}
			c.OnPacket = prev
		}
		if prev != nil {
			prev(pkt)
		}
	}

	return ch
}

// awaitPacket blocks until a packet arrives on ch, ctx expires, or the connection closes.
func (c *Client) awaitPacket(ctx context.Context, ch <-chan *Packet) (*Packet, error) {
	select {
	case pkt := <-ch:
		return pkt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrDisconnected
	}
}

// expectJobID registers a one-shot listener for a service method response
// matched by JobidTarget. The match is handled directly in handlePacket
// under the mutex, avoiding data races with readLoop.
func (c *Client) expectJobID(jobID uint64) <-chan *Packet {
	ch := make(chan *Packet, 1)
	c.mu.Lock()
	if c.pendingJobs == nil {
		c.pendingJobs = make(map[uint64]chan<- *Packet)
	}
	c.pendingJobs[jobID] = ch
	c.mu.Unlock()
	return ch
}

// callServiceMethod sends a unified service method request and awaits the
// matching response, correlated by job ID.
func (c *Client) callServiceMethod(ctx context.Context, method string, body []byte) (*Packet, error) {
	jobID := c.nextJobID.Add(1)
	responseCh := c.expectJobID(jobID)
	defer func() {
		c.mu.Lock()
		delete(c.pendingJobs, jobID)
		c.mu.Unlock()
	}()

	hdr := &protocol.CMsgProtoBufHeader{
		TargetJobName: proto.String(method),
		JobidSource:   proto.Uint64(jobID),
	}
	if err := c.sendPacket(ctx, EMsgServiceMethodCallFromClient, hdr, body); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	pkt, err := c.awaitPacket(ctx, responseCh)
	if err != nil {
		return nil, fmt.Errorf("wait for %s response: %w", method, err)
	}
	if pkt.Header.GetEresult() != 1 {
		return pkt, fmt.Errorf("service method %s: eresult=%d", method, pkt.Header.GetEresult())
	}
	return pkt, nil
}

func (c *Client) heartbeatLoop(interval time.Duration) {
	defer c.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			body, _ := proto.Marshal(&protocol.CMsgClientHeartBeat{})
			if err := c.sendPacket(context.Background(), EMsgClientHeartBeat, nil, body); err != nil {
				c.logger.Error("heartbeat failed", "err", err)
				return
			}
			c.logger.Debug("heartbeat sent")
		}
	}
}
