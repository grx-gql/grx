package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/patrickkabwe/grx/core"
)

// Transport streams GraphQL subscription responses over Server-Sent Events.
// It is safe to share across requests.
type Transport struct {
	config Config
	active int64
}

// Config tunes an SSE transport.
type Config struct {
	// MaxActiveSubscriptions limits concurrent SSE streams on this transport.
	// Zero disables the limit.
	MaxActiveSubscriptions int64
}

// New returns a Transport ready to be registered with the server.
func New(cfg ...Config) *Transport {
	t := &Transport{}
	if len(cfg) > 0 {
		t.config = cfg[0]
	}
	return t
}

// Match reports whether r is asking for a text/event-stream response.
func (Transport) Match(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}
	return core.HeaderContains(r.Header.Values("Accept"), "text/event-stream")
}

// Serve runs the SSE response loop, forwarding each emitted Response as a
// "next" event and closing with a "complete" event when the stream ends.
func (t *Transport) Serve(w http.ResponseWriter, r *http.Request, executor core.Executor) {
	if t.config.MaxActiveSubscriptions > 0 {
		next := atomic.AddInt64(&t.active, 1)
		if next > t.config.MaxActiveSubscriptions {
			atomic.AddInt64(&t.active, -1)
			core.WriteJSON(w, http.StatusTooManyRequests, core.Response{Errors: []core.Error{{Message: "active SSE subscription limit exceeded"}}})
			return
		}
		defer atomic.AddInt64(&t.active, -1)
	}

	body, err := readRequest(r)
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, core.Response{Errors: []core.Error{{Message: err.Error()}}})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	stream, err := executor.Subscribe(r.Context(), core.Request{
		Query:         body.Query,
		OperationName: body.OperationName,
		Variables:     body.Variables,
	})
	if err != nil {
		writeEvent(w, "next", core.Response{Errors: []core.Error{{Message: err.Error()}}})
		writeComplete(w)
		flusher.Flush()
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case res, open := <-stream:
			if !open {
				writeComplete(w)
				flusher.Flush()
				return
			}
			writeEvent(w, "next", res)
			flusher.Flush()
		}
	}
}

func readRequest(r *http.Request) (core.GraphQLBody, error) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		return core.GraphQLBody{}, fmt.Errorf("method %s not allowed for SSE", r.Method)
	}
	return core.DecodeGraphQLRequest(r)
}

func writeEvent(w http.ResponseWriter, event string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(w, "event: next\ndata: %s\n\n", `{"errors":[{"message":"failed to encode payload"}]}`)
		return
	}
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	for _, line := range strings.Split(string(encoded), "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}

func writeComplete(w http.ResponseWriter) {
	fmt.Fprint(w, "event: complete\ndata: \n\n")
}
