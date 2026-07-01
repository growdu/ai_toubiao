# BidWriter Makefile
# Common development tasks. Run `make help` for the full list.

# ============================================================================
# Variables
# ============================================================================
SHELL := /bin/bash
.DEFAULT_GOAL := help

SERVICES := api-gateway project-svc document-svc workflow-svc knowledge-svc \
            router-svc template-svc billing-svc notify-svc audit-svc

# SERVICES_IMPL is the subset that has an actual go.mod / cmd/ directory.
# Used by build/test targets so missing services don't break the build.
SERVICES_IMPL := $(shell for s in $(SERVICES); do \
  if [ -f services/$$s/go.mod ] && [ -d services/$$s/cmd/$$s ]; then echo $$s; fi; \
done)

GO         := go
GOFLAGS    := -trimpath
DOCKER     := docker
COMPOSE    := docker compose
PNPM       := pnpm
MKDOCS     := mkdocs

# ============================================================================
# Help
# ============================================================================
.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ============================================================================
# Setup
# ============================================================================
.PHONY: setup
setup: ## Initial project setup (download deps, install tools)
	@echo "==> Installing Go tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/swaggo/swag/cmd/swag@latest
	$(GO) install github.com/pressly/goose/v3/cmd/goose@latest
	$(GO) install github.com/air-verse/air@latest
	@echo "==> Installing Node tools..."
	$(PNPM) install
	@echo "==> Installing Python tools (for docs)"
	pip install mkdocs-material mkdocs-mermaid2-plugin
	@echo "==> Generating go.sum..."
	cd shared && $(GO) mod download
	@for svc in $(SERVICES_IMPL); do \
		echo "  -> $$svc"; \
		cd services/$$svc && $(GO) mod download && cd ../..; \
	done
	@echo "==> Setup complete."

.PHONY: deps
deps: ## Download all Go dependencies
	@for svc in $(SERVICES_IMPL); do \
		echo "  -> $$svc"; \
		cd services/$$svc && $(GO) mod download && cd ../..; \
	done
	cd shared && $(GO) mod download

# ============================================================================
# Build
# ============================================================================
.PHONY: build
build: ## Build all services
	@if [ -z "$(SERVICES_IMPL)" ]; then echo "No services implemented yet."; exit 0; fi
	@echo "==> Building services..."
	@for svc in $(SERVICES_IMPL); do \
		echo "  -> $$svc"; \
		cd services/$$svc && $(GO) build $(GOFLAGS) -o ../../bin/$$svc ./cmd/$$svc && cd ../..; \
	done

.PHONY: build-svc
build-svc: ## Build a single service (usage: make build-svc SVC=project-svc)
	@if [ -z "$(SVC)" ]; then echo "Usage: make build-svc SVC=<service-name>"; exit 1; fi
	cd services/$(SVC) && $(GO) build $(GOFLAGS) -o ../../bin/$(SVC) ./cmd/$(SVC)

# ============================================================================
# Test
# ============================================================================
.PHONY: test
test: ## Run all tests
	@echo "==> Running tests..."
	cd shared && $(GO) test -race -cover ./... && cd ..
	@for svc in $(SERVICES_IMPL); do \
		echo "  -> $$svc"; \
		cd services/$$svc && $(GO) test -race -cover ./... && cd ../..; \
	done

.PHONY: test-svc
test-svc: ## Test a single service (usage: make test-svc SVC=project-svc)
	@if [ -z "$(SVC)" ]; then echo "Usage: make test-svc SVC=<service-name>"; exit 1; fi
	cd services/$(SVC) && $(GO) test -race -cover ./...

.PHONY: coverage
coverage: ## Generate coverage report
	@echo "==> Generating coverage..."
	@mkdir -p coverage
	cd shared && $(GO) test -coverprofile=../coverage/shared.out ./... && cd ..
	@for svc in $(SERVICES_IMPL); do \
		cd services/$$svc && $(GO) test -coverprofile=../../coverage/$$svc.out ./... && cd ../..; \
	done

# ============================================================================
# Benchmarks
# ============================================================================
# Run all Go benchmarks and write to bench/current.txt. Combine with
# `make bench-guard` to detect regressions vs bench/baseline/baseline.txt.
BENCH_PKGS := ./services/api-gateway/... \
              ./services/workflow-svc/internal/api/... \
              ./shared/...
BENCH_FLAGS := -run=^$$ -bench=. -benchmem -count=3 -benchtime=1s

.PHONY: bench
bench: ## Run all Go benchmarks and write to bench/current.txt
	@mkdir -p bench
	$(GO) test $(BENCH_FLAGS) $(BENCH_PKGS) | tee bench/current.txt

.PHONY: bench-baseline
bench-baseline: ## Refresh the stored baseline from a fresh bench run
	@mkdir -p bench/baseline
	$(GO) test $(BENCH_FLAGS) $(BENCH_PKGS) | tee bench/baseline/baseline.txt
	@echo "==> baseline updated: bench/baseline/baseline.txt"

.PHONY: bench-guard
bench-guard: bench ## Run benchmarks and fail if any regress >20% vs baseline
	@echo "==> comparing current vs baseline (threshold 20%)"
	@cd scripts/bench-guard && \
	  $(GO) run main.go \
	    --baseline ../../bench/baseline/baseline.txt \
	    --current  ../../bench/current.txt \
	    --threshold 20 \
	    --output   ../../bench/report.md
	@echo "==> report: bench/report.md"

.PHONY: bench-guard-threshold
bench-guard-threshold: bench ## Same as bench-guard, configurable threshold (THRESHOLD=15)
	@cd scripts/bench-guard && \
	  $(GO) run main.go \
	    --baseline ../../bench/baseline/baseline.txt \
	    --current  ../../bench/current.txt \
	    --threshold $(or $(THRESHOLD),20) \
	    --output   ../../bench/report.md

# ============================================================================
# Lint
# ============================================================================
.PHONY: lint
lint: ## Run linters (golangci-lint per module + markdownlint + yaml)
	@for svc in $(SERVICES_IMPL); do \
		echo "  -> golangci-lint $$svc"; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
	done
	cd shared && golangci-lint run ./... && cd ..
	markdownlint docs/ README.md AGENTS.md CONTRIBUTING.md
	PROJECT_ROOT=$$(cd .. && pwd) python3 scripts/validate-ci.py

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...
	gofmt -s -w .
	cd web && $(PNPM) run format 2>/dev/null || true

# ============================================================================
# Database
# ============================================================================
.PHONY: db-up
db-up: ## Start PostgreSQL + Redis + MinIO via docker compose
	$(COMPOSE) -f docker-compose.yml up -d postgres redis minio
	@echo "Waiting for postgres..."
	@sleep 5
	@$(COMPOSE) -f docker-compose.yml exec postgres pg_isready -U postgres

.PHONY: db-down
db-down: ## Stop docker compose services
	$(COMPOSE) -f docker-compose.yml down

.PHONY: db-logs
db-logs: ## Tail logs from docker compose
	$(COMPOSE) -f docker-compose.yml logs -f

.PHONY: migrate
migrate: ## Run database migrations
	@echo "==> Running migrations..."
	cd migrations && goose -dir . postgres "$$DB_DSN" up
	@echo "==> Done."

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	cd migrations && goose -dir . postgres "$$DB_DSN" down

.PHONY: migrate-status
migrate-status: ## Show migration status
	cd migrations && goose -dir . postgres "$$DB_DSN" status

.PHONY: psql
psql: ## Open psql shell
	$(COMPOSE) -f docker-compose.yml exec postgres psql -U postgres bidwriter

# ============================================================================
# Run (development)
# ============================================================================
.PHONY: run-svc
run-svc: ## Run a service with hot reload (usage: make run-svc SVC=project-svc)
	@if [ -z "$(SVC)" ]; then echo "Usage: make run-svc SVC=<service-name>"; exit 1; fi
	cd services/$(SVC) && air

.PHONY: dev
dev: ## Run full local stack (postgres + redis + minio + all services)
	$(COMPOSE) -f docker-compose.yml up

.PHONY: dev-web
dev-web: ## Run web frontend
	cd web && $(PNPM) dev

# ============================================================================
# Docs
# ============================================================================
.PHONY: docs
docs: ## Build mkdocs site (strict mode)
	$(MKDOCS) build --strict

.PHONY: docs-serve
docs-serve: ## Serve docs locally
	$(MKDOCS) serve

.PHONY: docs-validate
docs-validate: ## Validate docs structure (CI equivalent)
	python3 scripts/validate-ci.py
	$(MKDOCS) build --strict 2>&1 | grep -E "WARNING|ERROR" || echo "OK"

# ============================================================================
# Docker
# ============================================================================
.PHONY: docker-build
docker-build: ## Build all service images
	@for svc in $(SERVICES); do \
		echo "  -> $$svc"; \
		$(DOCKER) build -t bidwriter/$$svc:latest -f services/$$svc/Dockerfile .; \
	done

.PHONY: docker-push
docker-push: ## Push images to registry
	@for svc in $(SERVICES); do \
		$(DOCKER) push bidwriter/$$svc:latest; \
	done

# ============================================================================
# Cleanup
# ============================================================================
.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/
	rm -rf coverage/
	rm -rf site/
	$(GO) clean -cache

.PHONY: reset
reset: clean db-down ## Full reset (clean + remove docker volumes)
	$(COMPOSE) -f docker-compose.yml down -v

# ============================================================================
# Git helpers
# ============================================================================
.ONESHELL:
.SHELLFLAGS := -ec

.PHONY: hook-install
hook-install: ## Install git pre-commit hook
	@mkdir -p .git/hooks
	@printf '#!/bin/bash\nset -e\necho "==> Running pre-commit checks..."\nmake lint\n' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Done."

.PHONY: status
status: ## Show project status
	@echo "==> Git status:"
	@git status --short 2>/dev/null || echo "Not a git repo"
	@echo ""
	@echo "==> Service count: $(words $(SERVICES))"
	@echo "==> Services: $(SERVICES)"