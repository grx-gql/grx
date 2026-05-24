package websocket

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// These tests exercise the RFC 6455 framing layer in isolation by spinning
// up a tiny HTTP server that calls upgrade and then drives the resulting
// Conn directly. They focus on the hardening surface (UTF-8 validation,
// reserved bits, control frame validation, max-message limits) since the
// happy paths are already covered by the integration tests in `server`.

func TestIsUpgradeRequiresGetMethod(t *testing.T) {
	t.Parallel()

	post := httptest.NewRequest(http.MethodPost, "/", nil)
	post.Header.Set("Connection", "Upgrade")
	post.Header.Set("Upgrade", "websocket")
	if IsUpgrade(post) {
		t.Fatal("non-GET requests must not qualify as websocket upgrades")
	}
}

func TestConnReadBinaryUtf8BypassesUnicodeCheck(t *testing.T) {
	t.Parallel()

	client, srv := dialUpgraded(t, Config{}, OpcodeBinary, []byte(`payload`))
	defer srv.Close()
	defer client.Close()
}

func TestConnRejectsInvalidUTF8Text(t *testing.T) {
	t.Parallel()

	conn, server := dialUpgraded(t, Config{}, OpcodeText, []byte{0xC3, 0x28}) // invalid UTF-8 sequence
	defer server.Close()
	defer conn.Close()

	opcode, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close frame, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
}

func TestConnRejectsRSVBits(t *testing.T) {
	t.Parallel()

	conn, server := dialRaw(t, Config{}, func(c net.Conn) {
		// FIN + RSV1 + Text opcode, masked, length 1, mask 0,0,0,0, payload 0x41
		header := []byte{0xC1, 0x81, 0, 0, 0, 0, 0x41}
		if _, err := c.Write(header); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	})
	defer server.Close()
	defer conn.Close()

	opcode, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
}

func TestConnRejectsFragmentedControlFrame(t *testing.T) {
	t.Parallel()

	conn, server := dialRaw(t, Config{}, func(c net.Conn) {
		// Ping (op=0x9) without FIN bit, masked, empty payload
		header := []byte{0x09, 0x80, 0, 0, 0, 0}
		if _, err := c.Write(header); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	})
	defer server.Close()
	defer conn.Close()

	opcode, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
}

func TestConnRejectsOversizedControlFrame(t *testing.T) {
	t.Parallel()

	conn, server := dialRaw(t, Config{}, func(c net.Conn) {
		// Ping with 200-byte payload (> 125 limit)
		body := make([]byte, 200)
		header := []byte{0x89, 0x80 | 126, 0, 200, 0, 0, 0, 0}
		if _, err := c.Write(append(header, body...)); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	})
	defer server.Close()
	defer conn.Close()

	opcode, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
}

func TestConnEnforcesMaxMessageSize(t *testing.T) {
	t.Parallel()

	conn, server := dialUpgraded(t, Config{MaxMessageSize: 16}, OpcodeText, []byte(strings.Repeat("a", 32)))
	defer server.Close()
	defer conn.Close()

	opcode, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
	if len(payload) >= 2 {
		code := binary.BigEndian.Uint16(payload[:2])
		if code != closeMessageTooBig {
			t.Fatalf("expected close code 1009, got %d", code)
		}
	}
}

func TestConnRejectsContinuationWithoutDataFrame(t *testing.T) {
	t.Parallel()

	conn, server := dialRaw(t, Config{}, func(c net.Conn) {
		// FIN + Continuation, masked, empty payload
		header := []byte{0x80, 0x80, 0, 0, 0, 0}
		if _, err := c.Write(header); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	})
	defer server.Close()
	defer conn.Close()

	opcode, _, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close, got err=%v", err)
	}
	if opcode != OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
}

func TestUpgradeRejectsForbiddenOrigin(t *testing.T) {
	t.Parallel()

	called := false
	server := newUpgradeServer(t, Config{
		CheckOrigin: func(r *http.Request) bool {
			called = true
			return false
		},
	})
	defer server.Close()

	parsed, _ := url.Parse(server.URL)
	tcp, err := net.DialTimeout("tcp", parsed.Host, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tcp.Close()

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
		"Origin: http://evil.example.com",
		"", "",
	}, "\r\n")
	if _, err := tcp.Write([]byte(request)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(tcp)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(statusLine, "403") {
		t.Fatalf("expected 403, got %q", statusLine)
	}
	if !called {
		t.Fatalf("expected CheckOrigin to be called")
	}
}

func TestSubprotocolNegotiationPicksSupportedFirst(t *testing.T) {
	t.Parallel()
	got := negotiateSubprotocol([]string{"graphql-ws, graphql-transport-ws"}, []string{Subprotocol})
	if got != Subprotocol {
		t.Fatalf("expected %q, got %q", Subprotocol, got)
	}
	got = negotiateSubprotocol([]string{"graphql-ws"}, []string{Subprotocol})
	if got != "" {
		t.Fatalf("expected empty negotiation for unsupported subprotocol, got %q", got)
	}
	got = negotiateSubprotocol([]string{"unknown"}, []string{Subprotocol})
	if got != "" {
		t.Fatalf("expected empty negotiation, got %q", got)
	}
}

// --- helpers -----------------------------------------------------------

// newUpgradeServer returns an httptest.Server whose handler upgrades any
// inbound request and immediately spawns the supplied driver. It exists so
// the tests can drive the framing layer without instantiating the full
// graphql-transport-ws WebSocketTransport.
func newUpgradeServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrade(w, r, upgradeOptions{
			subprotocols:   []string{"any"},
			checkOrigin:    cfg.CheckOrigin,
			maxMessageSize: cfg.maxMessageSize(),
			writeTimeout:   cfg.WriteTimeout,
		})
		if err != nil {
			return
		}
		defer conn.Close()

		_, _, _ = conn.ReadMessage()
	}))
}

// dialUpgraded performs a real WebSocket handshake against newUpgradeServer
// and writes one client frame so the server's ReadMessage runs against the
// configured policy.
func dialUpgraded(t *testing.T, cfg Config, opcode byte, payload []byte) (net.Conn, *httptest.Server) {
	t.Helper()
	server := newUpgradeServer(t, cfg)
	conn := dialHandshake(t, server.URL, "any")
	writeMaskedFrame(t, conn, opcode, payload)
	return conn, server
}

// dialRaw performs the handshake then hands the raw client connection to
// the supplied driver, so tests can write malformed frames that break the
// framing rules before any decoding happens.
func dialRaw(t *testing.T, cfg Config, driver func(net.Conn)) (net.Conn, *httptest.Server) {
	t.Helper()
	server := newUpgradeServer(t, cfg)
	conn := dialHandshake(t, server.URL, "any")
	driver(conn)
	return conn, server
}

func dialHandshake(t *testing.T, baseURL, subprotocol string) net.Conn {
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
		"Sec-WebSocket-Protocol: " + subprotocol,
		"", "",
	}, "\r\n")
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write handshake: %v", err)
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
	return &readerConn{Conn: conn, reader: reader}
}

type readerConn struct {
	net.Conn
	reader *bufio.Reader
}

func (r *readerConn) Read(b []byte) (int, error) { return r.reader.Read(b) }

func writeMaskedFrame(t *testing.T, conn net.Conn, opcode byte, payload []byte) {
	t.Helper()
	header := []byte{0x80 | (opcode & 0x0F)}
	length := len(payload)
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
		masked[i] = payload[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func readFrame(t *testing.T, conn net.Conn) (opcode byte, payload []byte, fin bool, err error) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return 0, nil, false, err
	}
	fin = buffer[0]&0x80 != 0
	opcode = buffer[0] & 0x0F
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
	payload = make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false, err
		}
	}
	return opcode, payload, fin, nil
}

func TestUpgradeRejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name    string
		request *http.Request
		opts    upgradeOptions
		status  int
	}{
		{name: "plain", request: httptest.NewRequest(http.MethodGet, "/", nil), status: http.StatusOK},
		{name: "bad version", request: func() *http.Request {
			req := websocketRequest("bad", Subprotocol)
			req.Header.Set("Sec-WebSocket-Version", "12")
			return req
		}(), status: http.StatusUpgradeRequired},
		{name: "missing key", request: websocketRequest("", Subprotocol), status: http.StatusBadRequest},
		{name: "forbidden origin", request: websocketRequest("key", Subprotocol), opts: upgradeOptions{checkOrigin: func(*http.Request) bool { return false }}, status: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if _, err := upgrade(rec, tc.request, tc.opts); err == nil {
				t.Fatal("expected upgrade error")
			}
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestConnFrameLengthAndWriteBranches(t *testing.T) {
	lengths := []struct {
		indicator byte
		extra     []byte
		want      uint64
	}{
		{5, nil, 5},
		{126, []byte{1, 2}, 258},
		{127, []byte{0, 0, 0, 0, 0, 0, 1, 2}, 258},
	}
	for _, tc := range lengths {
		reader := bufio.NewReader(bytes.NewReader(tc.extra))
		got, err := readPayloadLength(reader, tc.indicator)
		if err != nil {
			t.Fatalf("read len: %v", err)
		}
		if got != tc.want {
			t.Fatalf("len = %d, want %d", got, tc.want)
		}
	}
	if _, err := readPayloadLength(bufio.NewReader(bytes.NewReader(nil)), 126); err == nil {
		t.Fatal("expected short length read error")
	}

	client, server := net.Pipe()
	conn := &Conn{conn: server, reader: bufio.NewReader(server), writeTimeout: time.Second}
	done := make(chan struct{})
	go func() {
		_, _, _, _ = readFrame(t, client)
		close(done)
	}()
	if err := conn.writeFrame(OpcodeText, []byte(strings.Repeat("x", 130))); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	<-done
	_ = conn.Close()
	_ = client.Close()
	if err := conn.writeFrame(OpcodeText, []byte("x")); err == nil {
		t.Fatal("expected closed write error")
	}
}

func TestConnReadMessageControlAndCompressionBranches(t *testing.T) {
	t.Run("ping then fragmented text", func(t *testing.T) {
		client, server := netPipeConn(t)
		defer client.Close()
		defer server.conn.Close()
		pongRead := make(chan error, 1)
		go func() {
			op, payload, _, err := readFrame(t, client)
			if err != nil {
				pongRead <- err
				return
			}
			if op != OpcodePong || string(payload) != "ok" {
				pongRead <- errors.New("unexpected pong frame")
				return
			}
			pongRead <- nil
		}()

		writes := make(chan struct{})
		go func() {
			writeMaskedFrame(t, client, OpcodePing, []byte("ok"))
			writeMaskedFrameOpts(t, client, OpcodeText, []byte("he"), false, false)
			writeMaskedFrame(t, client, OpcodeContinuation, []byte("llo"))
			close(writes)
		}()

		op, payload, err := server.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		if op != OpcodeText || string(payload) != "hello" {
			t.Fatalf("message opcode=%d payload=%q", op, payload)
		}
		if err := <-pongRead; err != nil {
			t.Fatalf("read pong: %v", err)
		}
		<-writes
	})

	t.Run("compressed text", func(t *testing.T) {
		client, server := netPipeConn(t)
		defer client.Close()
		defer server.conn.Close()
		server.permessageDeflate = true
		server.maxMessageSize = 64

		compressed, err := deflate([]byte("compressed"))
		if err != nil {
			t.Fatalf("deflate: %v", err)
		}
		writes := make(chan struct{})
		go func() {
			writeMaskedFrameOpts(t, client, OpcodeText, compressed, true, true)
			close(writes)
		}()
		op, payload, err := server.ReadMessage()
		if err != nil {
			t.Fatalf("read compressed: %v", err)
		}
		if op != OpcodeText || string(payload) != "compressed" {
			t.Fatalf("compressed opcode=%d payload=%q", op, payload)
		}
		if _, err := inflate(compressed, 1); err == nil {
			t.Fatal("expected inflate size limit error")
		}
		<-writes
	})
}

func TestConnReadMessageProtocolErrors(t *testing.T) {
	cases := []struct {
		name    string
		first   byte
		payload []byte
	}{
		{name: "reserved opcode", first: 0x80 | 0x03},
		{name: "unexpected continuation", first: 0x80 | OpcodeContinuation},
		{name: "fragmented control", first: OpcodePing},
		{name: "invalid text", first: 0x80 | OpcodeText, payload: []byte{0xff}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, server := netPipeConn(t)
			defer client.Close()
			defer server.conn.Close()
			done := make(chan struct{})
			go func() {
				_, _, _, _ = readFrame(t, client)
				close(done)
			}()
			writeErr := make(chan error, 1)
			go func() {
				writeErr <- writeRawMaskedFrameErr(client, tc.first, tc.payload)
			}()
			if _, _, err := server.ReadMessage(); err == nil {
				t.Fatal("expected protocol error")
			}
			if err := <-writeErr; err != nil {
				t.Fatalf("write raw: %v", err)
			}
			<-done
		})
	}
}

func netPipeConn(t *testing.T) (net.Conn, *Conn) {
	t.Helper()
	client, server := net.Pipe()
	return client, &Conn{conn: server, reader: bufio.NewReader(server)}
}

func websocketRequest(key string, subprotocol string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	if key != "" {
		req.Header.Set("Sec-WebSocket-Key", key)
	}
	if subprotocol != "" {
		req.Header.Set("Sec-WebSocket-Protocol", subprotocol)
	}
	return req
}

func writeRawMaskedFrame(t *testing.T, conn net.Conn, first byte, payload []byte) {
	t.Helper()
	if err := writeRawMaskedFrameErr(conn, first, payload); err != nil {
		t.Fatalf("write raw frame: %v", err)
	}
}

func writeRawMaskedFrameErr(conn net.Conn, first byte, payload []byte) error {
	length := len(payload)
	if length > 125 {
		return fmt.Errorf("raw helper only supports small payloads, got %d", length)
	}
	mask := []byte{0xA1, 0xB2, 0xC3, 0xD4}
	frame := []byte{first, 0x80 | byte(length)}
	frame = append(frame, mask...)
	for index, value := range payload {
		frame = append(frame, value^mask[index%4])
	}
	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("write raw frame: %w", err)
	}
	return nil
}

func writeMaskedFrameOpts(t *testing.T, conn net.Conn, opcode byte, payload []byte, fin bool, rsv1 bool) {
	t.Helper()
	first := opcode & 0x0F
	if fin {
		first |= 0x80
	}
	if rsv1 {
		first |= 0x40
	}
	header := []byte{first}
	length := len(payload)
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
		masked[i] = payload[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}
