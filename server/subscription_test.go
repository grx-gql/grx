package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/pkg/sse"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/schema"
)

// streamingExecutor is a tiny core.Executor used by the subscription transport
// tests so we can deterministically control the responses emitted by the
// server without spinning up the full reflection-based executor.
type streamingExecutor struct {
	t       *testing.T
	source  chan core.Response
	subErr  error
	gotReq  core.Request
	subOnce sync.Once
}

func newStreamingExecutor(t *testing.T) *streamingExecutor {
	return &streamingExecutor{t: t, source: make(chan core.Response, 8)}
}

func (e *streamingExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	return core.Response{Data: map[string]any{"executed": true}}
}

func (e *streamingExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	switch {
	case strings.HasPrefix(strings.TrimSpace(req.Query), "subscription"):
		return core.OperationSubscription, nil
	case strings.HasPrefix(strings.TrimSpace(req.Query), "mutation"):
		return core.OperationMutation, nil
	default:
		return core.OperationQuery, nil
	}
}

func (e *streamingExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	e.subOnce.Do(func() { e.gotReq = req })
	if e.subErr != nil {
		return nil, e.subErr
	}
	out := make(chan core.Response)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case res, open := <-e.source:
				if !open {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}
	}()
	return out, nil
}

func newSubscriptionServer(t *testing.T, transports ...core.Transport) (*Server, *streamingExecutor) {
	t.Helper()
	executor := newStreamingExecutor(t)
	return &Server{
		executor:         executor,
		PlaygroundPath:   "/playground",
		GraphqlPath:      "/graphql",
		SubscriptionPath: "/graphql",
		separateSubs:     false,
		mainChain:        transports,
		subChain:         nil,
	}, executor
}

func TestSSEStreamsSubscriptionPayloads(t *testing.T) {
	srv, executor := newSubscriptionServer(t, sse.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	body := strings.NewReader(`{"query":"subscription { userCreated { id } }"}`)
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/graphql", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch SSE request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", got)
	}

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "2"}}}
	close(executor.source)

	events := readSSEEvents(t, response.Body, 3)

	if events[0].Event != "next" {
		t.Fatalf("expected first event next, got %q", events[0].Event)
	}
	first := decodeJSON(t, events[0].Data)
	if id := nestedValue(t, first, "data", "userCreated", "id"); id != "1" {
		t.Fatalf("expected first id 1, got %#v", id)
	}

	second := decodeJSON(t, events[1].Data)
	if id := nestedValue(t, second, "data", "userCreated", "id"); id != "2" {
		t.Fatalf("expected second id 2, got %#v", id)
	}

	if events[2].Event != "complete" {
		t.Fatalf("expected complete event, got %q", events[2].Event)
	}
}

func TestSSESupportsGetWithQueryParameters(t *testing.T) {
	srv, executor := newSubscriptionServer(t, sse.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	values := url.Values{
		"query":     {`subscription { userCreated { id } }`},
		"variables": {`{"limit":3}`},
	}
	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch SSE GET: %v", err)
	}
	defer response.Body.Close()

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	close(executor.source)

	events := readSSEEvents(t, response.Body, 2)

	if events[0].Event != "next" || events[1].Event != "complete" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if executor.gotReq.Query != `subscription { userCreated { id } }` {
		t.Fatalf("unexpected query in executor: %q", executor.gotReq.Query)
	}
	if executor.gotReq.Variables["limit"] != float64(3) {
		t.Fatalf("expected variables limit 3, got %#v", executor.gotReq.Variables)
	}
}

func TestSSEDisabledByDefault(t *testing.T) {
	srv, _ := newSubscriptionServer(t)
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when SSE disabled, got %d", response.StatusCode)
	}
}

func TestWebSocketStreamsSubscriptionPayloads(t *testing.T) {
	srv, executor := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	if got := readServerJSON(t, conn); got["type"] != "connection_ack" {
		t.Fatalf("expected connection_ack, got %#v", got)
	}

	writeClientText(t, conn, `{"id":"sub-1","type":"subscribe","payload":{"query":"subscription { userCreated { id } }"}}`)

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "2"}}}

	first := readServerJSON(t, conn)
	if first["type"] != "next" || first["id"] != "sub-1" {
		t.Fatalf("expected next message, got %#v", first)
	}
	if id := nestedValue(t, first, "payload", "data", "userCreated", "id"); id != "1" {
		t.Fatalf("expected first id 1, got %#v", id)
	}

	second := readServerJSON(t, conn)
	if id := nestedValue(t, second, "payload", "data", "userCreated", "id"); id != "2" {
		t.Fatalf("expected second id 2, got %#v", id)
	}

	close(executor.source)

	complete := readServerJSON(t, conn)
	if complete["type"] != "complete" || complete["id"] != "sub-1" {
		t.Fatalf("expected complete, got %#v", complete)
	}
}

func TestWebSocketRejectsSubscribeBeforeInit(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"id":"1","type":"subscribe","payload":{"query":"subscription { userCreated { id } }"}}`)

	opcode, _, _, err := readServerFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close frame, got err=%v", err)
	}
	if opcode != websocket.OpcodeClose {
		t.Fatalf("expected close opcode, got %d", opcode)
	}
}

func TestWebSocketRespondsToPing(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	_ = readServerJSON(t, conn)

	writeClientText(t, conn, `{"type":"ping"}`)
	if got := readServerJSON(t, conn); got["type"] != "pong" {
		t.Fatalf("expected pong, got %#v", got)
	}
}

func TestWebSocketDisabledByDefault(t *testing.T) {
	srv, _ := newSubscriptionServer(t)
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	request.Header.Set("Sec-WebSocket-Protocol", websocket.Subprotocol)

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when WS disabled, got %d", response.StatusCode)
	}
}

func TestWebSocketConnectionInitTimeoutCloses4408(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New(websocket.Config{
		ConnectionInitTimeout: 100 * time.Millisecond,
	}))
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	opcode, payload, _, err := readServerFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close frame, got err=%v", err)
	}
	if opcode != websocket.OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
	if len(payload) < 2 {
		t.Fatalf("close frame missing status code")
	}
	code := binary.BigEndian.Uint16(payload[:2])
	if code != 4408 {
		t.Fatalf("expected close code 4408, got %d", code)
	}
}

func TestWebSocketOnConnectAuthorizesAndRejects(t *testing.T) {
	authorize := func(ctx context.Context, payload json.RawMessage) (context.Context, json.RawMessage, error) {
		var creds struct {
			Token string `json:"token"`
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &creds)
		}
		if creds.Token != "letmein" {
			return nil, nil, errorString("invalid token")
		}
		return ctx, json.RawMessage(`{"user":"alice"}`), nil
	}

	srv, _ := newSubscriptionServer(t, websocket.New(websocket.Config{
		OnConnect: authorize,
	}))
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	t.Run("rejects bad token", func(t *testing.T) {
		conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
		defer conn.Close()

		writeClientText(t, conn, `{"type":"connection_init","payload":{"token":"nope"}}`)

		opcode, payload, _, err := readServerFrame(t, conn)
		if err != nil {
			t.Fatalf("expected close, got err=%v", err)
		}
		if opcode != websocket.OpcodeClose {
			t.Fatalf("expected close opcode, got 0x%X", opcode)
		}
		if code := binary.BigEndian.Uint16(payload[:2]); code != 4403 {
			t.Fatalf("expected close code 4403, got %d", code)
		}
	})

	t.Run("accepts good token and emits ack payload", func(t *testing.T) {
		conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
		defer conn.Close()

		writeClientText(t, conn, `{"type":"connection_init","payload":{"token":"letmein"}}`)

		ack := readServerJSON(t, conn)
		if ack["type"] != "connection_ack" {
			t.Fatalf("expected connection_ack, got %#v", ack)
		}
		ackPayload, ok := ack["payload"].(map[string]any)
		if !ok {
			t.Fatalf("expected ack payload, got %#v", ack)
		}
		if ackPayload["user"] != "alice" {
			t.Fatalf("expected alice, got %#v", ackPayload)
		}
	})
}

func TestWebSocketRejectsSubscribeWithEmptyQuery(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	_ = readServerJSON(t, conn)

	writeClientText(t, conn, `{"id":"sub-1","type":"subscribe","payload":{"query":""}}`)

	first := readServerJSON(t, conn)
	if first["type"] != "error" {
		t.Fatalf("expected error message, got %#v", first)
	}
	second := readServerJSON(t, conn)
	if second["type"] != "complete" {
		t.Fatalf("expected complete, got %#v", second)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestNewServerBuildsSubscriptionRoot(t *testing.T) {
	subscription := schemaSubscriptionRoot{}
	srv, err := New(Config{
		Schema: schema.Config{
			Query:        schemaQueryRoot{},
			Subscription: subscription,
		},
		Plugins:    []plugin.Plugin{},
		Transports: []core.Transport{websocket.New(), sse.New()},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	// The server appends a default HTTP+JSON transport to the chain, so
	// the user-supplied two transports become three after construction.
	if len(srv.mainChain) != 3 {
		t.Fatalf("expected 3 transports (websocket, sse, default http), got %d", len(srv.mainChain))
	}
}

type schemaQueryRoot struct{}

func (schemaQueryRoot) Hello() string { return "hi" }

type schemaSubscriptionRoot struct{}

func (schemaSubscriptionRoot) Hello(ctx context.Context) (<-chan string, error) {
	out := make(chan string)
	close(out)
	return out, nil
}

// --- helpers -----------------------------------------------------------

type sseEvent struct {
	Event string
	Data  string
}

func readSSEEvents(t *testing.T, body io.Reader, count int) []sseEvent {
	t.Helper()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	events := make([]sseEvent, 0, count)
	current := sseEvent{}
	deadline := time.Now().Add(5 * time.Second)
	for len(events) < count {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for SSE events; got %#v", events)
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan SSE: %v", err)
			}
			break
		}
		line := scanner.Text()
		switch {
		case line == "":
			if current.Event != "" || current.Data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		case strings.HasPrefix(line, "event: "):
			current.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			value := strings.TrimPrefix(line, "data: ")
			if current.Data == "" {
				current.Data = value
			} else {
				current.Data += "\n" + value
			}
		}
	}
	return events
}

func decodeJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode json %q: %v", raw, err)
	}
	return value
}

func nestedValue(t *testing.T, value map[string]any, keys ...string) any {
	t.Helper()
	current := any(value)
	for _, key := range keys {
		mapValue, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map at key path %v, got %T", keys, current)
		}
		current = mapValue[key]
	}
	return current
}

func dialWebSocket(t *testing.T, baseURL string, subprotocol string) net.Conn {
	return dialWebSocketAt(t, baseURL, "/graphql", subprotocol)
}

func dialWebSocketAt(t *testing.T, baseURL string, requestPath string, subprotocol string) net.Conn {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("random key: %v", err)
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	request := strings.Join([]string{
		"GET " + requestPath + " HTTP/1.1",
		"Host: " + parsed.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Version: 13",
		"Sec-WebSocket-Key: " + key,
		"Sec-WebSocket-Protocol: " + subprotocol,
		"", "",
	}, "\r\n")

	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("send handshake: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 switching protocols, got %q", statusLine)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	wrapped := &readerConn{Conn: conn, reader: reader}
	return wrapped
}

type readerConn struct {
	net.Conn
	reader *bufio.Reader
}

func (r *readerConn) Read(b []byte) (int, error) {
	return r.reader.Read(b)
}

func writeClientText(t *testing.T, conn net.Conn, payload string) {
	t.Helper()

	body := []byte(payload)
	header := []byte{0x81} // FIN + text
	length := len(body)
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
	mask := []byte{0xA1, 0xB2, 0xC3, 0xD4}
	frame := append([]byte{}, header...)
	frame = append(frame, mask...)
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = body[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func readServerFrame(t *testing.T, conn net.Conn) (opcode byte, payload []byte, fin bool, err error) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return 0, nil, false, err
	}
	fin = buffer[0]&0x80 != 0
	opcode = buffer[0] & 0x0F
	masked := buffer[1]&0x80 != 0
	length := uint64(buffer[1] & 0x7F)
	switch length {
	case 126:
		extra := make([]byte, 2)
		if _, err := io.ReadFull(conn, extra); err != nil {
			return 0, nil, false, err
		}
		length = uint64(binary.BigEndian.Uint16(extra))
	case 127:
		extra := make([]byte, 8)
		if _, err := io.ReadFull(conn, extra); err != nil {
			return 0, nil, false, err
		}
		length = binary.BigEndian.Uint64(extra)
	}
	if masked {
		mask := make([]byte, 4)
		if _, err := io.ReadFull(conn, mask); err != nil {
			return 0, nil, false, err
		}
		payload = make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false, err
		}
		for index := range payload {
			payload[index] ^= mask[index%4]
		}
		return opcode, payload, fin, nil
	}
	payload = make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false, err
		}
	}
	return opcode, payload, fin, nil
}

func readServerJSON(t *testing.T, conn net.Conn) map[string]any {
	t.Helper()
	for {
		opcode, payload, _, err := readServerFrame(t, conn)
		if err != nil {
			t.Fatalf("read server frame: %v", err)
		}
		if opcode == websocket.OpcodeText {
			var message map[string]any
			if err := json.Unmarshal(payload, &message); err != nil {
				t.Fatalf("decode json %q: %v", string(payload), err)
			}
			return message
		}
		if opcode == websocket.OpcodeClose {
			t.Fatalf("server closed connection: %s", string(payload))
		}
	}
}
