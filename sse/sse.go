package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/grx-gql/grx/core"
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

	// MaxRequestBytes limits POST JSON bodies and GET query, variables, and
	// operationName parameter bytes combined. Zero disables the limit.
	MaxRequestBytes int64

	// MaxVariableBytes limits the JSON-encoded variables payload. Zero disables
	// the limit.
	MaxVariableBytes int64
}

// New returns a Transport ready to be registered with the server.
func New(cfg ...Config) *Transport {
	t := &Transport{}
	if len(cfg) > 0 {
		t.config = cfg[0]
	}
	return t
}

// ApplyServerLimits copies server-level request limits onto the transport when
// the transport did not set stricter local limits.
func (t *Transport) ApplyServerLimits(maxRequestBytes int64, maxVariableBytes int64) {
	if t == nil {
		return
	}
	if t.config.MaxRequestBytes == 0 {
		t.config.MaxRequestBytes = maxRequestBytes
	}
	if t.config.MaxVariableBytes == 0 {
		t.config.MaxVariableBytes = maxVariableBytes
	}
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

	if err := limitRequestSize(w, r, t.config.MaxRequestBytes); err != nil {
		core.WriteJSON(w, http.StatusRequestEntityTooLarge, core.Response{Errors: []core.Error{{Message: err.Error()}}})
		return
	}

	body, err := readRequest(r, t.config)
	if err != nil {
		status := http.StatusBadRequest
		if t.config.MaxRequestBytes > 0 && requestBodyTooLarge(err) {
			status = http.StatusRequestEntityTooLarge
		}
		core.WriteJSON(w, status, core.Response{Errors: []core.Error{{Message: err.Error()}}})
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

func readRequest(r *http.Request, config Config) (core.GraphQLBody, error) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		return core.GraphQLBody{}, fmt.Errorf("method %s not allowed for SSE", r.Method)
	}
	body, err := core.DecodeGraphQLRequest(r)
	if err != nil {
		return body, err
	}
	if err := validateVariableBytes(body.Variables, config.MaxVariableBytes); err != nil {
		return body, err
	}
	return body, nil
}

func limitRequestSize(w http.ResponseWriter, r *http.Request, max int64) error {
	if max <= 0 {
		return nil
	}
	switch r.Method {
	case http.MethodPost:
		if r.ContentLength > max {
			return fmt.Errorf("request exceeds %d byte limit", max)
		}
		r.Body = http.MaxBytesReader(w, r.Body, max)
	case http.MethodGet:
		q := r.URL.Query()
		size := int64(len(q.Get("query")) + len(q.Get("variables")) + len(q.Get("operationName")))
		if size > max {
			return fmt.Errorf("request exceeds %d byte limit", max)
		}
	}
	return nil
}

func requestBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "request body too large") ||
		(strings.Contains(msg, "request exceeds") && strings.Contains(msg, "byte limit"))
}

func validateVariableBytes(variables map[string]any, max int64) error {
	if max <= 0 || len(variables) == 0 {
		return nil
	}
	raw, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("invalid GraphQL variables: %s", err.Error())
	}
	if int64(len(raw)) > max {
		return fmt.Errorf("GraphQL variables exceed %d byte limit", max)
	}
	return nil
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
