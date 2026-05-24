---
title: Security
description: Mask resolver errors, apply authorizers & rate-limit hooks, and keep gateway trust boundaries - beyond introspection knobs.
outline: deep
---

# Security

Public GraphQL sits on **`POST`** (plus optional streams). Prefer **HTTPS**, reverse-proxy **rate limits**, and **`Authorization`/session middleware** terminating **before** GraphQL unmarshals payloads.

Prefer **`grx.NewServer`** **`With*`** helpers; fields only on **`[server.Config](https://pkg.go.dev/github.com/grx-gql/grx/server#Config)`** require **`server.New`** (**same **`http.Handler`** ergonomics **`grx.NewServer`** uses internally).

::: tip Prerequisites  

Have **`schema.Config`** (**[Define your schema](/concepts/schema-basics)**). Identity propagation + Bearer patterns live under **[Authentication & authorization](/guides/auth)**.

:::

Companion guides:

- [**Introspection**](/guides/introspection)
- [**Limits**](/guides/request-limits)
- [**Deployment**](/guides/deployment) (containers, proxies, Kubernetes)

---

## Mask internal errors

Resolver/driver panics leaking raw strings into **`errors[].message`** is risky. [`MaskInternalErrors`](https://pkg.go.dev/github.com/grx-gql/grx/server#Config) on **`server.Config`** collapses internals to **`ClientErrorMessage`** (or the default string) while **`plugin`** hooks still observe the original error.

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
		MaskInternalErrors: true,
		ClientErrorMessage: "internal error", // omit for default wording
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

Discuss partial success styling in **[Errors & client responses](/guides/errors-and-masking)**.

---

## Authorization at the GraphQL boundary

Layer **`grx.WithOperationAuthorizer`** (parsed document gate) plus **`WithFieldAuthorizer`** (fine-grained per selection) atop HTTP identity - details + Bearer samples in **[Auth guide](/guides/auth)**.

**[`server.Config.RateLimiter`](https://pkg.go.dev/github.com/grx-gql/grx/server#Config)** hooks [**`exec.RateLimiter`](https://pkg.go.dev/github.com/grx-gql/grx/exec#RateLimiter)** implementations that reject workloads **before** resolver execution (**still after** lexical parse unless you choke earlier middleware).

::: warning Field authorizers ≠ auth spec sugar  

Patterns match many servers (Apollo “shield” analogues); **`WithFieldAuthorizer`** is **grx‑named**, not GraphQL‑spec vocabulary.

:::

## Checklist snapshot

| Area | Knob |
| --- | --- |
| Error hygiene | **`MaskInternalErrors`** + structured logging exporters |
| GraphQL‑aware authZ | **`WithOperationAuthorizer`**, **`WithFieldAuthorizer`** |
| Request storms | Proxies **`OR`** **`RateLimiter`** integration |
