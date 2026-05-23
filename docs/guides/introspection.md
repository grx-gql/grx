---
title: Introspection
description: Control schema discovery, bundled GraphiQL, and plaintext SDL endpoints when tightening deployments.
outline: deep
---

# Introspection & GraphiQL

Clients rely on **`__schema`**, **`__type(name: …)`**, and related fields to introspect the graph. Bundled GraphiQL issues those queries over **`GET`**, so **dev‑first defaults** (**on**) usually flip to **`off`** (or network‑segmented equivalents) alongside hardened **`POST`** traffic.

Companion guides: **[Security](/guides/production-security)** · **[Limits](/guides/request-limits)**

---

## Disable introspection

```go
package main

import (
	"log"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithDisableIntrospection(),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

Executors emit GraphQL **`errors`** for introspection-only operations whenever **`WithDisableIntrospection`** applies.

::: info Exposure beyond **`__schema`  

Git/SDL artifacts, codegen dumps, **`WithSchemaSDLPath`**, leaked mobile binaries—coordinate release hygiene independent of **`WithDisableIntrospection`** alone.

:::

## Turn off bundled GraphiQL

```go
package main

import (
	"log"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath(""), // no GET explorer
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

Assume **`POST /graphql`** is the supported API when production facing; host explorers internally (auth + SSO) whenever teams still crave UI assistance.

Keeping GraphiQL while disabling introspection is rarely productive—explorer panes stall without schema queries.

---

## SDL export mirrors

[**`WithSchemaSDLPath`](/reference/grx/)** publishes schema text **`GET`** when configured. Omit it—or wrap with auth/proxy ACLs—as part of tightening schema disclosure.
