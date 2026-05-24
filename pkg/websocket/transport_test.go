package websocket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
)

type wsCoverExecutor struct {
	stream       chan core.Response
	subscribeErr error
}

func (e *wsCoverExecutor) Execute(context.Context, core.Request) core.Response {
	return core.Response{Data: map[string]any{"ok": true}}
}

func (e *wsCoverExecutor) Subscribe(context.Context, core.Request) (<-chan core.Response, error) {
	if e.subscribeErr != nil {
		return nil, e.subscribeErr
	}
	return e.stream, nil
}

func (e *wsCoverExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	if strings.Contains(req.Query, "bad") {
		return "", errors.New("bad operation")
	}
	if strings.HasPrefix(strings.TrimSpace(req.Query), "subscription") {
		return core.OperationSubscription, nil
	}
	return core.OperationQuery, nil
}

// hangSubscriberExecutor occupies a graphql-transport-ws subscription slot until
// the operation context is cancelled, without emitting events.
type hangSubscriberExecutor struct{}

func (hangSubscriberExecutor) Execute(context.Context, core.Request) core.Response {
	return core.Response{Data: map[string]any{"ok": true}}
}

func (hangSubscriberExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	q := strings.TrimSpace(req.Query)
	if strings.HasPrefix(q, "subscription") {
		return core.OperationSubscription, nil
	}
	return core.OperationQuery, nil
}

func (hangSubscriberExecutor) Subscribe(ctx context.Context, _ core.Request) (<-chan core.Response, error) {
	out := make(chan core.Response)
	go func() {
		<-ctx.Done()
		close(out)
	}()
	return out, nil
}

func TestTransportGraphQLWebSocketSession(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
	tr := New(Config{
		ConnectionInitTimeout: time.Second,
		ReadIdleTimeout:       time.Second,
		MaxSubscriptions:      2,
		OnConnect: func(ctx context.Context, payload json.RawMessage) (context.Context, json.RawMessage, error) {
			return ctx, json.RawMessage(`{"ready":true}`), nil
		},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init","payload":{"token":"ok"}}`))
	_, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	var ack ackMessage
	if err := json.Unmarshal(payload, &ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ack.Type != "connection_ack" || string(ack.Payload) != `{"ready":true}` {
		t.Fatalf("ack = %#v payload=%s", ack, ack.Payload)
	}

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping","payload":{"n":1}}`))
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var pong message
	if err := json.Unmarshal(payload, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("pong = %#v err=%v", pong, err)
	}

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"id":"1","type":"subscribe","payload":{"query":"{ ok }"}}`))
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatalf("read next: %v", err)
	}
	var next nextMessage
	if err := json.Unmarshal(payload, &next); err != nil || next.Type != "next" || next.ID != "1" {
		t.Fatalf("next = %#v err=%v payload=%s", next, err, payload)
	}
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatalf("read complete: %v", err)
	}
	var complete message
	if err := json.Unmarshal(payload, &complete); err != nil || complete.Type != "complete" || complete.ID != "1" {
		t.Fatalf("complete = %#v err=%v payload=%s", complete, err, payload)
	}

	executor.stream <- core.Response{Data: map[string]any{"event": true}}
	close(executor.stream)
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"id":"2","type":"subscribe","payload":{"query":"subscription { event }"}}`))
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatalf("read stream next: %v", err)
	}
	if err := json.Unmarshal(payload, &next); err != nil || next.Type != "next" || next.ID != "2" {
		t.Fatalf("stream next = %#v err=%v payload=%s", next, err, payload)
	}
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatalf("read stream complete: %v", err)
	}
	if err := json.Unmarshal(payload, &complete); err != nil || complete.Type != "complete" || complete.ID != "2" {
		t.Fatalf("stream complete = %#v err=%v payload=%s", complete, err, payload)
	}
}

func TestTransportGraphQLWebSocketProtocolErrors(t *testing.T) {
	cases := []struct {
		name    string
		config  Config
		frames  []string
		skip    int
		want    uint16
		wantMsg string
	}{
		{name: "unknown", frames: []string{`{"type":"bogus"}`}, want: wsCloseInvalidMessage},
		{name: "invalid json", frames: []string{`{`}, want: wsCloseInvalidMessage},
		{name: "unauthorized subscribe", frames: []string{`{"id":"1","type":"subscribe","payload":{"query":"{ ok }"}}`}, want: wsCloseUnauthorized},
		{name: "too many init", frames: []string{`{"type":"connection_init"}`, `{"type":"connection_init"}`}, skip: 1, want: wsCloseTooManyInits},
		{name: "forbidden connect", config: Config{OnConnect: func(context.Context, json.RawMessage) (context.Context, json.RawMessage, error) {
			return nil, nil, errors.New(strings.Repeat("x", 200))
		}}, frames: []string{`{"type":"connection_init"}`}, want: wsCloseForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
			tr := New(tc.config)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tr.Serve(w, r, executor)
			}))
			defer server.Close()

			conn := dialHandshake(t, server.URL, Subprotocol)
			defer conn.Close()
			for _, frame := range tc.frames {
				writeMaskedFrame(t, conn, OpcodeText, []byte(frame))
			}
			for range tc.skip {
				if _, _, _, err := readFrame(t, conn); err != nil {
					t.Fatalf("read skipped frame: %v", err)
				}
			}
			op, payload, _, err := readFrame(t, conn)
			if err != nil {
				t.Fatalf("read close: %v", err)
			}
			if op != OpcodeClose {
				t.Fatalf("opcode = %d payload=%s", op, payload)
			}
			if got := binary.BigEndian.Uint16(payload[:2]); got != tc.want {
				t.Fatalf("close code = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTransportServerPingAndHelpers(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
	tr := New(Config{PingInterval: time.Millisecond})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, err := readFrame(t, conn) // ack
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	_, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ping: %v", err)
	}
	var msg message
	if err := json.Unmarshal(payload, &msg); err != nil || msg.Type != "ping" {
		t.Fatalf("server ping = %#v err=%v payload=%s", msg, err, payload)
	}

	if IsUpgrade(httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Fatal("plain request is not an upgrade")
	}
	if sanitizeReason(strings.Repeat("x", 200)) == strings.Repeat("x", 200) {
		t.Fatal("reason should be truncated")
	}
	if IsTimeout(errors.New("x")) {
		t.Fatal("plain error is not timeout")
	}
	if New().Match(httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Fatal("plain request should not match websocket transport")
	}
	up := httptest.NewRequest(http.MethodGet, "/", nil)
	up.Header.Set("Connection", "keep-alive, Upgrade")
	up.Header.Set("Upgrade", "websocket")
	if !IsUpgrade(up) || !New().Match(up) {
		t.Fatal("expected websocket upgrade match")
	}
	if offersPermessageDeflate([]string{"x, permessage-deflate; client_max_window_bits"}) != true {
		t.Fatal("expected deflate offer")
	}
	if offersPermessageDeflate([]string{"x"}) {
		t.Fatal("unexpected deflate offer")
	}
	if (Config{}).maxMessageSize() != DefaultMaxMessageSize {
		t.Fatal("expected default max message size")
	}
	if (Config{}).connectionInitTimeout() != DefaultConnectionInitTimeout {
		t.Fatal("expected default init timeout")
	}
	if (Config{MaxMessageSize: -1}).maxMessageSize() != 0 {
		t.Fatal("expected disabled max message size")
	}
	if (Config{ConnectionInitTimeout: -1}).connectionInitTimeout() != 0 {
		t.Fatal("expected disabled init timeout")
	}
}

func TestTransportHandshakeCloseBranches(t *testing.T) {
	t.Run("unsupported subprotocol", func(t *testing.T) {
		executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
		tr := New()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr.Serve(w, r, executor)
		}))
		defer server.Close()

		conn := dialHandshake(t, server.URL, "graphql-ws")
		defer conn.Close()
		op, payload, _, err := readFrame(t, conn)
		if err != nil {
			t.Fatalf("read close: %v", err)
		}
		if op != OpcodeClose {
			t.Fatalf("opcode = %d payload=%s", op, payload)
		}
		if got := binary.BigEndian.Uint16(payload[:2]); got != closeUnsupported {
			t.Fatalf("close code = %d, want %d", got, closeUnsupported)
		}
	})

	t.Run("connection limit", func(t *testing.T) {
		executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
		tr := New(Config{MaxConnections: 1})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr.Serve(w, r, executor)
		}))
		defer server.Close()

		first := dialHandshake(t, server.URL, Subprotocol)
		defer first.Close()
		second := dialHandshake(t, server.URL, Subprotocol)
		defer second.Close()

		op, payload, _, err := readFrame(t, second)
		if err != nil {
			t.Fatalf("read close: %v", err)
		}
		if op != OpcodeClose {
			t.Fatalf("opcode = %d payload=%s", op, payload)
		}
		if got := binary.BigEndian.Uint16(payload[:2]); got != closeTryAgainLater {
			t.Fatalf("close code = %d, want %d", got, closeTryAgainLater)
		}
	})
}

type wsNetTimeoutErr struct{}

func (wsNetTimeoutErr) Error() string   { return "timeout" }
func (wsNetTimeoutErr) Timeout() bool   { return true }
func (wsNetTimeoutErr) Temporary() bool { return false }

func TestCoverDeadlineClassificationAndPayloadLengthIndicator(t *testing.T) {
	if !IsTimeout(os.ErrDeadlineExceeded) {
		t.Fatal("expected os.ErrDeadlineExceeded to be a timeout")
	}
	wrapped := fmt.Errorf("wrapped: %w", os.ErrDeadlineExceeded)
	if !IsTimeout(wrapped) {
		t.Fatal("expected errors.Is traversal for deadlines")
	}
	if !IsTimeout(wsNetTimeoutErr{}) {
		t.Fatal("expected net.Error.Timeout branch")
	}
	if _, err := readPayloadLength(bufio.NewReader(bytes.NewReader(nil)), 200); err == nil || !strings.Contains(err.Error(), "invalid payload indicator") {
		t.Fatalf("expected invalid-length indicator error, got %v", err)
	}
	if sanitizeReason("") != "" {
		t.Fatal("sanitized empty reason should remain empty")
	}
}

func TestTransportConnectionInitDeadlineClosesHandshake(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
	tr := New(Config{ConnectionInitTimeout: 150 * time.Millisecond})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	op, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read close: %v", err)
	}
	if op != OpcodeClose || len(payload) < 2 {
		t.Fatalf("expected close frame with code, got op=%x payload=%x", op, payload)
	}
	if got := binary.BigEndian.Uint16(payload[:2]); got != wsCloseConnectionInitTimeout {
		t.Fatalf("close code = %d, want connection init timeout", got)
	}
}

func TestTransportSkipsNonTextFramesUntilPing(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
	tr := New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	writeMaskedFrame(t, conn, OpcodeBinary, []byte{0x42})
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping"}`))

	_, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read pong after binary: %v", err)
	}
	var pong message
	if err := json.Unmarshal(payload, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("pong payload = %#v err=%v", pong, err)
	}
}

func TestTransportAcceptsClientPongBeforeHeartbeat(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response, 1)}
	tr := New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"pong"}`))
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping"}`))

	_, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read pong after client pong cycle: %v", err)
	}
	var pong message
	if err := json.Unmarshal(payload, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("expected responding pong payload, got %#v err=%v", pong, err)
	}
}

func TestTransportDuplicateSubscriptionIDClosesConnection(t *testing.T) {
	stream := make(chan core.Response)
	executor := &wsCoverExecutor{stream: stream}
	tr := New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	dup := []byte(`{"id":"same","type":"subscribe","payload":{"query":"subscription { event }"}}`)
	writeMaskedFrame(t, conn, OpcodeText, dup)
	writeMaskedFrame(t, conn, OpcodeText, dup)

	op, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read close: %v", err)
	}
	if op != OpcodeClose {
		t.Fatalf("expected close frame after duplicate id, got opcode 0x%x", op)
	}
	if got := binary.BigEndian.Uint16(payload[:2]); got != wsCloseSubscriberAlreadyExists {
		t.Fatalf("close code = %d, want %d (subscriber exists)", got, wsCloseSubscriberAlreadyExists)
	}
	close(stream)
}

func TestTransportSubscribePropagatesSubscribeError(t *testing.T) {
	stream := make(chan core.Response)
	executor := &wsCoverExecutor{
		stream:       stream,
		subscribeErr: errors.New("subscribe refused"),
	}
	tr := New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"id":"sub","type":"subscribe","payload":{"query":"subscription { event }"}}`))

	_, errPayload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	var em errorMessage
	if json.Unmarshal(errPayload, &em) != nil || em.Type != "error" || len(em.Payload) == 0 {
		t.Fatalf("expected error envelope, raw=%s", errPayload)
	}
	if got := em.Payload[0].Message; !strings.Contains(got, "subscribe refused") {
		t.Fatalf("error message %q missing subscribe error", got)
	}
	var completeMsg message
	_, cp, _, err := readFrame(t, conn)
	if err != nil || json.Unmarshal(cp, &completeMsg) != nil || completeMsg.Type != "complete" || completeMsg.ID != "sub" {
		t.Fatalf("expected complete for sub, payload=%s err=%v", cp, err)
	}
	close(stream)
}

func TestTransportRejectSubscriptionPastMaxActive(t *testing.T) {
	executor := hangSubscriberExecutor{}
	tr := New(Config{MaxSubscriptions: 2})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	if _, _, _, err := readFrame(t, conn); err != nil {
		t.Fatalf("read ack: %v", err)
	}

	payload := func(id string) string {
		return fmt.Sprintf(`{"id":"%s","type":"subscribe","payload":{"query":"subscription { hang }}"}}`, id)
	}
	writeMaskedFrame(t, conn, OpcodeText, []byte(payload("one")))
	writeMaskedFrame(t, conn, OpcodeText, []byte(payload("two")))
	time.Sleep(20 * time.Millisecond)
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"id":"three","type":"subscribe","payload":{"query":"{ ok }"}}`))

	_, emRaw, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read third-subscription error frame: %v", err)
	}
	var limErr errorMessage
	if json.Unmarshal(emRaw, &limErr) != nil || limErr.Type != "error" || limErr.ID != "three" ||
		len(limErr.Payload) == 0 || !strings.Contains(limErr.Payload[0].Message, "subscription limit") {
		t.Fatalf("unexpected limit payload: %+v raw=%s", limErr, emRaw)
	}
	var compl message
	if _, complRaw, _, err := readFrame(t, conn); err != nil {
		t.Fatalf("read complete: %v", err)
	} else if json.Unmarshal(complRaw, &compl) != nil || compl.Type != "complete" || compl.ID != "three" {
		t.Fatalf("expected complete three, got %+v (%s)", compl, complRaw)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("conn close: %v", err)
	}
}

func TestTransportAckWithoutOptionalPayloadBody(t *testing.T) {
	executor := &wsCoverExecutor{stream: make(chan core.Response)}
	tr := New(Config{OnConnect: func(ctx context.Context, payload json.RawMessage) (context.Context, json.RawMessage, error) {
		return ctx, nil, nil
	}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, executor)
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()

	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, raw, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	var ack map[string]any
	if json.Unmarshal(raw, &ack) != nil {
		t.Fatalf("decode ack JSON: %s", raw)
	}
	if ack["type"] != "connection_ack" {
		t.Fatalf("ack type = %#v", ack["type"])
	}
	if _, has := ack["payload"]; has {
		t.Fatalf("did not expect ack payload body, got %#v", ack)
	}
}

func TestTransportMaxConnectionsLimit(t *testing.T) {
	tr := &Transport{config: Config{MaxConnections: 1}}
	c1, c2 := &Conn{}, &Conn{}

	if !tr.register(c1) {
		t.Fatal("first connection should register")
	}
	if tr.register(c2) {
		t.Fatal("second connection should be rejected by the limit")
	}
	tr.unregister(c1)
	if !tr.register(c2) {
		t.Fatal("connection should register after a slot frees up")
	}
	tr.unregister(c2)
}

func TestTransportShutdownDrainsWithGoingAway(t *testing.T) {
	tr := &Transport{}
	client, server := net.Pipe()
	sc := &Conn{conn: server, reader: bufio.NewReader(server)}
	if !tr.register(sc) {
		t.Fatal("register failed")
	}

	gotCode := make(chan uint16, 1)
	go func() {
		op, payload, _, err := readFrame(t, client)
		if err == nil && op == OpcodeClose && len(payload) >= 2 {
			gotCode <- binary.BigEndian.Uint16(payload[:2])
		} else {
			gotCode <- 0
		}
		_ = client.Close()
	}()
	go func() {
		buf := make([]byte, 1)
		_, _ = server.Read(buf) // unblocks when Shutdown closes the conn
		tr.unregister(sc)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tr.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	select {
	case code := <-gotCode:
		if code != closeGoingAway {
			t.Fatalf("close code = %d, want %d (1001 Going Away)", code, closeGoingAway)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not observe close frame")
	}

	// Draining must refuse new connections.
	if tr.register(&Conn{}) {
		t.Fatal("register should fail while draining")
	}
}

func TestPermessageDeflateRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrade(w, r, upgradeOptions{
			subprotocols:      []string{"any"},
			maxMessageSize:    DefaultMaxMessageSize,
			permessageDeflate: true,
		})
		if err != nil {
			return
		}
		defer conn.Close()
		op, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if op == OpcodeText {
			_ = conn.WriteText(payload)
		}
	}))
	defer server.Close()

	conn, negotiated := dialDeflateHandshake(t, server.URL)
	defer conn.Close()
	if !negotiated {
		t.Fatal("server did not negotiate permessage-deflate")
	}

	message := []byte(strings.Repeat("graphql-", 64)) // compresses well
	writeMaskedCompressedText(t, conn, message)

	rsv1, payload := readReplyFrame(t, conn)
	if !rsv1 {
		t.Fatal("expected server reply to set RSV1 (compressed)")
	}
	got, err := inflate(payload, DefaultMaxMessageSize)
	if err != nil {
		t.Fatalf("inflate reply: %v", err)
	}
	if string(got) != string(message) {
		t.Fatalf("round-trip mismatch: got %q", got)
	}
}

// dialDeflateHandshake performs a handshake advertising permessage-deflate and
// reports whether the server accepted it.

func dialDeflateHandshake(t *testing.T, baseURL string) (net.Conn, bool) {
	t.Helper()
	parsed, _ := url.Parse(baseURL)
	conn, err := net.DialTimeout("tcp", parsed.Host, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	request := strings.Join([]string{
		"GET / HTTP/1.1",
		"Host: " + parsed.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Version: 13",
		"Sec-WebSocket-Key: " + key,
		"Sec-WebSocket-Protocol: any",
		"Sec-WebSocket-Extensions: permessage-deflate",
		"", "",
	}, "\r\n")
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101, got %q (%v)", statusLine, err)
	}
	negotiated := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-extensions:") &&
			strings.Contains(strings.ToLower(line), "permessage-deflate") {
			negotiated = true
		}
	}
	return &readerConn{Conn: conn, reader: reader}, negotiated
}

func writeMaskedCompressedText(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	compressed, err := deflate(payload)
	if err != nil {
		t.Fatalf("deflate: %v", err)
	}
	header := []byte{0x80 | 0x40 | OpcodeText} // FIN + RSV1 + text
	length := len(compressed)
	maskBit := byte(0x80)
	switch {
	case length < 126:
		header = append(header, maskBit|byte(length))
	case length <= 0xFFFF:
		header = append(header, maskBit|126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(length))
	default:
		header = append(header, maskBit|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(length))
	}
	mask := []byte{0x11, 0x22, 0x33, 0x44}
	frame := append([]byte{}, header...)
	frame = append(frame, mask...)
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = compressed[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write compressed frame: %v", err)
	}
}

// readReplyFrame reads one server frame, returning whether RSV1 was set and the
// raw (still-compressed) payload.

func readReplyFrame(t *testing.T, conn net.Conn) (bool, []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	head := make([]byte, 2)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read header: %v", err)
	}
	rsv1 := head[0]&0x40 != 0
	length := uint64(head[1] & 0x7F)
	switch length {
	case 126:
		extra := make([]byte, 2)
		if _, err := io.ReadFull(conn, extra); err != nil {
			t.Fatalf("read len16: %v", err)
		}
		length = uint64(binary.BigEndian.Uint16(extra))
	case 127:
		extra := make([]byte, 8)
		if _, err := io.ReadFull(conn, extra); err != nil {
			t.Fatalf("read len64: %v", err)
		}
		length = binary.BigEndian.Uint64(extra)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return rsv1, payload
}
