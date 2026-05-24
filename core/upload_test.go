package core

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadHelpers(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if !IsMultipartRequest(req) {
		t.Fatal("expected multipart request")
	}
	if IsMultipartRequest(httptest.NewRequest(http.MethodPost, "/graphql", nil)) {
		t.Fatal("unexpected multipart request")
	}
	jsonReq := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	jsonReq.Header.Set("Content-Type", MediaTypeJSON)
	if IsMultipartRequest(jsonReq) {
		t.Fatal("json request should not be multipart")
	}
	bad := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	bad.Header.Set("Content-Type", "%%%")
	if IsMultipartRequest(bad) {
		t.Fatal("invalid media type should not be multipart")
	}

	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("multipart reader: %v", err)
	}
	form, err := reader.ReadForm(1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	upload := NewUpload(form.File["file"][0])
	if upload == nil || upload.Filename != "a.txt" || upload.Size != 5 {
		t.Fatalf("upload = %#v", upload)
	}
	file, err := upload.Open()
	if err != nil {
		t.Fatalf("open upload: %v", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read upload: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q", content)
	}
	if NewUpload(nil) != nil {
		t.Fatal("nil header should produce nil upload")
	}
}
