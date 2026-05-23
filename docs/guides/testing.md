---
title: Testing with the HTTP client
description: Black-box integration tests against grx handlers using net/http/httptest and pkg/client (GraphQL-over-HTTP).
outline: deep
---

# Testing with the HTTP client

The recommended way to test a **`grx` HTTP surface** “for real” is to stand up the same **`http.Handler`** you ship (`grx.NewServer` / **`server.New`**) behind [**`httptest.Server`**](https://pkg.go.dev/net/http/httptest#NewServer) and drive it with **`github.com/patrickkabwe/grx/pkg/client`**.

That path matches browsers and **`curl`** more closely than calling **`exec.Executor`** directly, so you validate routing, transports, middleware, **`Content-Type` / `Accept`**, and serialization end-to-end.

::: tip Canonical sample  

[`server/query_test.go`](https://github.com/patrickkabwe/grx/blob/main/server/query_test.go) and [`server/server_test.go`](https://github.com/patrickkabwe/grx/blob/main/server/server_test.go) (**`execGraphQL`**, **`wrapServerInHarness`**) mirror the patterns below—including custom headers via **`WithRequestHeader`**.

:::

## Minimal harness

[`client.New`](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#New) expects the **full URL to the GraphQL path** (`scheme` + host + **`GraphQLPath`**). With **`httptest`**, concatenate the server base URL with **[`*server.Server.GraphqlPath`](https://pkg.go.dev/github.com/patrickkabwe/grx/server#Server.GraphqlPath)** (defaults normalize to **`/graphql`**):

```go
package graph_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/pkg/client"
)

func TestGraphQLPing(t *testing.T) {
	srv, err := grx.NewServer(grx.WithSchema(graph.NewSchema()))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	c := client.New(ts.URL + srv.GraphqlPath)
	resp, err := c.Exec(context.Background(), &client.Request{
		Query: `{ __typename }`,
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("graphql errors: %+v", resp.Errors)
	}
}
```

The **`Response`** envelope is **`[core.Response](https://pkg.go.dev/github.com/patrickkabwe/grx/core#Response)** (`Data`, **`Errors`**, optional incremental payloads). **`Exec`** sets **`error`** only for **transport** problems (DNS, connection refused, malformed JSON)—**GraphQL failures** arrive with **`resp.Errors` nonempty** instead.

## Variables and operation names

```go
package graph_test

import (
	"context"

	"github.com/patrickkabwe/grx/pkg/client"
)

// c is built with client.New in your httptest harness.
func ExampleVariables(c *client.Client) error {
	resp, err := c.Exec(context.Background(), &client.Request{
		OperationName: "GetUser",
		Query: `
      query GetUser($id: String!) {
        user(id: $id) { id name }
      }`,
		Variables: map[string]any{"id": "user_42"},
	})
	_ = resp
	return err
}
```

**`client.New`** expects the GraphQL endpoint URL (**`scheme` + host + **`GraphQLPath`**), as shown in Minimal harness above.

## Authenticated requests

Reuse the same Bearer/cookie/session headers your routers expect via **[`client.WithRequestHeader`](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#WithRequestHeader)**:

```go
package graph_test

import (
	"context"

	"github.com/patrickkabwe/grx/pkg/client"
)

// c is built with client.New in your httptest harness; tok is your bearer string.
func exampleAuth(c *client.Client, tok string) error {
	resp, err := c.Exec(context.Background(), &client.Request{Query: `{ me { id } }`},
		client.WithRequestHeader("Authorization", "Bearer "+tok),
	)
	_ = resp
	return err
}
```

[`client.WithHTTPClient`](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#WithHTTPClient) swaps in a traced or short-timeout **`http.Client`** for table tests.

## Persisted queries (extensions)

[**`client.Request.Extensions`**](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#Request) maps to the JSON **`extensions`** object—wire APQ payloads the same way a browser client would (**[persisted-queries guide](/guides/persisted-queries)**).

## When you need HTTP details

[**`Exec`**](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#Client.Exec) decodes **`200`** bodies into **`core.Response`** but discards **`http.Response`**.

Use **`[PostGraphQL](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client#Client.PostGraphQL)`** when you must assert **`StatusCode`** (for example malformed JSON envelopes, gateway-only behaviour):

```go
package graph_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/patrickkabwe/grx/pkg/client"
)

func ExamplePostGraphQL(t *testing.T, c *client.Client, ctx context.Context) {
	payload, err := json.Marshal(client.Request{Query: "{ __typename }"})
	if err != nil {
		t.Fatal(err)
	}
	httpResp, err := c.PostGraphQL(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", httpResp.StatusCode, body)
	}

	var gql client.Response // alias of core.Response
	if err := json.Unmarshal(body, &gql); err != nil {
		t.Fatal(err)
	}
}
```

## Assertions on **`Data`**

`resp.Data` is **`any`** (typically **`map[string]any`** for objects). Narrow with type asserts or marshal round-trip:

```go
package graph_test

import (
	"testing"

	"github.com/patrickkabwe/grx/pkg/client"
)

func ExampleAssertions(t *testing.T, resp *client.Response) {
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type %T", resp.Data)
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("user field %T", data["user"])
	}
	_ = user
}
```

Alternatively **`json.Marshal` / `json.Unmarshal`** into typed structs mirrors production JSON codecs.

## What **`pkg/client` is not**

- **`pkg/client`** speaks **HTTP `POST`** only. **WebSocket** / **SSE** subscriptions need [`graphql-ws`](https://github.com/enisdenjo/graphql-ws) (or equivalent) **or** in-process **`Executor.Subscribe`** tests—outside this package (**[Realtime subscriptions](/guides/subscriptions)**).
- It does **not** replace **`go test`** **unit tests** beside **`exec`**, **`schema`**, and **`resolver`** helpers when you isolate parsing/validation behaviour without HTTP.

## See also

- **[Queries and mutations](/guides/query-mutation-server)** — building the schema under test  
- **[pkg/client](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/client)** on pkg.go.dev
