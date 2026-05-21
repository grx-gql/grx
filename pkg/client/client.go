// Package client is a small GraphQL-over-HTTP client for grx servers. It
// sends the same Accept and Content-Type headers as the bundled GraphiQL
// playground so server-side content negotiation and JSON responses match
// production browser clients.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/patrickkabwe/grx/core"
)

// DefaultAccept is sent on each request so the server prefers
// application/graphql-response+json per GraphQL-over-HTTP.
const DefaultAccept = "application/graphql-response+json, application/json;q=0.9"

// RequestOption mutates an outgoing HTTP request before it is sent.
type RequestOption func(*http.Request)

// WithRequestHeader sets a header on the outgoing request (after defaults).
func WithRequestHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// Option configures a [Client].
type Option func(*Client)

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithAccept overrides the Accept header (empty restores [DefaultAccept]).
func WithAccept(accept string) Option {
	return func(c *Client) {
		c.accept = accept
	}
}

// Client posts GraphQL operations to a single HTTP endpoint (for example
// https://api.example.com/graphql).
type Client struct {
	url        string
	httpClient *http.Client
	accept     string
}

// New builds a client that POSTs to url (the full GraphQL path, including
// scheme, host, and path).
func New(url string, opts ...Option) *Client {
	c := &Client{
		url:        strings.TrimSpace(url),
		httpClient: http.DefaultClient,
		accept:     DefaultAccept,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Request is the JSON body shape for a GraphQL-over-HTTP POST.
type Request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// FromCore maps a transport-level [core.Request] into a wire request.
func FromCore(req core.Request) Request {
	return Request{
		Query:         req.Query,
		OperationName: req.OperationName,
		Variables:     req.Variables,
	}
}

// Execute marshals req as JSON, POSTs it, and decodes a single [core.Response].
func (c *Client) Execute(ctx context.Context, req Request, opts ...RequestOption) (core.Response, *http.Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return core.Response{}, nil, err
	}
	httpResp, err := c.PostGraphQL(ctx, payload, opts...)
	if err != nil {
		return core.Response{}, httpResp, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return core.Response{}, httpResp, err
	}

	var gql core.Response
	if err := json.Unmarshal(body, &gql); err != nil {
		return core.Response{}, httpResp, fmt.Errorf("decode GraphQL response: %w", err)
	}
	return gql, httpResp, nil
}

// PostGraphQL sends a raw JSON body (a single object or batched array) with
// GraphQL-over-HTTP POST defaults. The caller must close resp.Body when err
// is nil.
func (c *Client) PostGraphQL(ctx context.Context, body []byte, opts ...RequestOption) (*http.Response, error) {
	if strings.TrimSpace(c.url) == "" {
		return nil, fmt.Errorf("client: empty endpoint URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", core.MediaTypeJSON)
	accept := strings.TrimSpace(c.accept)
	if accept == "" {
		accept = DefaultAccept
	}
	req.Header.Set("Accept", accept)
	for _, opt := range opts {
		opt(req)
	}
	return c.httpClient.Do(req)
}
