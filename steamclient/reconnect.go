package steamclient

import (
	"context"
	"errors"
)

// ErrDisconnected is returned when the connection is closed or was never
// established: by awaitPacket while waiting for a response, and by sends
// attempted before Connect.
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
	c.mu.Lock()
	if c.disconnectFired {
		c.mu.Unlock()
		return
	}
	c.disconnectFired = true
	c.loggedIn = false
	// The wallet cache is session-scoped: a caller that sees
	// WalletInfo ok=true must be able to read it as "the CM told me
	// this during the session I have now", not a figure left over
	// from a connection that has since dropped. Steam re-pushes on
	// logon, so the gap is brief.
	c.walletInfo = nil
	c.mu.Unlock()

	if c.OnDisconnect != nil {
		go c.OnDisconnect(evt)
	}
}

// Reconnect tears down the existing connection and establishes a new one.
// After Reconnect returns successfully the caller should call Login again.
func (c *Client) Reconnect(ctx context.Context) error {
	// Signal goroutines to stop and unblock pending I/O.
	c.closeDone()
	c.closeConn()

	// Wait for readLoop + heartbeatLoop to finish.
	c.wg.Wait()

	c.mu.Lock()
	c.loggedIn = false
	c.walletInfo = nil
	c.mu.Unlock()

	// Connect installs a fresh done channel, readLoop, and per-cycle flags.
	return c.Connect(ctx)
}
