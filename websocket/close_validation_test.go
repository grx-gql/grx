package websocket

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestRoadmapAuditRejectsMalformedCloseFrame(t *testing.T) {
	conn, server := dialUpgraded(t, Config{}, OpcodeClose, []byte{0x03})
	defer conn.Close()
	defer server.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	op, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatal(err)
	}
	if op != OpcodeClose || len(payload) < 2 || binary.BigEndian.Uint16(payload[:2]) != 1002 {
		t.Fatalf("one-byte close payload must be rejected with 1002; got opcode=%d payload=%x", op, payload)
	}
}

func TestCloseFrameStatusAndReasonValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
		want    uint16
	}{
		{"empty", nil, 1000},
		{"normal", []byte{3, 232}, 1000},
		{"registered service restart", []byte{3, 244}, 1000},
		{"application code", []byte{15, 160}, 1000},
		{"reserved code", []byte{3, 237}, 1002},
		{"out of range", []byte{19, 136}, 1002},
		{"invalid UTF-8", []byte{3, 232, 255}, 1007},
		{"valid reason", []byte{3, 232, 'o', 'k'}, 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, server := dialUpgraded(t, Config{}, OpcodeClose, tc.payload)
			defer conn.Close()
			defer server.Close()
			op, payload, _, err := readFrame(t, conn)
			if err != nil {
				t.Fatal(err)
			}
			if op != OpcodeClose || len(payload) < 2 || binary.BigEndian.Uint16(payload[:2]) != tc.want {
				t.Fatalf("close response = %x, want %d", payload, tc.want)
			}
		})
	}
}
