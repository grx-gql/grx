---
title: Query & document limits
description: Cap selection depth, aliases, and root fields before resolvers run - complementing HTTP body limits and timeouts.
outline: deep
---

# Query & document limits

GraphQL’s flexibility means a single **`POST`** may reference thousands of selections or fan out combinatorially (**alias bombs**, extremely deep fragments). grx rejects abusive shapes **during parse/validation** via executor options usually driven from **`server.New(server.Config{…})`**:

| Field | What it limits |
| --- | --- |
| **`MaxSelectionCount`** | Total field selections in the operation |
| **`MaxAliasCount`** | Number of aliased fields |
| **`MaxRootFieldCount`** | Parallel top-level selections |
| **`exec.WithMaxSelectionDepth`** | Nested selection depth *(only when assembling **[`exec.Executor`](https://pkg.go.dev/github.com/grx-gql/grx/exec#New)** yourself today - **not** surfaced on **`grx.With…` yet**)* |

::: warning Not a full “cost engine”  
Counters are **cheap guard rails**, not GraphQL cost analysis (no per-type weights). Pair with **gateway rate limits**, **`WithRequestTimeout`**, and **APQ**/trusted documents for public APIs.

:::

HTTP payload caps and operational walkthrough: **[Limits](/guides/request-limits)**; masking and gateway hooks: **[Security](/guides/production-security)**. Document-shape mechanics: **[Execution pipeline](/concepts/execution#document-shape-limits-and-parse-cache)**.

## Example

```go
package main

import (
	"log"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx/server"
)

func main() {
	srv, err := server.New(server.Config{
		Schema:             graph.NewSchema(),
		MaxSelectionCount:  750,
		MaxAliasCount:      120,
		MaxRootFieldCount:  8,
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

## Related

- **[Persisted queries](/guides/persisted-queries)**  -  shrinking unknown document surface
