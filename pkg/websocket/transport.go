package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/patrickkabwe/grx/core"
)

// Subprotocol is the only WebSocket subprotocol this transport speaks. The
// legacy Apollo subscriptions-transport-ws ("graphql-ws") protocol is
// intentionally not supported; it was deprecated in 2021 and clients that
// have not migrated should upgrade rather than be carried indefinitely.
const Subprotocol = "graphql-transport-ws"

// Default values applied when the corresponding Config field is left zero.
const (
	DefaultMaxMessageSize        = 1 << 20 // 1 MiB
	DefaultConnectionInitTimeout = 3 * time.Second
)

// Config tunes a WebSocket Transport. All fields are optional; zero values
// fall back to safe production defaults documented per field.
type Config struct {
	// ConnectionInitTimeout is the maximum time the dispatcher waits for a
	// ConnectionInit message after the WebSocket handshake completes. Once
	// exceeded, the connection is closed with code 4408. Zero defaults to
	// DefaultConnectionInitTimeout (3s); a negative value disables the
	// timeout.
	ConnectionInitTimeout time.Duration

	// ReadIdleTimeout caps the time the dispatcher will wait for any next
	// frame from the peer once the connection is initialised. The deadline
	// resets after each successful ReadMessage. Zero disables the timeout.
	ReadIdleTimeout time.Duration

	// WriteTimeout caps a single frame write. Slow consumers that fail to
	// drain a frame within the deadline are closed; this is the primary
	// backpressure protection. Zero disables the timeout.
	WriteTimeout time.Duration

	// MaxMessageSize bounds the total decoded payload of a single
	// (potentially fragmented) message. Zero defaults to
	// DefaultMaxMessageSize. A negative value disables the limit.
	MaxMessageSize int64

	// CheckOrigin authorises the Origin header during the handshake.
	// Returning false fails the upgrade with HTTP 403. nil accepts every
	// origin and should not be used for browser-facing deployments.
	CheckOrigin func(r *http.Request) bool

	// OnConnect is invoked when the first ConnectionInit message arrives.
	// It receives the request-scoped context and the raw init payload
	// (typically containing auth tokens). Implementations may either:
	//
	//  - Return a derived context (used as the parent for every
	//    subscription on the connection) and an optional ack payload.
	//    A nil ack payload omits the payload from the ack message.
	//
	//  - Return an error to reject the connection. The socket is closed
	//    with code 4403 Forbidden.
	OnConnect func(ctx context.Context, payload json.RawMessage) (context.Context, json.RawMessage, error)

	// PingInterval, when non-zero, makes the server send application-level
	// ping messages on this cadence. The peer is expected to respond with
	// pong; absence of pong combined with ReadIdleTimeout causes the
	// connection to close.
	PingInterval time.Duration
}

func (c Config) maxMessageSize() int64 {
	switch {
	case c.MaxMessageSize < 0:
		return 0
	case c.MaxMessageSize == 0:
		return DefaultMaxMessageSize
	default:
		return c.MaxMessageSize
	}
}

func (c Config) connectionInitTimeout() time.Duration {
	switch {
	case c.ConnectionInitTimeout < 0:
		return 0
	case c.ConnectionInitTimeout == 0:
		return DefaultConnectionInitTimeout
	default:
		return c.ConnectionInitTimeout
	}
}

// Transport implements core.Transport for the graphql-transport-ws
// subprotocol. Construct with New; the zero value also works and applies
// all defaults.
type Transport struct {
	config Config
}

// New returns a Transport ready to be registered with the server.
func New(cfg ...Config) *Transport {
	t := &Transport{}
	if len(cfg) > 0 {
		t.config = cfg[0]
	}
	return t
}

// Match reports whether r is a WebSocket upgrade request.
func (Transport) Match(r *http.Request) bool { return IsUpgrade(r) }

// Serve performs the WebSocket handshake, verifies that the negotiated
// subprotocol is graphql-transport-ws, and runs the dispatcher loop for the
// lifetime of the session.
func (t *Transport) Serve(w http.ResponseWriter, r *http.Request, executor core.Executor) {
	conn, err := upgrade(w, r, upgradeOptions{
		subprotocols:   []string{Subprotocol},
		checkOrigin:    t.config.CheckOrigin,
		maxMessageSize: t.config.maxMessageSize(),
		writeTimeout:   t.config.WriteTimeout,
	})
	if err != nil {
		return
	}
	defer conn.Close()

	// The dispatcher derives its own background context for subscription
	// resolvers so they remain alive for the duration of the WebSocket
	// session. Net-level disconnect (TCP close, read timeout, etc.) makes
	// ReadMessage fail and the dispatcher's deferred cleanup cancels every
	// in-flight subscription.
	//
	// We deliberately do not chain to r.Context(): Go's HTTP server may
	// cancel the request context immediately after the connection is
	// hijacked, which would falsely terminate every long-lived subscription
	// before it could deliver its first response.
	connCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if conn.Subprotocol() != Subprotocol {
		conn.SendClose(closeUnsupported, "subprotocol not supported")
		return
	}
	newDispatcher(conn, executor, t.config).run(connCtx)
}
