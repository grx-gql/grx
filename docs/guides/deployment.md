---
title: Deployment
description: Ship a grx GraphQL server in containers, behind reverse proxies, on Kubernetes, or managed platforms—plus graceful shutdown and health checks.
outline: deep
---

# Deployment

**`grx.Server`** is a plain [`http.Handler`](https://pkg.go.dev/net/http#Handler). Anything that runs a Go **`net/http`** service applies here: build a static binary (typical), listen on **`0.0.0.0`** in containers, terminate TLS at the edge, and front it with a reverse proxy when you need rate limits, WebSocket upgrades, or multi-service routing.

::: tip Production checklist  

Before traffic hits the Internet: **[Security](/guides/production-security)** · **[Introspection](/guides/introspection)** · **[Limits](/guides/request-limits)** · subscriptions **[Securing subscriptions](/guides/subscriptions#securing-subscriptions)** · **[Authentication & authorization](/guides/auth)**  

:::

---

## Listen address & ports

Development often uses **`localhost`**. Inside Docker or Kubernetes you must bind the host interface containers expose:

```go
package main

import (
	"log"
	"net/http"
	"os"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func listeningAddr(defaultAddr string) string {
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		return v
	}
	return defaultAddr
}

func main() {
	addr := listeningAddr(":4000")
	srv, err := grx.NewServer(grx.WithSchema(graph.NewSchema()))
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(addr, srv))
}
```

Use **`HTTP_ADDR=:4000`** (or **`127.0.0.1:4000`** behind a sidecar-only mesh) consistently in **`Dockerfile`**, Compose, and chart values.

---

## Graceful shutdown

`http.ListenAndServe` exits hard on **`SIGINT`**. Prefer [`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown) so in-flight GraphQL requests and draining WebSockets get a bounded window:

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath(""), // production: disable playground
	)
	if err != nil {
		log.Fatal(err)
	}

	addr := ":4000"
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		addr = v
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
```

Tune the shutdown timeout to match your slowest resolver or WebSocket policy.

---

## Health checks

There is **no** built-in **`/healthz`** on **`grx.Server`**. Add a tiny handler on a parent [`ServeMux`](https://pkg.go.dev/net/http#ServeMux) (or your router) **beside** the GraphQL handler:

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	srv, err := grx.NewServer(grx.WithSchema(graph.NewSchema()))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", srv)

	log.Fatal(http.ListenAndServe(":4000", mux))
}
```

For **readiness** that depends on PostgreSQL/Redis, return **`503`** until dependencies respond—load balancers and orchestrators use that signal.

---

## Docker

### Multi-stage image

Typical **static** Linux binary (adjust **`./cmd/api`** to your main package path):

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/api /api
ENV HTTP_ADDR=:4000
EXPOSE 4000
ENTRYPOINT ["/api"]
```

- **`CGO_ENABLED=0`** keeps a portable static binary (matches most **`grx`** installs).
- **Distroless** shrinks attack surface; **Alpine** `FROM alpine:3.21` + **`apk add ca-certificates`** is fine if you shell in for debugging.

### `.dockerignore`

Avoid sending **`docs/node_modules`**, **`.git`**, and local artifacts into the build context:

```text
.git
docs/node_modules
docs/.vitepress/dist
**/*_test.go
```

Omit `**/*_test.go` if you run `go test` inside the Docker build stage; otherwise drop that line entirely and rely on CI for tests.

### Compose (API + Redis for pub/sub)

If you use **[`pkg/pubsub/redis`](/reference/pkg/pubsub/redis/)** for multi-replica subscriptions, wire the same network and env vars your Go code expects:

```yaml
services:
  api:
    build: .
    ports:
      - "4000:4000"
    environment:
      HTTP_ADDR: ":4000"
      REDIS_ADDR: "redis:6379"
    depends_on:
      - redis
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

---

## Reverse proxies

### NGINX (HTTP + WebSocket upgrade)

GraphQL **`POST`** and WebSocket upgrades often share a **`location`**. Forward **`Connection`** / **`Upgrade`** headers and raise **`proxy_read_timeout`** for long-lived streams:

```nginx
upstream grx_api {
    server api:4000;
}

server {
    listen 443 ssl;
    server_name api.example.com;

    location /graphql {
        proxy_pass http://grx_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
    }
}
```

If you split subscriptions with **`grx.WithSubscriptionPath("/graphql/ws")`**, mirror that path (or use one `location` prefix that covers both).

### Caddy (automatic HTTPS)

A minimal **`Caddyfile`** block:

```text
api.example.com {
    reverse_proxy api:4000
}
```

Enable WebSocket proxying (Caddy v2 passes upgrades by default for `reverse_proxy`); verify timeouts for SSE/WebSocket in your environment.

---

## Kubernetes

Sketch only—adapt namespaces, probes, and resources to your org:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grx-api
spec:
  replicas: 2
  selector:
    matchLabels: { app: grx-api }
  template:
    metadata:
      labels: { app: grx-api }
    spec:
      containers:
        - name: api
          image: your-registry/grx-api:latest
          ports:
            - containerPort: 4000
          env:
            - name: HTTP_ADDR
              value: ":4000"
          readinessProbe:
            httpGet: { path: /healthz, port: 4000 }
            initialDelaySeconds: 3
          livenessProbe:
            httpGet: { path: /healthz, port: 4000 }
---
apiVersion: v1
kind: Service
metadata:
  name: grx-api
spec:
  selector: { app: grx-api }
  ports:
    - port: 80
      targetPort: 4000
```

Use an **Ingress** controller that supports WebSocket pass-through (most do when annotations match your cloud). For **sticky sessions**, only some subscription topologies need it—**Redis pub/sub** often removes the requirement at the app layer.

---

## Managed platforms & serverless

| Style | Notes |
| --- | --- |
| **VM / systemd** | Run the binary behind **`ExecStart=`**; put **Caddy**/NGINX on the same host or in front for TLS. |
| **Fly.io, Railway, Render** | Set **`HTTP_ADDR`**, public port mapping, and scale horizontally; enable WebSocket support in the product settings if subscriptions are used. |
| **AWS ECS / Fargate, GCP Cloud Run (containers)** | Cloud Run enforces **request timeouts**—long **GraphQL subscriptions** or **SSE** may need **GKE/Compute** or **ECS** instead; verify limits before shipping streams. |
| **AWS Lambda + API Gateway** | HTTP **queries/mutations** can work with a Go Lambda adapter; **WebSocket subscriptions** are a different API Gateway/WebSocket API design—do not assume `grx.WithTransports` maps 1:1 without extra bridge code. |

When in doubt, choose a **container** or **VM** profile if **WebSocket** or **hour-long SSE** clients are first-class.

---

## See also

- **[Transports](/concepts/transports)** — how HTTP, WebSocket, and SSE mount on **`http.Handler`**
- **[Subscriptions](/guides/subscriptions)** — paths, `CheckOrigin`, **`OnConnect`**
- **[Production hardening hub](/concepts/graphql-security-production)** — links to security, introspection, limits
