package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/patrickkabwe/grx/core"
)

func TestDispatcherErrorBranches(t *testing.T) {
	client, server := netPipeConn(t)
	defer client.Close()
	defer server.conn.Close()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, client)
		close(done)
	}()

	executor := &wsCoverExecutor{stream: make(chan core.Response)}
	d := newDispatcher(server, executor, Config{MaxSubscriptions: 1})
	d.setConnCtx(context.Background())

	if _, err := d.invokeOnConnect(context.Background(), nil); err != nil {
		t.Fatalf("invoke without hook: %v", err)
	}
	d.refreshReadDeadline()
	d.startSubscription(message{Type: "subscribe"})
	d.startSubscription(message{ID: "bad-json", Type: "subscribe", Payload: json.RawMessage(`{`)})
	d.startSubscription(message{ID: "missing-query", Type: "subscribe", Payload: json.RawMessage(`{}`)})
	d.startSubscription(message{ID: "bad-kind", Type: "subscribe", Payload: json.RawMessage(`{"query":"bad"}`)})
	d.cancelSubscription("missing")
	d.cancelAll()
	_ = server.conn.Close()
	<-done
}

func TestDispatcherSubscriptionLimitsAndStreamErrors(t *testing.T) {
	client, server := netPipeConn(t)
	defer client.Close()
	defer server.conn.Close()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, client)
		close(done)
	}()

	executor := &wsCoverExecutor{stream: make(chan core.Response), subscribeErr: errors.New("subscribe failed")}
	d := newDispatcher(server, executor, Config{MaxSubscriptions: 1})
	d.setConnCtx(context.Background())

	d.startSubscription(message{ID: "first", Type: "subscribe", Payload: json.RawMessage(`{"query":"subscription { event }"}`)})
	d.startSubscription(message{ID: "second", Type: "subscribe", Payload: json.RawMessage(`{"query":"subscription { event }"}`)})
	d.cancelSubscription("first")
	d.runStream(context.Background(), "direct", core.GraphQLBody{Query: "subscription { event }"})

	_ = server.conn.Close()
	<-done
}

func TestDispatcherSendHelpersReturnWriteErrors(t *testing.T) {
	client, server := netPipeConn(t)
	_ = client.Close()
	_ = server.conn.Close()

	d := newDispatcher(server, &wsCoverExecutor{}, Config{})
	if err := d.send(message{Type: "ping"}); err == nil {
		t.Fatal("expected send write error")
	}
	if err := d.sendAck(json.RawMessage(`{"ok":true}`)); err == nil {
		t.Fatal("expected ack write error")
	}
	if err := d.sendNext("1", core.Response{Data: map[string]any{"ok": true}}); err == nil {
		t.Fatal("expected next write error")
	}
	if err := d.sendError("1", errors.New("boom")); err == nil {
		t.Fatal("expected error write error")
	}
}
