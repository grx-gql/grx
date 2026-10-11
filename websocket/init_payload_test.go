package websocket

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestControlMessagesRejectMalformedPayloadBeforeHook(t *testing.T) {
	for _, kind := range []string{"connection_init", "ping", "pong"} {
		for _, payload := range []string{`[]`, `"token"`, `true`, `123`} {
			t.Run(kind+payload, func(t *testing.T) {
				var called atomic.Bool
				transport := New(Config{OnConnect: func(ctx context.Context, _ json.RawMessage) (context.Context, json.RawMessage, error) {
					called.Store(true)
					return ctx, nil, nil
				}})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { transport.Serve(w, r, &wsCoverExecutor{}) }))
				defer server.Close()
				conn := dialHandshake(t, server.URL, Subprotocol)
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				writeMaskedFrame(t, conn, OpcodeText, []byte(fmt.Sprintf(`{"type":%q,"payload":%s}`, kind, payload)))
				op, body, _, err := readFrame(t, conn)
				if err != nil {
					t.Fatal(err)
				}
				if op != OpcodeClose || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != wsCloseInvalidMessage {
					t.Fatalf("expected4400, op=%d body=%s", op, body)
				}
				if called.Load() {
					t.Fatal("invalid payload reached authentication hook")
				}
			})
		}
	}

}

func TestControlMessagesAcceptObjectNullOrOmittedPayload(t *testing.T) {
	for _, kind := range []string{"connection_init", "ping", "pong"} {
		for _, payload := range []string{"", `,"payload":null`, `,"payload":{"token":"ok"}`} {
			message := fmt.Sprintf(`{"type":%q%s}`, kind, payload)
			t.Run(message, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { New().Serve(w, r, &wsCoverExecutor{}) }))
				defer server.Close()
				conn := dialHandshake(t, server.URL, Subprotocol)
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				writeMaskedFrame(t, conn, OpcodeText, []byte(message))
				if kind == "pong" {
					writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"ping"}`))
				}
				op, body, _, err := readFrame(t, conn)
				if err != nil {
					t.Fatal(err)
				}
				var response struct{ Type string }
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				want := "pong"
				if kind == "connection_init" {
					want = "connection_ack"
				}
				if op != OpcodeText || response.Type != want {
					t.Fatalf("op=%d body=%s", op, body)
				}
			})
		}
	}

}

func TestSubscribeRejectsMalformedPayloadShape(t *testing.T) {
	for _, payload := range []string{`null`, `[]`, `{}`, `{"query":null}`, `{"query":12}`, `{"query":"{ ok }","variables":[]}`} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { New().Serve(w, r, &wsCoverExecutor{}) }))
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
			writeMaskedFrame(t, conn, OpcodeText, []byte(fmt.Sprintf(`{"id":"operation","type":"subscribe","payload":%s}`, payload)))
			op, body, _, err := readFrame(t, conn)
			if err != nil {
				t.Fatal(err)
			}
			if op != OpcodeClose || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != wsCloseInvalidMessage {
				t.Fatalf("expected4400, op=%d body=%s", op, body)
			}
		})
	}
}
