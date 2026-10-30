package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
)

type reproCtxKey string

func TestRequestContextValuesReachOnConnect(t *testing.T) {
	got := make(chan any, 1)
	tr := New(Config{
		ConnectionInitTimeout: time.Second,
		OnConnect: func(ctx context.Context, _ json.RawMessage) (context.Context, json.RawMessage, error) {
			got <- ctx.Value(reproCtxKey("session"))
			return ctx, json.RawMessage(`{}`), nil
		},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), reproCtxKey("session"), "sess-abc"))
		tr.Serve(w, r, &wsCoverExecutor{stream: make(chan core.Response)})
	}))
	defer server.Close()

	conn := dialHandshake(t, server.URL, Subprotocol)
	defer conn.Close()
	writeMaskedFrame(t, conn, OpcodeText, []byte(`{"type":"connection_init"}`))
	_, _, _, _ = readFrame(t, conn)

	select {
	case v := <-got:
		if v != "sess-abc" {
			t.Fatalf("OnConnect ctx missing session value, got %v", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnConnect never fired")
	}
}
