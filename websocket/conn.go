// Package websocket provides an RFC 6455 WebSocket implementation tailored
// for GraphQL subscription transports. It implements core.Transport for both
// the modern graphql-transport-ws subprotocol and the legacy graphql-ws
// (Apollo subscriptions-transport-ws) subprotocol.
package websocket

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/grx-gql/grx/core"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Frame opcodes from RFC 6455 §5.2. Exported so callers can inspect the
// frames produced by the transport.
const (
	OpcodeContinuation byte = 0x0
	OpcodeText         byte = 0x1
	OpcodeBinary       byte = 0x2
	OpcodeClose        byte = 0x8
	OpcodePing         byte = 0x9
	OpcodePong         byte = 0xA
)

// Standard close codes from RFC 6455 §7.4.1. Codes not currently emitted by
// the framing layer are kept for documentation; future graceful-shutdown
// and policy-enforcement work will use them.
const (
	closeNormal         uint16 = 1000
	closeGoingAway      uint16 = 1001
	closeProtocol       uint16 = 1002
	closeUnsupported    uint16 = 1003
	closeInvalidPayload uint16 = 1007
	closeMessageTooBig  uint16 = 1009
	closeTryAgainLater  uint16 = 1013
)

// permessageDeflateExtension is the RFC 7692 extension token. The server
// negotiates it with no-context-takeover in both directions so each message is
// (de)compressed independently, keeping per-connection state minimal.
const permessageDeflateExtension = "permessage-deflate"

// IsTimeout reports whether err originated from a deadline exceeded on the
// underlying connection. The dispatcher uses this to map a missed
// connection_init into the correct close code.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return false
}

// Conn is a server-side WebSocket connection. Reads are single-threaded and
// happen on the dispatcher goroutine; writes are guarded by a mutex so
// concurrent goroutines (e.g. subscription dispatchers, server-side ping)
// can push frames safely.
type Conn struct {
	conn        net.Conn
	reader      *bufio.Reader
	subprotocol string

	maxMessageSize int64
	writeTimeout   time.Duration

	// permessageDeflate is true when the RFC 7692 extension was negotiated
	// during the handshake; data frames are then compressed per message.
	permessageDeflate bool

	writeMu  sync.Mutex
	closedMu sync.Mutex
	closed   bool

	// continuation tracks the in-flight fragmented data message. It is set
	// by the first non-FIN data frame and cleared by the FIN continuation.
	continuation byte
}

// IsUpgrade reports whether r is a well-formed WebSocket upgrade request.
func IsUpgrade(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if !core.HeaderContains(r.Header.Values("Connection"), "Upgrade") {
		return false
	}
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// upgradeOptions controls handshake-time behaviour and is populated from the
// parent WebSocketTransport's Config so callers do not have to thread it manually.
type upgradeOptions struct {
	subprotocols      []string
	checkOrigin       func(r *http.Request) bool
	maxMessageSize    int64
	writeTimeout      time.Duration
	permessageDeflate bool
}

// upgrade performs the RFC 6455 server handshake, hijacks the connection,
// and returns a Conn ready for message exchange. It also enforces the
// configured Origin allowlist before any framing happens.
func upgrade(w http.ResponseWriter, r *http.Request, opts upgradeOptions) (*Conn, error) {
	if !IsUpgrade(r) {
		return nil, errors.New("not a websocket upgrade")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "websocket version 13 required", http.StatusUpgradeRequired)
		return nil, errors.New("invalid websocket version")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	if opts.checkOrigin != nil && !opts.checkOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return nil, errors.New("origin not allowed")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return nil, errors.New("response writer does not implement http.Hijacker")
	}

	negotiated := negotiateSubprotocol(r.Header.Values("Sec-WebSocket-Protocol"), opts.subprotocols)
	useDeflate := opts.permessageDeflate && offersPermessageDeflate(r.Header.Values("Sec-WebSocket-Extensions"))

	netConn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("websocket hijack: %w", err)
	}

	accept := computeAccept(key)
	response := strings.Builder{}
	response.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	response.WriteString("Upgrade: websocket\r\n")
	response.WriteString("Connection: Upgrade\r\n")
	response.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n")
	if negotiated != "" {
		response.WriteString("Sec-WebSocket-Protocol: " + negotiated + "\r\n")
	}
	if useDeflate {
		response.WriteString("Sec-WebSocket-Extensions: permessage-deflate; client_no_context_takeover; server_no_context_takeover\r\n")
	}
	response.WriteString("\r\n")
	if _, err := buf.WriteString(response.String()); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("write handshake: %w", err)
	}
	if err := buf.Flush(); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("flush handshake: %w", err)
	}

	maxMessage := opts.maxMessageSize
	if maxMessage == 0 {
		maxMessage = DefaultMaxMessageSize
	}
	return &Conn{
		conn:              netConn,
		reader:            buf.Reader,
		subprotocol:       negotiated,
		maxMessageSize:    maxMessage,
		writeTimeout:      opts.writeTimeout,
		permessageDeflate: useDeflate,
	}, nil
}

// offersPermessageDeflate reports whether the client advertised the
// permessage-deflate extension in its Sec-WebSocket-Extensions header.
func offersPermessageDeflate(values []string) bool {
	for _, raw := range values {
		for _, ext := range strings.Split(raw, ",") {
			name := strings.TrimSpace(ext)
			if semi := strings.IndexByte(name, ';'); semi >= 0 {
				name = strings.TrimSpace(name[:semi])
			}
			if strings.EqualFold(name, permessageDeflateExtension) {
				return true
			}
		}
	}
	return false
}

// Subprotocol returns the negotiated WebSocket subprotocol, or the empty
// string if no subprotocol was agreed upon.
func (c *Conn) Subprotocol() string { return c.subprotocol }

// SetReadDeadline sets the absolute time after which a pending read will
// fail. A zero value disables the deadline. Dispatchers use it to enforce
// the connection_init timeout and the read idle timeout.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.conn.SetReadDeadline(t) }

func negotiateSubprotocol(offered []string, accepted []string) string {
	for _, supported := range accepted {
		for _, raw := range offered {
			for _, candidate := range strings.Split(raw, ",") {
				if strings.EqualFold(strings.TrimSpace(candidate), supported) {
					return supported
				}
			}
		}
	}
	return ""
}

func computeAccept(key string) string {
	hasher := sha1.New()
	hasher.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

// ReadMessage reads the next text or binary message from the peer,
// reassembling fragmented frames and responding to ping and close control
// frames automatically. It enforces the RFC 6455 framing rules: control
// frames must be unfragmented and ≤125 bytes, reserved bits must be zero,
// continuation frames must follow an in-flight data message, and text
// messages must be valid UTF-8.
func (c *Conn) ReadMessage() (opcode byte, payload []byte, err error) {
	var assembled []byte
	var totalSize int64
	dataOpcode := byte(0)
	messageCompressed := false
	for {
		header := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, header); err != nil {
			return 0, nil, err
		}
		fin := header[0]&0x80 != 0
		rsv1 := header[0]&0x40 != 0
		// RSV2 and RSV3 must always be zero; RSV1 is only permitted on the
		// first frame of a data message when permessage-deflate is negotiated.
		if header[0]&0x30 != 0 {
			c.SendClose(closeProtocol, "RSV bits must be zero")
			return 0, nil, errors.New("websocket frame had non-zero RSV2/RSV3 bits")
		}
		op := header[0] & 0x0F
		isControlFrame := op&0x8 != 0
		if rsv1 && (!c.permessageDeflate || isControlFrame || op == OpcodeContinuation) {
			c.SendClose(closeProtocol, "unexpected RSV1 bit")
			return 0, nil, errors.New("websocket frame had unexpected RSV1 bit")
		}
		masked := header[1]&0x80 != 0
		if !masked {
			c.SendClose(closeProtocol, "frames from client must be masked")
			return 0, nil, errors.New("websocket frame from client was not masked")
		}

		length, err := readPayloadLength(c.reader, header[1]&0x7F)
		if err != nil {
			return 0, nil, err
		}

		isControl := op&0x8 != 0
		if isControl {
			if !fin {
				c.SendClose(closeProtocol, "control frames must not be fragmented")
				return 0, nil, errors.New("fragmented control frame")
			}
			if length > 125 {
				c.SendClose(closeProtocol, "control frame too large")
				return 0, nil, errors.New("oversized control frame")
			}
		} else {
			if op == OpcodeContinuation {
				if dataOpcode == 0 {
					c.SendClose(closeProtocol, "unexpected continuation frame")
					return 0, nil, errors.New("continuation without preceding data frame")
				}
			} else {
				if dataOpcode != 0 {
					c.SendClose(closeProtocol, "expected continuation frame")
					return 0, nil, errors.New("interleaved data frame inside fragmented message")
				}
				dataOpcode = op
				messageCompressed = rsv1
			}
			if c.maxMessageSize > 0 {
				totalSize += int64(length)
				if totalSize > c.maxMessageSize {
					c.SendClose(closeMessageTooBig, "message too large")
					return 0, nil, errors.New("websocket message exceeds configured limit")
				}
			}
		}

		mask := make([]byte, 4)
		if _, err := io.ReadFull(c.reader, mask); err != nil {
			return 0, nil, err
		}
		framePayload := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(c.reader, framePayload); err != nil {
				return 0, nil, err
			}
		}
		for i := range framePayload {
			framePayload[i] ^= mask[i%4]
		}

		switch op {
		case OpcodePing:
			if err := c.writeFrame(OpcodePong, framePayload); err != nil {
				return 0, nil, err
			}
			continue
		case OpcodePong:
			continue
		case OpcodeClose:
			c.SendClose(closeNormal, "")
			return 0, nil, io.EOF
		case OpcodeContinuation, OpcodeText, OpcodeBinary:
			assembled = append(assembled, framePayload...)
		default:
			c.SendClose(closeProtocol, "reserved opcode")
			return 0, nil, fmt.Errorf("reserved websocket opcode 0x%X", op)
		}

		if fin {
			result := dataOpcode
			if messageCompressed {
				inflated, err := inflate(assembled, c.maxMessageSize)
				if err != nil {
					c.SendClose(closeInvalidPayload, "invalid compressed payload")
					return 0, nil, fmt.Errorf("permessage-deflate inflate: %w", err)
				}
				assembled = inflated
			}
			if result == OpcodeText && !utf8.Valid(assembled) {
				c.SendClose(closeInvalidPayload, "invalid UTF-8 in text frame")
				return 0, nil, errors.New("invalid UTF-8 text frame")
			}
			return result, assembled, nil
		}
	}
}

func readPayloadLength(reader *bufio.Reader, indicator byte) (uint64, error) {
	switch {
	case indicator < 126:
		return uint64(indicator), nil
	case indicator == 126:
		header := make([]byte, 2)
		if _, err := io.ReadFull(reader, header); err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint16(header)), nil
	case indicator == 127:
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(header), nil
	}
	return 0, fmt.Errorf("invalid payload indicator %d", indicator)
}

// WriteText writes a complete UTF-8 text frame to the peer. Concurrent writes
// are serialised through the connection's mutex; the configured write
// deadline (if any) protects against slow consumers.
func (c *Conn) WriteText(payload []byte) error {
	if c.permessageDeflate {
		if compressed, err := deflate(payload); err == nil {
			return c.writeFrameOpts(OpcodeText, compressed, true)
		}
		// Fall back to an uncompressed frame if compression fails.
	}
	return c.writeFrame(OpcodeText, payload)
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	return c.writeFrameOpts(opcode, payload, false)
}

// writeFrameOpts writes a single final frame, optionally setting the RSV1 bit
// to mark a permessage-deflate compressed payload.
func (c *Conn) writeFrameOpts(opcode byte, payload []byte, rsv1 bool) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.isClosed() {
		return errors.New("websocket connection is closed")
	}
	if c.writeTimeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		defer c.conn.SetWriteDeadline(time.Time{})
	}

	first := byte(0x80 | (opcode & 0x0F))
	if rsv1 {
		first |= 0x40
	}
	header := []byte{first}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 0xFFFF:
		header = append(header, 126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(length))
	default:
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(length))
	}

	if _, err := c.conn.Write(header); err != nil {
		c.markClosed()
		return err
	}
	if length == 0 {
		return nil
	}
	if _, err := c.conn.Write(payload); err != nil {
		c.markClosed()
		return err
	}
	return nil
}

// SendClose sends a WebSocket close frame with the given code and reason and
// marks the connection as closed for further writes.
func (c *Conn) SendClose(code uint16, reason string) {
	body := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(body, code)
	copy(body[2:], reason)
	_ = c.writeFrame(OpcodeClose, body)
	c.markClosed()
}

func (c *Conn) markClosed() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	c.closed = true
}

func (c *Conn) isClosed() bool {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	return c.closed
}

// Close terminates the underlying network connection.
func (c *Conn) Close() error {
	c.markClosed()
	return c.conn.Close()
}

// deflate compresses data for a permessage-deflate message using
// no-context-takeover semantics: a fresh DEFLATE stream is sync-flushed and the
// trailing empty block (0x00 0x00 0xFF 0xFF) is removed per RFC 7692 §7.2.1.
func deflate(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(data); err != nil {
		return nil, err
	}
	if err := fw.Flush(); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	if n := len(out); n >= 4 && out[n-4] == 0x00 && out[n-3] == 0x00 && out[n-2] == 0xff && out[n-1] == 0xff {
		out = out[:n-4]
	}
	return out, nil
}

// inflate decompresses a permessage-deflate message by appending the trailing
// empty block removed on the wire and reading the DEFLATE stream, bounded by
// maxSize (0 = unbounded) to guard against decompression bombs.
func inflate(data []byte, maxSize int64) ([]byte, error) {
	// Re-append the sync-flush marker stripped on the wire, plus a final empty
	// DEFLATE block so the reader terminates with io.EOF instead of
	// io.ErrUnexpectedEOF (RFC 7692 §7.2.2 / the standard flate tail).
	full := make([]byte, 0, len(data)+9)
	full = append(full, data...)
	full = append(full, 0x00, 0x00, 0xff, 0xff, 0x01, 0x00, 0x00, 0xff, 0xff)

	reader := flate.NewReader(bytes.NewReader(full))
	defer reader.Close()

	if maxSize > 0 {
		limited := io.LimitReader(reader, maxSize+1)
		out, err := io.ReadAll(limited)
		if err != nil {
			return nil, err
		}
		if int64(len(out)) > maxSize {
			return nil, errors.New("decompressed message exceeds configured limit")
		}
		return out, nil
	}
	return io.ReadAll(reader)
}
