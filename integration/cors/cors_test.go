package cors_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ginCors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	fiberCors "github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/schema"
	stdCors "github.com/rs/cors"
)

type query struct{}

func (query) Hello() string { return "hello" }

type serve func(*http.Request) (*http.Response, error)

func TestNetHTTPExternalCORS(t *testing.T) {
	graphql := newGraphQLHandler(t)
	handler := stdCors.New(stdCors.Options{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler(graphql)

	assertCORS(t, func(request *http.Request) (*http.Response, error) {
		return serveNetHTTP(handler, request)
	})
}

func TestFiberExternalCORS(t *testing.T) {
	graphql := newGraphQLHandler(t)
	app := fiber.New()
	app.Use(fiberCors.New(fiberCors.Config{
		AllowOrigins: "https://app.example.com",
		AllowMethods: "POST,OPTIONS",
		AllowHeaders: "Content-Type",
	}))
	app.All("/graphql", adaptor.HTTPHandler(graphql))

	assertCORS(t, func(request *http.Request) (*http.Response, error) {
		return app.Test(request)
	})
}

func TestGinExternalCORS(t *testing.T) {
	graphql := newGraphQLHandler(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginCors.New(ginCors.Config{
		AllowOrigins: []string{"https://app.example.com"},
		AllowMethods: []string{http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{"Content-Type"},
	}))
	router.Any("/graphql", gin.WrapH(graphql))

	assertCORS(t, func(request *http.Request) (*http.Response, error) {
		return serveNetHTTP(router, request)
	})
}

func TestChiExternalCORS(t *testing.T) {
	graphql := newGraphQLHandler(t)
	router := chi.NewRouter()
	router.Use(stdCors.New(stdCors.Options{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler)
	router.Mount("/graphql", graphql)

	assertCORS(t, func(request *http.Request) (*http.Response, error) {
		return serveNetHTTP(router, request)
	})
}

func newGraphQLHandler(t *testing.T) http.Handler {
	t.Helper()
	server, err := grx.NewServer(grx.WithSchema(schema.Config{Query: query{}}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return server
}

func assertCORS(t *testing.T, serveRequest serve) {
	t.Helper()

	preflight := httptest.NewRequest(http.MethodOptions, "http://api.example.com/graphql", nil)
	preflight.Header.Set("Origin", "https://app.example.com")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "content-type")
	response, err := serveRequest(preflight)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("preflight allow origin = %q", got)
	}

	request := httptest.NewRequest(http.MethodPost, "http://api.example.com/graphql", bytes.NewBufferString(`{"query":"{ hello }"}`))
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Content-Type", "application/json")
	response, err = serveRequest(request)
	if err != nil {
		t.Fatalf("GraphQL request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GraphQL status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("GraphQL allow origin = %q", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read GraphQL response: %v", err)
	}
	if !strings.Contains(string(body), `"hello":"hello"`) {
		t.Fatalf("GraphQL response = %s", body)
	}
}

func serveNetHTTP(handler http.Handler, request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Result(), nil
}
