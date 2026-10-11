package http_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
)

func TestServeSingletonMultipartBatchReturnsArray(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("operations", `[{"query":"{ hello }"}]`); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("map", `{}`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	grxhttp.New().Serve(rec, req, &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}})
	var responses []core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("expected array response: %v; body=%s", err, rec.Body.String())
	}
	if len(responses) != 1 || len(responses[0].Errors) != 0 {
		t.Fatalf("responses=%+v", responses)
	}
}

func TestServeSingletonMutationUsesBatchPolicy(t *testing.T) {
	executor := &stubExecutor{}
	req := postGraphQLRequest("/graphql", `[{"query":"mutation { update }"}]`)
	rec := httptest.NewRecorder()
	grxhttp.New().Serve(rec, req, executor)
	var responses []core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || len(responses[0].Errors) != 1 || executor.gotReq.Query != "" {
		t.Fatalf("singleton batch bypassed operation policy: %+v", responses)
	}
}
