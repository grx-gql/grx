package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
)

func TestParsePostGraphQLJSONBatch(t *testing.T) {
	raw := []byte(`[{"query":"{a}"},{"query":"{b}"}]`)
	bodies, err := parsePostGraphQLJSON(raw, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[0].Query != "{a}" || bodies[1].Query != "{b}" {
		t.Fatalf("unexpected bodies: %#v", bodies)
	}
}

func TestParsePostGraphQLJSONPersistedQuery(t *testing.T) {
	q := `{__typename}`
	h := sha256.Sum256([]byte(q))
	hash := hex.EncodeToString(h[:])
	reg := map[string]string{strings.ToLower(hash): q}

	raw := []byte(`{"extensions":{"persistedQuery":{"version":1,"sha256Hash":"` + hash + `"}}}`)
	bodies, err := parsePostGraphQLJSON(raw, Config{PersistedQueries: reg})
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 1 || bodies[0].Query != q {
		t.Fatalf("unexpected bodies: %#v", bodies)
	}
}

func TestParsePostGraphQLJSONPersistedQueryUnknown(t *testing.T) {
	raw := []byte(`{"extensions":{"persistedQuery":{"version":1,"sha256Hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}`)
	_, err := parsePostGraphQLJSON(raw, Config{PersistedQueries: map[string]string{"dead": "query"}})
	if err == nil || !strings.Contains(err.Error(), "unknown persisted query") {
		t.Fatalf("expected unknown persisted query error, got %v", err)
	}
}

func TestParsePostGraphQLJSONStrictPersistedQueryRejectsHashMismatch(t *testing.T) {
	raw := []byte(`{"query":"{__typename}","extensions":{"persistedQuery":{"version":1,"sha256Hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}`)
	_, err := parsePostGraphQLJSON(raw, Config{StrictPersistedQueries: true})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected hash mismatch error, got %v", err)
	}
}

func TestParsePostGraphQLJSONRequiresPersistedQuery(t *testing.T) {
	raw := []byte(`{"query":"{__typename}"}`)
	_, err := parsePostGraphQLJSON(raw, Config{RequirePersistedQuery: true})
	if err == nil || !strings.Contains(err.Error(), "persisted query is required") {
		t.Fatalf("expected required persisted query error, got %v", err)
	}
}

func TestParsePostGraphQLJSONRejectsOversizedVariables(t *testing.T) {
	raw := []byte(`{"query":"query($name: String!) { hello }","variables":{"name":"abcdef"}}`)
	_, err := parsePostGraphQLJSON(raw, Config{MaxVariableBytes: 8})
	if err == nil || !strings.Contains(err.Error(), "variables exceed") {
		t.Fatalf("expected variable size error, got %v", err)
	}
}

type seqExecutor struct {
	step int
}

func (s *seqExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	s.step++
	return core.Response{Data: map[string]any{"step": s.step}}
}

func (s *seqExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	return nil, errors.New("not used")
}

func (s *seqExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	return core.OperationQuery, nil
}

func TestTransportServeExecutesJSONBatch(t *testing.T) {
	ex := &seqExecutor{}
	tr := New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`[{"query":"{a}"},{"query":"{b}"}]`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	tr.Serve(rec, req, ex)

	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var batch []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch) != 2 {
		t.Fatalf("expected 2 responses, got %#v", batch)
	}
	for i, item := range batch {
		data, ok := item["data"].(map[string]any)
		if !ok {
			t.Fatalf("item %d: expected data map, got %#v", i, item["data"])
		}
		step, ok := data["step"].(float64)
		if !ok {
			t.Fatalf("item %d: expected numeric step, got %#v", i, data["step"])
		}
		if int(step) != i+1 {
			t.Fatalf("item %d: expected step %d, got %v", i, i+1, step)
		}
	}
}
