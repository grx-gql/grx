package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/file-upload/graph"
)

func newUploadServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := grx.NewServer(grx.WithSchema(graph.NewSchema()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func multipartRequest(t *testing.T, operations, fileMap, partName, contents string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("operations", operations); err != nil {
		t.Fatalf("write operations: %v", err)
	}
	if err := mw.WriteField("map", fileMap); err != nil {
		t.Fatalf("write map: %v", err)
	}
	fw, err := mw.CreateFormFile(partName, "hello.txt")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := io.WriteString(fw, contents); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadFileEndToEnd(t *testing.T) {
	srv := newUploadServer(t)

	operations := `{"query":"mutation($f: Upload!){ uploadFile(file: $f){ filename size contents } }","variables":{"f":null}}`
	fileMap := `{"0":["variables.f"]}`
	req := multipartRequest(t, operations, fileMap, "0", "hello world")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			UploadFile struct {
				Filename string `json:"filename"`
				Size     int    `json:"size"`
				Contents string `json:"contents"`
			} `json:"uploadFile"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", resp.Errors)
	}
	got := resp.Data.UploadFile
	if got.Filename != "hello.txt" {
		t.Errorf("filename = %q, want hello.txt", got.Filename)
	}
	if got.Size != len("hello world") {
		t.Errorf("size = %d, want %d", got.Size, len("hello world"))
	}
	if got.Contents != "hello world" {
		t.Errorf("contents = %q, want %q", got.Contents, "hello world")
	}
}

func TestUploadMissingFilePartFails(t *testing.T) {
	srv := newUploadServer(t)

	// The map references file part "0" but no such part is sent.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("operations", `{"query":"mutation($f: Upload!){ uploadFile(file: $f){ filename } }","variables":{"f":null}}`)
	_ = mw.WriteField("map", `{"0":["variables.f"]}`)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected error status, got 200: %s", rec.Body.String())
	}
}
