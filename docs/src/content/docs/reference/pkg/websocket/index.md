---
title: pkg/websocket
description: API reference for the pkg/websocket package, generated from Go doc comments.
tableOfContents:
  minHeadingLevel: 2
  maxHeadingLevel: 4
editUrl: false
---



```go
import "github.com/patrickkabwe/grx/pkg/websocket"
```

Package websocket provides an RFC 6455 WebSocket implementation tailored for GraphQL subscription transports. It implements core.Transport for both the modern graphql\-transport\-ws subprotocol and the legacy graphql\-ws \(Apollo subscriptions\-transport\-ws\) subprotocol.

## Index

- [Constants](<#constants>)
- [func IsTimeout\(err error\) bool](<#IsTimeout>)
- [func IsUpgrade\(r \*http.Request\) bool](<#IsUpgrade>)
- [type Config](<#Config>)
- [type Conn](<#Conn>)
  - [func \(c \*Conn\) Close\(\) error](<#Conn.Close>)
  - [func \(c \*Conn\) ReadMessage\(\) \(opcode byte, payload \[\]byte, err error\)](<#Conn.ReadMessage>)
  - [func \(c \*Conn\) SendClose\(code uint16, reason string\)](<#Conn.SendClose>)
  - [func \(c \*Conn\) SetReadDeadline\(t time.Time\) error](<#Conn.SetReadDeadline>)
  - [func \(c \*Conn\) Subprotocol\(\) string](<#Conn.Subprotocol>)
  - [func \(c \*Conn\) WriteText\(payload \[\]byte\) error](<#Conn.WriteText>)
- [type Transport](<#Transport>)
  - [func New\(cfg ...Config\) \*Transport](<#New>)
  - [func \(Transport\) Match\(r \*http.Request\) bool](<#Transport.Match>)
  - [func \(t \*Transport\) Serve\(w http.ResponseWriter, r \*http.Request, executor core.Executor\)](<#Transport.Serve>)


## Constants

<a name="OpcodeContinuation"></a>Frame opcodes from RFC 6455 §5.2. Exported so callers can inspect the frames produced by the transport.

```go
const (
    OpcodeContinuation byte = 0x0
    OpcodeText         byte = 0x1
    OpcodeBinary       byte = 0x2
    OpcodeClose        byte = 0x8
    OpcodePing         byte = 0x9
    OpcodePong         byte = 0xA
)
```

<a name="DefaultMaxMessageSize"></a>Default values applied when the corresponding Config field is left zero.

```go
const (
    DefaultMaxMessageSize        = 1 << 20 // 1 MiB
    DefaultConnectionInitTimeout = 3 * time.Second
)
```

<a name="Subprotocol"></a>Subprotocol is the only WebSocket subprotocol this transport speaks. The legacy Apollo subscriptions\-transport\-ws \("graphql\-ws"\) protocol is intentionally not supported; it was deprecated in 2021 and clients that have not migrated should upgrade rather than be carried indefinitely.

```go
const Subprotocol = "graphql-transport-ws"
```

<a name="IsTimeout"></a>
## func IsTimeout

```go
func IsTimeout(err error) bool
```

IsTimeout reports whether err originated from a deadline exceeded on the underlying connection. The dispatcher uses this to map a missed connection\_init into the correct close code.

<a name="IsUpgrade"></a>
## func IsUpgrade

```go
func IsUpgrade(r *http.Request) bool
```

IsUpgrade reports whether r is a well\-formed WebSocket upgrade request.

<a name="Config"></a>
## type Config

Config tunes a WebSocket Transport. All fields are optional; zero values fall back to safe production defaults documented per field.

```go
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
```

<a name="Conn"></a>
## type Conn

Conn is a server\-side WebSocket connection. Reads are single\-threaded and happen on the dispatcher goroutine; writes are guarded by a mutex so concurrent goroutines \(e.g. subscription dispatchers, server\-side ping\) can push frames safely.

```go
type Conn struct {
    // contains filtered or unexported fields
}
```

<a name="Conn.Close"></a>
### func \(\*Conn\) Close

```go
func (c *Conn) Close() error
```

Close terminates the underlying network connection.

<a name="Conn.ReadMessage"></a>
### func \(\*Conn\) ReadMessage

```go
func (c *Conn) ReadMessage() (opcode byte, payload []byte, err error)
```

ReadMessage reads the next text or binary message from the peer, reassembling fragmented frames and responding to ping and close control frames automatically. It enforces the RFC 6455 framing rules: control frames must be unfragmented and ≤125 bytes, reserved bits must be zero, continuation frames must follow an in\-flight data message, and text messages must be valid UTF\-8.

<a name="Conn.SendClose"></a>
### func \(\*Conn\) SendClose

```go
func (c *Conn) SendClose(code uint16, reason string)
```

SendClose sends a WebSocket close frame with the given code and reason and marks the connection as closed for further writes.

<a name="Conn.SetReadDeadline"></a>
### func \(\*Conn\) SetReadDeadline

```go
func (c *Conn) SetReadDeadline(t time.Time) error
```

SetReadDeadline sets the absolute time after which a pending read will fail. A zero value disables the deadline. Dispatchers use it to enforce the connection\_init timeout and the read idle timeout.

<a name="Conn.Subprotocol"></a>
### func \(\*Conn\) Subprotocol

```go
func (c *Conn) Subprotocol() string
```

Subprotocol returns the negotiated WebSocket subprotocol, or the empty string if no subprotocol was agreed upon.

<a name="Conn.WriteText"></a>
### func \(\*Conn\) WriteText

```go
func (c *Conn) WriteText(payload []byte) error
```

WriteText writes a complete UTF\-8 text frame to the peer. Concurrent writes are serialised through the connection's mutex; the configured write deadline \(if any\) protects against slow consumers.

<a name="Transport"></a>
## type Transport

Transport implements core.Transport for the graphql\-transport\-ws subprotocol. Construct with New; the zero value also works and applies all defaults.

```go
type Transport struct {
    // contains filtered or unexported fields
}
```

<a name="New"></a>
### func New

```go
func New(cfg ...Config) *Transport
```

New returns a Transport ready to be registered with the server.

<a name="Transport.Match"></a>
### func \(Transport\) Match

```go
func (Transport) Match(r *http.Request) bool
```

Match reports whether r is a WebSocket upgrade request.

<a name="Transport.Serve"></a>
### func \(\*Transport\) Serve

```go
func (t *Transport) Serve(w http.ResponseWriter, r *http.Request, executor core.Executor)
```

Serve performs the WebSocket handshake, verifies that the negotiated subprotocol is graphql\-transport\-ws, and runs the dispatcher loop for the lifetime of the session.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
