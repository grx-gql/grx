package websocket

import (
	"bufio"
	"context"
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
