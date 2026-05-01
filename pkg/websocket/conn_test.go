package websocket

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
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
// graphql-transport-ws Transport.
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
