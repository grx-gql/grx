package websocket

import (
	"bufio"
	"bytes"
	"testing"
	"time"
)

func TestRoadmapAuditRejectsHighBitFrameLengthWithoutPanic(t *testing.T) {
	client, server := netPipeConn(t)
	defer client.Close()
	defer server.conn.Close()
	frame := []byte{0x81, 0xff, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	server.reader = bufio.NewReader(bytes.NewReader(frame))
	server.maxMessageSize = DefaultMaxMessageSize
	server.writeTimeout = time.Millisecond
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Errorf("untrusted 64-bit payload length bypassed size limit and panicked: %v", recovered)
		}
	}()
	if _, _, err := server.ReadMessage(); err == nil {
		t.Fatal("invalid frame length accepted")
	}
}

func TestReadPayloadLengthRejectsInvalidEncodings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		indicator byte
		encoded   []byte
	}{
		{"short encoded as 16-bit", 126, []byte{0, 125}},
		{"short encoded as 64-bit", 127, []byte{0, 0, 0, 0, 0, 0, 255, 255}},
		{"reserved high bit", 127, []byte{128, 0, 0, 0, 0, 0, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readPayloadLength(bufio.NewReader(bytes.NewReader(tc.encoded)), tc.indicator); err == nil {
				t.Fatal("invalid payload encoding accepted")
			}
		})
	}
}
