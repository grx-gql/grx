.DEFAULT_GOAL := help

GO            ?= go
BUN           ?= bun
DOCS_DIR      ?= docs
PKGSITE_PKG   := golang.org/x/pkgsite/cmd/pkgsite@latest

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z0-9_.-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

## ---- Run Examples --------------------------------------------------------

.PHONY: run-basic
run-basic: ## Run the basic example server.
	$(GO) run ./examples/basic

## ---- Build / test --------------------------------------------------------

.PHONY: build
build: ## go build all packages.
	$(GO) build ./...

.PHONY: test
test: ## go test all packages (root module + submodules).
	$(GO) test ./...
	cd pkg/pubsub/redis && $(GO) test ./...

.PHONY: test-race
test-race: ## go test -race all packages (root module + submodules).
	$(GO) test -race ./...
	cd pkg/pubsub/redis && $(GO) test -race ./...

.PHONY: vet
vet: ## go vet all packages.
	$(GO) vet ./...

.PHONY: fmt
fmt: ## gofmt every Go file in place.
	gofmt -w .

## ---- Docs (Astro Starlight) ---------------------------------------------

.PHONY: docs-install
docs-install: ## Install docs site dependencies (uses bun).
	cd $(DOCS_DIR) && $(BUN) install

.PHONY: docs-changelog
docs-changelog: ## Mirror CHANGELOG.md into the docs site.
	./scripts/sync-changelog.sh

.PHONY: docs-roadmap
docs-roadmap: ## Mirror the README Feature Parity Checklist into the docs site as a roadmap.
	./scripts/sync-roadmap.sh

.PHONY: docs-content
docs-content: docs-changelog docs-roadmap ## Regenerate every auto-generated docs page (API ref + changelog + roadmap).

.PHONY: docs-dev
docs-dev: docs-content ## Run the docs site dev server with HMR (http://localhost:4321/grx).
	cd $(DOCS_DIR) && $(BUN) run dev

.PHONY: docs-build
docs-build: docs-content ## Build the static docs site into docs/dist.
	cd $(DOCS_DIR) && $(BUN) run build

.PHONY: docs-preview
docs-preview: ## Preview the built site (run after docs-build).
	cd $(DOCS_DIR) && $(BUN) run preview

.PHONY: docs-pkgsite
docs-pkgsite: ## Run pkgsite locally (the engine behind pkg.go.dev) on :6060.
	$(GO) run $(PKGSITE_PKG) -http=127.0.0.1:6060 .

.PHONY: docs-clean
docs-clean: ## Remove the built docs site and node_modules.
	rm -rf $(DOCS_DIR)/dist $(DOCS_DIR)/node_modules $(DOCS_DIR)/.astro

## ---- Benchmark -----------------------------------------------------------

.PHONY: benchmark
benchmark: ## Run the benchmarks.
	$(GO) test -bench=. ./...

.PHONY: benchmark-race
benchmark-race: ## Run the benchmarks with race detection.
	$(GO) test -race -bench=. ./...

.PHONY: benchmark-mem
benchmark-mem: ## Run the benchmarks with memory profiling.
	$(GO) test -bench=. -memprofile=mem.prof ./...