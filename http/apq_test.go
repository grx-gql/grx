package http_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestRoadmapAutomaticPersistedQueryRegistration(t *testing.T) {
	const query = "{ hello }"
	sum := sha256.Sum256([]byte(query))
	hash := hex.EncodeToString(sum[:])
	transport := grxhttp.New(grxhttp.Config{APQCacheSize: 2})
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
	extensions := fmt.Sprintf(`"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}`, hash)
	registration := postGraphQLRequest("/graphql", fmt.Sprintf(`{"query":%q,%s}`, query, extensions))
	registered := httptest.NewRecorder()
	transport.Serve(registered, registration, executor)
	if registered.Code != 200 {
		t.Fatalf("registration status=%d body=%s", registered.Code, registered.Body.String())
	}
	lookup := postGraphQLRequest("/graphql", "{"+extensions+"}")
	resolved := httptest.NewRecorder()
	transport.Serve(resolved, lookup, executor)
	if resolved.Code != 200 || !strings.Contains(resolved.Body.String(), `"hello":"world"`) {
		t.Fatalf("registered hash was not persisted: status=%d body=%s", resolved.Code, resolved.Body.String())
	}
}

func TestAutomaticPersistedQueryProtocol(t *testing.T) {
	transport := grxhttp.New(grxhttp.Config{APQCacheSize: 1})
	send := func(query, hash string, get bool) *httptest.ResponseRecorder {
		t.Helper()
		extensions := fmt.Sprintf(`{"persistedQuery":{"version":1,"sha256Hash":%q}}`, hash)
		req := postGraphQLRequest("/graphql", fmt.Sprintf(`{"query":%q,"extensions":%s}`, query, extensions))
		if get {
			req = httptest.NewRequest("GET", "/graphql?"+url.Values{"extensions": {extensions}}.Encode(), nil)
		}
		rec := httptest.NewRecorder()
		transport.Serve(rec, req, &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}})
		return rec
	}
	hash := func(query string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(query))) }
	first, second := "{ first }", "{ second }"
	if rec := send("", hash(first), false); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"code":"PERSISTED_QUERY_NOT_FOUND"`) {
		t.Fatalf("miss: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(first, hash(second), false); rec.Code != 400 {
		t.Fatalf("mismatched hash registered: %s", rec.Body.String())
	}
	for _, query := range []string{first, first, second} {
		if rec := send(query, hash(query), false); rec.Code != 200 {
			t.Fatalf("register: %s", rec.Body.String())
		}
	}
	if rec := send("", hash(first), true); rec.Code != 400 {
		t.Fatalf("entry was not evicted: %s", rec.Body.String())
	}
	if rec := send("", hash(second), true); rec.Code != 200 {
		t.Fatalf("GET lookup: %s", rec.Body.String())
	}
	large := "{ hello }" + strings.Repeat(" ", 64<<10)
	if rec := send(large, hash(large), false); rec.Code != 400 || !strings.Contains(rec.Body.String(), "cache limit") {
		t.Fatalf("unbounded query: %s", rec.Body.String())
	}
}

func TestAutomaticPersistedQueryDisabledAndStatic(t *testing.T) {
	const query = "{ hello }"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(query)))
	for _, size := range []int{0, -1, 1} {
		transport := grxhttp.New(grxhttp.Config{APQCacheSize: size, PersistedQueries: map[string]string{hash: query}})
		rec := httptest.NewRecorder()
		transport.Serve(rec, postGraphQLRequest("/graphql", fmt.Sprintf(`{"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}}`, hash)), &stubExecutor{})
		if rec.Code != 200 {
			t.Fatalf("static query size=%d: %s", size, rec.Body.String())
		}
	}
	transport := grxhttp.New()
	for _, queryText := range []string{query, ""} {
		rec := httptest.NewRecorder()
		transport.Serve(rec, postGraphQLRequest("/graphql", fmt.Sprintf(`{"query":%q,"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}}`, queryText, hash)), &stubExecutor{})
		if queryText == "" && rec.Code != 400 {
			t.Fatal("disabled cache registered query")
		}
	}
}

func TestAutomaticPersistedQueryConcurrent(t *testing.T) {
	transport := grxhttp.New(grxhttp.Config{APQCacheSize: 8})
	var workers sync.WaitGroup
	for i := range 32 {
		workers.Go(func() {
			query := fmt.Sprintf("{ field%d }", i)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(query)))
			for range 10 {
				for _, text := range []string{query, ""} {
					rec := httptest.NewRecorder()
					transport.Serve(rec, postGraphQLRequest("/graphql", fmt.Sprintf(`{"query":%q,"extensions":{"persistedQuery":{"version":1,"sha256Hash":%q}}}`, text, hash)), &stubExecutor{})
					if rec.Code != 200 && !(text == "" && rec.Code == 400 && strings.Contains(rec.Body.String(), "PERSISTED_QUERY_NOT_FOUND")) {
						t.Errorf("concurrent request: %d %s", rec.Code, rec.Body.String())
					}
				}
			}
		})
	}
	workers.Wait()
}
