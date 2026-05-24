.DEFAULT_GOAL := help

GO            ?= go
COVER_MIN     ?= 90
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
	cd redis-pubsub && $(GO) test ./...

.PHONY: test-race
test-race: ## go test -race all packages (root module + submodules).
	$(GO) test -race ./...
	cd redis-pubsub && $(GO) test -race ./...

.PHONY: test-cover-lib
test-coverage: ## Library packages only (exclude ./examples/...); enforce COVER_MIN %% stmts coverage each.
	env COVER_MIN=$(COVER_MIN) ./scripts/cover-lib.sh

.PHONY: vet
vet: ## go vet all packages.
	$(GO) vet ./...

.PHONY: fmt
fmt: ## gofmt every Go file in place.
	gofmt -w .

## ---- Go modules -----------------------------------------------------------

.PHONY: mod-tidy
mod-tidy: ## go mod tidy (root module, benchmark/, redis-pubsub).
	$(GO) mod tidy
	cd benchmark && $(GO) mod tidy
	cd redis-pubsub && $(GO) mod tidy

## ---- Git hooks -----------------------------------------------------------

.PHONY: install-hooks uninstall-hooks dev-setup
dev-setup: ## go mod tidy everywhere, then git hooks (local bootstrap after clone).
	@$(MAKE) mod-tidy
	@$(MAKE) install-hooks

install-hooks: ## Point core.hooksPath at .githooks (commit-msg conventional check).
	git config core.hooksPath .githooks

uninstall-hooks: ## Undo install-hooks for this repo (removes core.hooksPath).
	git config --unset-all core.hooksPath || true

## ---- Docs (VitePress) ----------------------------------------------------

.PHONY: docs-install
docs-install: ## Install docs site dependencies (uses bun).
	cd $(DOCS_DIR) && $(BUN) install

.PHONY: docs-roadmap
docs-roadmap: ## Mirror ROADMAP.md into the docs site as a roadmap.
	./scripts/sync-roadmap.sh

.PHONY: docs-content
docs-content: docs-roadmap ## Regenerate mirrored docs pages (roadmap only).

.PHONY: docs-dev
docs-dev: docs-content ## Run the docs dev server with HMR (http://localhost:4321/grx/).
	cd $(DOCS_DIR) && $(BUN) run dev

.PHONY: docs-build
docs-build: docs-content ## Build the static docs site into docs/.vitepress/dist.
	cd $(DOCS_DIR) && $(BUN) run build

.PHONY: docs-preview
docs-preview: ## Preview the built site (run after docs-build).
	cd $(DOCS_DIR) && $(BUN) run preview

.PHONY: docs-pkgsite
docs-pkgsite: ## Run pkgsite locally (the engine behind pkg.go.dev) on :6060.
	$(GO) run $(PKGSITE_PKG) -http=127.0.0.1:6060 .

.PHONY: docs-clean
docs-clean: ## Remove the built docs site and node_modules.
	rm -rf $(DOCS_DIR)/.vitepress/dist $(DOCS_DIR)/.vitepress/cache \
		$(DOCS_DIR)/node_modules

## ---- Benchmark & profiling ----------------------------------------------

PROFILE_DIR ?= .profiles

.PHONY: benchmark
benchmark: ## Comparative GraphQL benchmarks (benchmark/ sibling module, -benchmem).
	cd benchmark && $(GO) test -bench=. -benchmem ./...

.PHONY: benchmark-race
benchmark-race: ## Comparative benchmarks under the race detector.
	cd benchmark && $(GO) test -race -bench=. ./...

.PHONY: benchmark-mem
benchmark-mem: ## Alias for benchmark-mem-prof (writes .profiles/benchmark-mem.prof).
	@$(MAKE) benchmark-mem-prof

.PHONY: benchmark-mem-prof
benchmark-mem-prof: ## Comparative benchmarks + heap allocation profile (.profiles/).
	@mkdir -p $(PROFILE_DIR)
	cd benchmark && $(GO) test -bench=. -benchmem -memprofile=../$(PROFILE_DIR)/benchmark-mem.prof ./...

.PHONY: profile-exec-lex-cpu
profile-exec-lex-cpu: ## CPU profile exec.BenchmarkLex into .profiles/exec-lex.cpu.prof.
	@mkdir -p $(PROFILE_DIR)
	$(GO) test ./exec -run=^$$ -bench=BenchmarkLex -cpuprofile=$(PROFILE_DIR)/exec-lex.cpu.prof

.PHONY: profile-bench-simple-grx-cpu
profile-bench-simple-grx-cpu: ## CPU profile benchmark PersistedCompound/grx (.profiles/).
	@mkdir -p $(PROFILE_DIR)
	cd benchmark && $(GO) test -run=^$$ -bench='BenchmarkPersistedCompound/grx' \
		-cpuprofile=../$(PROFILE_DIR)/bench-persisted-grx.cpu.prof ./...