package steamclient

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
)

// Connection abstracts a transport to a Steam CM server.
type Connection interface {
	Write(ctx context.Context, data []byte) error
	Read(ctx context.Context) ([]byte, error)
	Close() error
	RemoteAddr() string
}

// wsConn implements Connection over WebSocket.
type wsConn struct {
	conn *websocket.Conn
	addr string
}

func dialWebSocket(ctx context.Context, host string) (*wsConn, error) {
	url := fmt.Sprintf("wss://%s/cmsocket/", host)

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", url, err)
	}

	// Steam can send large multi messages
	conn.SetReadLimit(1 << 24) // 16 MB

	return &wsConn{conn: conn, addr: host}, nil
}

func (w *wsConn) Write(ctx context.Context, data []byte) error {
	return w.conn.Write(ctx, websocket.MessageBinary, data)
}

func (w *wsConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := w.conn.Read(ctx)
	return data, err
}

func (w *wsConn) Close() error {
	return w.conn.CloseNow()
}

func (w *wsConn) RemoteAddr() string {
	return w.addr
}
