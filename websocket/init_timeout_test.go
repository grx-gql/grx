package websocket

import (
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoadmapAuditPingPreservesInitializationTimeout(t *testing.T) {
	tr := New(Config{ConnectionInitTimeout: 50 * time.Millisecond})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, &wsCoverExecutor{})
	}))
	defer server.Close()
	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping"}`))
	if _, _, _, err := readFrame(t, conn); err != nil {
		t.Fatal(err)
	}
	op, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatalf("pre-init ping disabled initialization deadline: %v", err)
	}
	if op != OpcodeClose || len(payload) < 2 || binary.BigEndian.Uint16(payload[:2]) != 4408 {
		t.Fatalf("expected close 4408, got opcode=%d payload=%q", op, payload)
	}
}
