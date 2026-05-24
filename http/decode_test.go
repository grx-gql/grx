package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
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

func TestDecodeCoverageBranches(t *testing.T) {
	if New().config.PersistedQueries != nil {
		t.Fatal("unexpected persisted queries")
	}
	norm := New(Config{PersistedQueries: map[string]string{" ABC ": "{ ok }"}})
	if _, ok := norm.config.PersistedQueries["abc"]; !ok {
		t.Fatalf("normalized persisted queries = %#v", norm.config.PersistedQueries)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(`{ ok }`)))
	cfg := Config{PersistedQueries: map[string]string{hash: `{ ok }`}, RequirePersistedQuery: true, StrictPersistedQueries: true, MaxVariableBytes: 64}

	body := core.GraphQLBody{Extensions: map[string]any{"persistedQuery": map[string]any{"version": float64(1), "sha256Hash": hash}}}
	if err := resolvePersistedQuery(&body, cfg); err != nil {
		t.Fatalf("resolve persisted: %v", err)
	}
	if body.Query != `{ ok }` {
		t.Fatalf("query = %q", body.Query)
	}

	body = core.GraphQLBody{Query: `{ bad }`, Extensions: map[string]any{"persistedQuery": map[string]any{"version": float64(1), "sha256Hash": hash}}}
	if err := resolvePersistedQuery(&body, cfg); err == nil {
		t.Fatal("expected strict hash mismatch")
	}
	for _, body := range []*core.GraphQLBody{
		nil,
		{Query: `{ ok }`},
		{Query: `{ ok }`, Extensions: map[string]any{}},
		{Query: `{ ok }`, Extensions: map[string]any{"persistedQuery": map[string]any{}}},
	} {
		if err := resolvePersistedQuery(body, Config{}); err != nil {
			t.Fatalf("optional persisted query should pass: %v", err)
		}
	}
	required := core.GraphQLBody{}
	if err := resolvePersistedQuery(&required, Config{RequirePersistedQuery: true}); err == nil {
		t.Fatal("expected required persisted query error")
	}
	required.Extensions = map[string]any{"persistedQuery": map[string]any{}}
	if err := resolvePersistedQuery(&required, Config{RequirePersistedQuery: true}); err == nil {
		t.Fatal("expected required persisted hash error")
	}
	for _, pq := range []map[string]any{
		{},
		{"version": float64(2), "sha256Hash": hash},
		{"version": float64(1), "sha256Hash": "short"},
		{"version": float64(1), "sha256Hash": strings.Repeat("z", 64)},
	} {
		if err := validatePersistedQueryMetadata(pq, fmt.Sprint(pq["sha256Hash"])); err == nil {
			t.Fatalf("expected metadata error for %#v", pq)
		}
	}

	if err := validateVariableBytes(map[string]any{"v": strings.Repeat("x", 128)}, 8); err == nil {
		t.Fatal("expected variable size error")
	}
	if err := validateVariableBytes(map[string]any{"v": strings.Repeat("x", 128)}, 0); err != nil {
		t.Fatalf("disabled variable byte limit: %v", err)
	}
	if _, err := parsePostGraphQLJSON([]byte(`[{"query":"{ a }"},{"query":"{ b }"}]`), Config{}); err != nil {
		t.Fatalf("parse batch: %v", err)
	}
	if _, err := parsePostGraphQLJSON([]byte(`[]`), Config{}); err != nil {
		t.Fatalf("parse empty batch: %v", err)
	}
	if _, err := parsePostGraphQLJSON([]byte(``), Config{}); err == nil {
		t.Fatal("expected empty body error")
	}
	if _, err := parsePostGraphQLJSON([]byte(`[{"variables":{}}]`), Config{}); err == nil {
		t.Fatal("expected missing query in batch")
	}
	if _, err := parsePostGraphQLJSONLenient([]byte(`[{"variables":{"file":null}}]`), Config{}); err != nil {
		t.Fatalf("lenient batch: %v", err)
	}
	if _, err := parsePostGraphQLJSONLenient([]byte(``), Config{}); err == nil {
		t.Fatal("expected lenient empty error")
	}
	if _, err := parsePostGraphQLJSONLenient([]byte(`[`), Config{}); err == nil {
		t.Fatal("expected lenient invalid batch")
	}
	if _, err := parsePostGraphQLJSONLenient([]byte(`{`), Config{}); err == nil {
		t.Fatal("expected lenient invalid object")
	}

	get := httptest.NewRequest(http.MethodGet, "/graphql?query=%7B%20ok%20%7D", nil)
	if _, err := decodeGraphQLHTTP(get, Config{}); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if _, err := decodeGraphQLHTTP(httptest.NewRequest(http.MethodPatch, "/graphql", nil), Config{}); err == nil {
		t.Fatal("expected unsupported method")
	}
	post := httptest.NewRequest(http.MethodPost, "/graphql", errReader{})
	post.Header.Set("Content-Type", core.MediaTypeJSON)
	if _, err := decodeGraphQLHTTP(post, Config{}); err == nil {
		t.Fatal("expected read error")
	}
}

func TestMultipartDecodeCoverageBranches(t *testing.T) {
	req := multipartRequest(t, `{"query":"mutation($file: Upload){ upload(file:$file) }","variables":{"file":null,"files":[null]}}`, `{"0":["variables.file","variables.files.0"]}`, map[string]string{"0": "hello"})
	bodies, err := parseMultipartGraphQL(req, Config{})
	if err != nil {
		t.Fatalf("multipart: %v", err)
	}
	if _, ok := bodies[0].Variables["file"].(*core.Upload); !ok {
		t.Fatalf("file variable = %#v", bodies[0].Variables["file"])
	}
	files := bodies[0].Variables["files"].([]any)
	if _, ok := files[0].(*core.Upload); !ok {
		t.Fatalf("files[0] = %#v", files[0])
	}

	for _, req := range []*http.Request{
		multipartRequest(t, "", `{}`, nil),
		multipartRequest(t, `{"query":"{ ok }"}`, "", nil),
		multipartRequest(t, `{"query":"{ ok }"}`, `{`, nil),
		multipartRequest(t, `{"query":"{ ok }"}`, `{"missing":["variables.file"]}`, nil),
		multipartRequest(t, `{"query":"{ ok }","variables":null}`, `{"0":["variables.file"]}`, map[string]string{"0": "x"}),
		multipartRequest(t, `{"query":"{ ok }","variables":{"file":null}}`, `{"0":["bad.file"]}`, map[string]string{"0": "x"}),
		multipartRequest(t, `{"query":"{ ok }","variables":{"file":null}}`, `{"0":["variables.file.x"]}`, map[string]string{"0": "x"}),
	} {
		if _, err := parseMultipartGraphQL(req, Config{}); err == nil {
			t.Fatal("expected multipart error")
		}
	}

	bodies = []core.GraphQLBody{{Variables: map[string]any{"items": []any{nil}}}}
	if err := injectUpload(bodies, true, "x.variables.items.0", &core.Upload{}); err == nil {
		t.Fatal("expected bad batch index")
	}
	if err := injectUpload(bodies, true, "2.variables.items.0", &core.Upload{}); err == nil {
		t.Fatal("expected out-of-range batch index")
	}
	if err := setAtPath(map[string]any{"a": 1}, []string{"a", "b"}, &core.Upload{}); err == nil {
		t.Fatal("expected descend error")
	}
	if err := setAtPath(map[string]any{}, nil, &core.Upload{}); err == nil {
		t.Fatal("expected empty path error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read failed") }

func TestLimitAndRequestSizeBranches(t *testing.T) {
	cfg := Config{MaxRequestBytes: 4}
	if got := cfg.maxRequestBytes(); got != 4 {
		t.Fatalf("max = %d", got)
	}
	if got := (Config{MaxRequestBytes: -1}).maxRequestBytes(); got != 0 {
		t.Fatalf("negative max = %d", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", io.NopCloser(strings.NewReader("123456")))
	req.ContentLength = -1
	if err := limitRequestSize(rec, req, 4); err != nil {
		t.Fatalf("limit request: %v", err)
	}
	if _, err := io.ReadAll(req.Body); err == nil {
		t.Fatal("expected limited reader error")
	}

	if !requestBodyTooLarge(fmt.Errorf("request exceeds 4 byte limit")) {
		t.Fatal("expected requestBodyTooLarge")
	}
	if requestBodyTooLarge(fmt.Errorf("other")) {
		t.Fatal("unexpected requestBodyTooLarge")
	}
}

type coverIncrementalExecutor struct{}

func (coverIncrementalExecutor) Execute(context.Context, core.Request) core.Response {
	return core.Response{Data: map[string]any{"ok": true}}
}

func (coverIncrementalExecutor) Subscribe(context.Context, core.Request) (<-chan core.Response, error) {
	return nil, nil
}

func (coverIncrementalExecutor) OperationKind(core.Request) (core.OperationKind, error) {
	return core.OperationQuery, nil
}

func (coverIncrementalExecutor) HasIncrementalDirectives(core.Request) bool { return true }

func (coverIncrementalExecutor) ExecuteIncremental(context.Context, core.Request) (core.Response, []core.IncrementalPayload) {
	hasNext := true
	return core.Response{Data: map[string]any{"initial": true}, HasNext: &hasNext}, []core.IncrementalPayload{{Label: "x", Data: map[string]any{"patch": true}}}
}

func TestServeIncrementalBranch(t *testing.T) {
	tr := New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ ok }"}`))
	req.Header.Set("Content-Type", core.MediaTypeJSON)
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()
	tr.Serve(rec, req, coverIncrementalExecutor{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "multipart/mixed") {
		t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
	}
}

func multipartRequest(t *testing.T, operations string, fileMap string, parts map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if operations != "" {
		if err := writer.WriteField("operations", operations); err != nil {
			t.Fatalf("operations: %v", err)
		}
	}
	if fileMap != "" {
		if err := writer.WriteField("map", fileMap); err != nil {
			t.Fatalf("map: %v", err)
		}
	}
	for name, content := range parts {
		part, err := writer.CreateFormFile(name, name+".txt")
		if err != nil {
			t.Fatalf("file: %v", err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestValidateVariableBytesMarshalError(t *testing.T) {
	if err := validateVariableBytes(map[string]any{"bad": func() {}}, 1); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestParsePostGraphQLJSONInvalidBatchItem(t *testing.T) {
	raw, _ := json.Marshal([]json.RawMessage{json.RawMessage(`{`)})
	if _, err := parsePostGraphQLJSON(raw, Config{}); err == nil {
		t.Fatal("expected invalid batch item")
	}
}
