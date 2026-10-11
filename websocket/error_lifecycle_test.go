package websocket

import (
	"encoding/json"
	"github.com/grx-gql/grx/core"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRoadmapAuditNoCompleteAfterOperationError(t *testing.T) {
	tr := New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr.Serve(w, r, &wsCoverExecutor{stream: make(chan core.Response)})
	}))
	defer server.Close()
	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	if _, _, _, err := readFrame(t, conn); err != nil {
		t.Fatal(err)
	}
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"subscribe","id":"op","payload":{"query":"bad"}}`))
	_, payload, _, err := readFrame(t, conn)
	if err != nil {
		t.Fatal(err)
	}
	var msg message
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "error" {
		t.Fatalf("expected operation error, got %s", payload)
	}
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping"}`))
	_, payload, _, err = readFrame(t, conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "pong" {
		t.Fatalf("error must terminate the operation; unexpected subsequent message: %s", payload)
	}
}
