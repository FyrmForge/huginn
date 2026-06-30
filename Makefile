.PHONY: build test lint templint db-sh docker-up docker-down docker-delete docker-reload docker-build clean install check-templ generate check-node-modules css css-build e2e e2e-local e2e-run e2e-run-local

# Force bash so the ENV_LOAD eval below works cross-shell (sh on Debian/Ubuntu
# is dash, which doesn't grok `eval "$(...)"` quoting consistently).
SHELL := bash
.SHELLFLAGS := -ec

HAMR_VERSION  := $(shell grep 'github.com/FyrmForge/hamr ' go.mod | awk '{print $$2}')
TEMPL_VERSION := $(shell grep 'github.com/a-h/templ ' go.mod | awk '{print $$2}')

# ENV_LOAD: prefix targets that need hamr dev's port-walked .env values
# (DATABASE_URL, S3_ENDPOINT, etc.). `hamr env --export` reads
# .hamr/walks.json (written by `hamr dev` after walking) + .env, and emits
# `export KEY=VALUE` lines for the values that were rewritten. When hamr
# dev isn't running or no ports walked, output is empty and eval is a
# no-op — the target falls through to whatever .env it already had.
# `|| true` survives the case where `hamr` isn't on PATH (e.g. during
# `make install`); plain .env still loads normally inside the binary.
ENV_LOAD := eval "$$(hamr env --export 2>/dev/null || true)";

## fmt: Format all Go source files
fmt:
	go fmt ./...

## install: Install development dependencies
install:
	go install github.com/FyrmForge/hamr/cmd/hamr@$(HAMR_VERSION)
	go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)
	npm install

## check-templ: Verify templ is installed
check-templ:
	@command -v templ >/dev/null 2>&1 || { echo "templ not found. Run: make install" >&2; exit 1; }

## check-node-modules: Auto-install npm deps if missing
check-node-modules:
	@[ -d node_modules ] || npm install

## css: Watch and rebuild Tailwind CSS on changes
css: check-node-modules
	npm run css

## css-build: Build Tailwind CSS for production (minified)
css-build: check-node-modules
	npm run css:build

VERSION := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")

## build: Build the site binary
build: check-templ check-node-modules
	templ generate
	npm run css:build
	hamr gen static
	go build -ldflags "-X main.version=$(VERSION)" -o bin/site ./cmd/site
	$(MAKE) generate

## generate: Generate static pages
generate:
	$(ENV_LOAD) ./bin/site --generate

## test: Run all tests
test: check-templ
	templ generate
	go test ./...

## db-sh: Open an interactive shell to the local dev database
db-sh:
	$(ENV_LOAD) ./scripts/db-shell.sh

## lint: Run linters
lint:
	golangci-lint run

## templint: Lint .templ files for common issues
templint:
	hamr lint templ

## docker-up: Start Docker services
docker-up:
	docker compose -f docker/docker-compose.yaml up -d

## docker-down: Stop Docker services
docker-down:
	docker compose -f docker/docker-compose.yaml down

## docker-delete: Delete Docker containers and volumes
docker-delete:
	docker compose -f docker/docker-compose.yaml down -v

## docker-reload: Delete and recreate Docker services
docker-reload: docker-delete docker-up

## e2e: Run E2E tests (containerized)
e2e:
	go test -v -tags=e2e ./e2e -timeout 10m

## e2e-local: Run E2E tests against local server
e2e-local:
	$(ENV_LOAD) go test -v -tags=e2e ./e2e -local -timeout 10m

## e2e-run: Run specific E2E test: make e2e-run T=TestName
e2e-run:
	go test -v -tags=e2e ./e2e -run "$(T)" -timeout 5m

## e2e-run-local: Run specific E2E test locally: make e2e-run-local T=TestName
e2e-run-local:
	$(ENV_LOAD) go test -v -tags=e2e ./e2e -local -run "$(T)" -timeout 2m

## docker-build: Test the production image build (mirrors CI, no push)
docker-build:
	docker build --build-arg VERSION=$(VERSION) -f cmd/site/Dockerfile -t huginn:$(VERSION) .

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html
