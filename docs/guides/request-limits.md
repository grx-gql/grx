---
title: Limits
description: HTTP payload caps, timeouts, persisted-query modes, executor selection safeguards - contain abusive graphs before resolver work dominates.
outline: deep
---

# Limits

Even well‑formed GraphQL documents can overwhelm memory or CPU (**alias bombs**, multi‑megabyte JSON bodies). Separate these controls from masking/auth (**[Security](/guides/production-security)**) and schema discovery knobs (**[Introspection](/guides/introspection)**).

---

## Payload & timeouts

| Control | Goal | **`grx` helper** (`grx.NewServer`) | Only **`server.Config`** |
| --- | --- | --- | --- |
| HTTP envelope | Oversized **`POST`** JSON bodies | **`WithMaxHTTPRequestBytes(n)`** | **`MaxHTTPRequestBytes`** |
| Variables JSON | Oversized **`variables`** payloads |  -  | **`MaxVariableBytes`** |
| Clock guard | Hanging resolvers | **`WithRequestTimeout(d)`** | **`RequestTimeout`** |

```go
package main

import (
	"log"
	"time"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/server"
)

func main() {
	var err error

	_, err = grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithRequestTimeout(15*time.Second),
		grx.WithMaxHTTPRequestBytes(1<<20),
		grx.WithMaxSelectionDepth(16),
	)
	if err != nil {
		log.Fatal(err)
	}

	_, err = server.New(server.Config{
		Schema:              graph.NewSchema(),
		RequestTimeout:      30 * time.Second,
		MaxHTTPRequestBytes: 2 << 20,
		MaxVariableBytes:    96 << 10,
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

---

## Persisted & trusted corpus

[**`WithPersistedQueries`**](https://pkg.go.dev/github.com/grx-gql/grx#WithPersistedQueries) maps **`sha256` → literal query**. Harden **`server.Config`** with **`RequirePersistedQuery`**, **`StrictPersistedQueries`**, **`TrustedDocuments`**, **`RejectUnknownVariables`** for closed corpora (**`server`** package tests demonstrate behaviour).

Extended procedure: **[Persisted queries (APQ)](/guides/persisted-queries)**.

---

## Document-shape safeguards

Executors reject parses that explode combinationally (**[Execution pipeline](/concepts/execution#document-shape-limits-and-parse-cache)**).

| Mechanism | Source |
| --- | --- |
| Total selections | **`server.Config.MaxSelectionCount`** |
| Aliases | **`MaxAliasCount`** |
| Parallel root fields | **`MaxRootFieldCount`** |
| Deepest nesting | **`grx.WithMaxSelectionDepth(n)`** / **`server.Config.MaxSelectionDepth`** |

```go
package main

import (
	"log"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx/server"
)

func main() {
	srv, err := server.New(server.Config{
		Schema:            graph.NewSchema(),
		MaxSelectionDepth: 16,
		MaxSelectionCount: 750,
		MaxAliasCount:     200,
		MaxRootFieldCount: 10,
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

Patterns + rationale: **[Query & document limits](/guides/query-limits)**.

::: warning Limits ≠ deterministic cost modelling  

Counters are guardrails - not Apollo-style deterministic cost engines.

:::
