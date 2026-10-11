package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roadmapFailingParsePlugin struct{ plugins.Base }

func (roadmapFailingParsePlugin) ParsingStart(context.Context, core.Request) error {
	return errors.New("private service credential")
}

func TestRoadmapHookErrorsAreMasked(t *testing.T) {
	srv, err := New(Config{
		Schema:             schema.Config{Query: sdlTestQuery{}},
		Plugins:            []plugins.Plugin{roadmapFailingParsePlugin{}},
		MaskInternalErrors: true,
		ClientErrorMessage: "hidden",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var response core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Errors) != 1 || response.Errors[0].Message != "hidden" {
		t.Fatalf("internal hook error was not masked: %+v", response.Errors)
	}
}

type failingLifecyclePlugin struct {
	plugins.Base
	phase    string
	observed []error
}

func (p *failingLifecyclePlugin) failure(phase string) error {
	if p.phase == phase {
		return errors.New("private hook detail")
	}
	return nil
}
func (p *failingLifecyclePlugin) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	return ctx, p.failure("request")
}
func (p *failingLifecyclePlugin) ParsingStart(context.Context, core.Request) error {
	return p.failure("parse")
}
func (p *failingLifecyclePlugin) ValidationStart(context.Context, core.Request) error {
	return p.failure("validate")
}
func (p *failingLifecyclePlugin) ExecutionStart(context.Context, core.Request) error {
	return p.failure("execute")
}
func (p *failingLifecyclePlugin) FieldResolveStart(context.Context, plugins.FieldContext) error {
	return p.failure("field")
}
func (p *failingLifecyclePlugin) ResponseSend(context.Context, core.Response) error {
	return p.failure("response")
}
func (p *failingLifecyclePlugin) Error(ctx context.Context, err error) {
	p.observed = append(p.observed, err)
}

func TestLifecycleErrorsMaskClientsAndPreserveCause(t *testing.T) {
	for _, phase := range []string{"request", "parse", "validate", "execute", "field", "response"} {
		for _, mask := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mask=%t", phase, mask), func(t *testing.T) {
				plugin := &failingLifecyclePlugin{phase: phase}
				srv, err := New(Config{Schema: schema.Config{Query: sdlTestQuery{}}, Plugins: []plugins.Plugin{plugin}, MaskInternalErrors: mask, ClientErrorMessage: "hidden"})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ hello }"}`))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, req)
				var response core.Response
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				want := "private hook detail"
				if mask {
					want = "hidden"
				}
				if len(response.Errors) != 1 || response.Errors[0].Message != want {
					t.Fatalf("errors=%+v want=%s", response.Errors, want)
				}
				preserved := false
				for _, err := range plugin.observed {
					preserved = preserved || err.Error() == "private hook detail"
				}
				if !preserved {
					t.Fatalf("original error lost: %v", plugin.observed)
				}
			})
		}
	}
}
