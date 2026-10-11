package grx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grx-gql/grx/schema"
)

type concurrentRootQuery struct {
	started chan struct{}
	release chan struct{}
}

func (q concurrentRootQuery) Wait(ctx context.Context) (string, error) {
	q.started <- struct{}{}
	select {
	case <-q.release:
		return "ok", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestQueryConcurrencyOptionReachesHTTPExecutor(t *testing.T) {
	query := concurrentRootQuery{started: make(chan struct{}, 2), release: make(chan struct{})}
	srv, err := NewServer(WithSchema(schema.Config{Query: query}), WithQueryConcurrency(2))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ a:wait b:wait }"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); srv.ServeHTTP(rec, req) }()
	for range 2 {
		select {
		case <-query.started:
		case <-ctx.Done():
			t.Fatal("HTTP root resolvers did not overlap")
		}
	}
	close(query.release)
	<-done
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"data":{"a":"ok","b":"ok"}`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
