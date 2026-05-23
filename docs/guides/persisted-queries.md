---
title: Persisted queries (APQ)
description: Ship SHA-256 registered queries and optional hardened modes—reduce payload variance and tame public POST surfaces.
outline: deep
---

# Persisted queries (Automatic Persisted Queries, APQ)

Clients send **`extensions.persistedQuery`** carrying a **`sha256Hash`** digest; servers look up **`hash → GraphQL query text`** and execute as if the full document shipped.

## Quick wiring with **`grx.NewServer`**

```go
package main

import (
	"log"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	queries := map[string]string{
		// Example: SHA-256(lowercase hex) of UTF-8 `query { ping }` via `printf '%s' 'query { ping }' | shasum -a 256`.
		"d7b0dfafc61a1f0618f4f346911d5aa87bef97b134f2943383223bdac4410134": `query { ping }`,
	}

	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPersistedQueries(queries),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

The default HTTP transport matches known digests (**case-insensitive hex**) and substitutes **`core.Request`** text before **`Execute`** runs.

Stronger closures—**reject anything not APQ‑shaped**, **verify hash matches inlined body**, or **full trusted corpus** mapping—sit on **`[server.Config](https://pkg.go.dev/github.com/patrickkabwe/grx/server#Config)`** (`RequirePersistedQuery`, **`StrictPersistedQueries`**, **`TrustedDocuments`**). Those fields map into the same **`pkg/http`** decoding path—see **[Limits → Persisted & trusted corpus](/guides/request-limits#persisted-trusted-corpus)**.

::: tip Companion reading  
Treat APQ alongside **timeouts**, **`MaxHTTPRequestBytes`**, and **`WithDisableIntrospection`** whenever you tighten a public **`POST`** graph endpoint—pair **[Limits](/guides/request-limits)**, **[Introspection](/guides/introspection)**, and **[Security](/guides/production-security)**.

:::

## See also

- **[Limits — persisted & trusted corpus](/guides/request-limits#persisted-trusted-corpus)**
- Roadmap ✅ automatic persisted queries in **[Roadmap](/roadmap)**
