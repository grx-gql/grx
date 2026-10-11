package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/grx-gql/grx/core"
	"testing"
)

func TestStaleOperationCleanupPreservesReusedID(t *testing.T) {
	dispatcher := newDispatcher(nil, nil, Config{})
	_, oldCancel := context.WithCancel(context.Background())
	defer oldCancel()
	old := &activeSubscription{cancel: oldCancel}
	dispatcher.subscriptions["reused"] = old
	dispatcher.cancelSubscription("reused")
	replacementCtx, replacementCancel := context.WithCancel(context.Background())
	defer replacementCancel()
	dispatcher.subscriptions["reused"] = &activeSubscription{cancel: replacementCancel}
	// The old worker exits after the client has reused the canceled operation ID.
	dispatcher.removeSubscription("reused", old)
	dispatcher.cancelSubscription("reused")
	if replacementCtx.Err() == nil {
		t.Fatal("old worker cleanup removed the replacement operation; client completion no longer cancels it")
	}
}

type pendingQueryExecutor struct {
	wsCoverExecutor
	started chan struct{}
	release chan struct{}
}

func (e *pendingQueryExecutor) Execute(context.Context, core.Request) core.Response {
	close(e.started)
	<-e.release
	return core.Response{Data: map[string]any{"ok": true}}
}

func TestCanceledQueryDoesNotSendResult(t *testing.T) {
	client, server := netPipeConn(t)
	defer client.Close()
	defer server.Close()
	executor := &pendingQueryExecutor{started: make(chan struct{}), release: make(chan struct{})}
	dispatcher := newDispatcher(server, executor, Config{})
	dispatcher.setConnCtx(context.Background())
	defer dispatcher.cancelAll()
	dispatcher.startSubscription(message{ID: "query", Type: "subscribe", Payload: json.RawMessage(`{"query":"{ ok }"}`)})
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("query did not start")
	}
	dispatcher.cancelSubscription("query")
	close(executor.release)
	if err := client.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 2)
	count, err := client.Read(header)
	if count != 0 || !IsTimeout(err) {
		t.Fatalf("canceled query sent frame header=%v count=%d err=%v", header, count, err)
	}
}

func TestSlowClientWriteTimeoutReleasesOperation(t *testing.T) {
	client, server := netPipeConn(t)
	defer client.Close()
	defer server.Close()
	server.writeTimeout = 20 * time.Millisecond
	stream := make(chan core.Response, 1)
	stream <- core.Response{Data: map[string]any{"event": "ready"}}
	dispatcher := newDispatcher(server, &wsCoverExecutor{stream: stream}, Config{})
	dispatcher.setConnCtx(context.Background())
	defer dispatcher.cancelAll()
	dispatcher.startSubscription(message{ID: "slow", Type: "subscribe", Payload: json.RawMessage(`{"query":"subscription { event }"}`)})
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		dispatcher.mu.Lock()
		active := len(dispatcher.subscriptions)
		dispatcher.mu.Unlock()
		if active == 0 {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("stalled client write did not time out and release its operation")
		}
	}
}
