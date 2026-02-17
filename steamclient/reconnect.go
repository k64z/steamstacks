package steamclient

import (
	"context"
	"errors"
	"sync"
)

// ErrDisconnected is returned by awaitPacket when the connection is closed.
var ErrDisconnected = errors.New("steamclient: disconnected")

// DisconnectEvent describes why the client disconnected.
type DisconnectEvent struct {
	// Err is the underlying transport error (nil for server-initiated logoff).
	Err error
	// ServerInitiated is true when the server sent EMsgClientLoggedOff.
	ServerInitiated bool
	// EResult is the server's reason code (only meaningful when ServerInitiated is true).
	EResult int32
}

// WithDisconnectHandler sets a callback that fires when the connection drops.
func WithDisconnectHandler(fn func(*DisconnectEvent)) Option {
	return func(c *config) { c.onDisconnect = fn }
}

// fireDisconnect invokes the OnDisconnect callback at most once per connection lifecycle.
// The callback runs in a new goroutine so the caller can safely call Reconnect.
func (c *Client) fireDisconnect(evt *DisconnectEvent) {
	c.disconnectOnce.Do(func() {
		c.mu.Lock()
		c.loggedIn = false
		c.mu.Unlock()
		if c.OnDisconnect != nil {
			go c.OnDisconnect(evt)
		}
	})
}

// Reconnect tears down the existing connection and establishes a new one.
// After Reconnect returns successfully the caller should call Login again.
func (c *Client) Reconnect(ctx context.Context) error {
	// Signal goroutines to stop (safe if already closed).
	c.closeOnce.Do(func() { close(c.done) })

	// Close transport to unblock pending I/O.
	if c.conn != nil {
		c.conn.Close()
	}

	// Wait for readLoop + heartbeatLoop to finish.
	c.wg.Wait()

	// Reset sync primitives for new connection cycle.
	c.closeOnce = sync.Once{}
	c.disconnectOnce = sync.Once{}
	c.mu.Lock()
	c.loggedIn = false
	c.mu.Unlock()

	// Establish new connection (new c.done, new readLoop).
	return c.Connect(ctx)
}
