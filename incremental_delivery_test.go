package grx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/schema"
)

type deferUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type deferQuery struct{}

func (deferQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*deferUser, error) {
	return &deferUser{ID: args.ID, Name: "Ada"}, nil
}

func (deferQuery) Numbers(ctx context.Context) ([]int, error) {
	return []int{10, 20, 30}, nil
}

func newDeferServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := NewServer(WithSchema(schema.Config{Query: deferQuery{}}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestIncrementalDeliveryDeferOverHTTP(t *testing.T) {
	srv := newDeferServer(t)

	body := `{"query":"{ user(id: \"1\") { id ... on deferUser @defer { name } } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("Content-Type = %q, want multipart/mixed", ct)
	}

	out := rec.Body.String()
	// Initial part carries id and hasNext:true; the deferred part carries name.
	if !strings.Contains(out, `"id":"1"`) {
		t.Fatalf("expected initial id in body:\n%s", out)
	}
	if !strings.Contains(out, `"hasNext":true`) {
		t.Fatalf("expected hasNext:true in body:\n%s", out)
	}
	if !strings.Contains(out, `"name":"Ada"`) {
		t.Fatalf("expected deferred name in body:\n%s", out)
	}
	if !strings.Contains(out, `"hasNext":false`) {
		t.Fatalf("expected terminal hasNext:false in body:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\r\n"), "----") {
		t.Fatalf("expected closing boundary, body tail:\n%q", out)
	}
}

func TestIncrementalDeliveryStreamOverHTTP(t *testing.T) {
	srv := newDeferServer(t)

	body := `{"query":"{ numbers @stream(initialCount: 1) }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !strings.Contains(out, `"numbers":[10]`) {
		t.Fatalf("expected initial single item, body:\n%s", out)
	}
	// Streamed items 20 and 30 must each appear in an incremental part.
	if !strings.Contains(out, "20") || !strings.Contains(out, "30") {
		t.Fatalf("expected streamed items 20 and 30, body:\n%s", out)
	}
}

func TestNonIncrementalRequestStaysSingleJSON(t *testing.T) {
	srv := newDeferServer(t)

	// No @defer/@stream: even with multipart/mixed Accept, a normal JSON body
	// is returned.
	body := `{"query":"{ user(id: \"1\") { id name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("did not expect multipart for non-incremental op, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"name":"Ada"`) {
		t.Fatalf("expected name inlined in single response: %s", rec.Body.String())
	}
}
