.PHONY: build test test-short lint race fmt vet security cover clean install release version ci

BINARY=llm-box
CMD=./cmd/llm-box
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X github.com/alib8b8/llm-box/internal/version.Version=$(VERSION) -X github.com/alib8b8/llm-box/internal/version.BuildDate=$(BUILD_DATE)"

# Warning-level linters (errorlint, nilerr, unparam, prealloc) are enabled in
# .golangci.yml with severity=warning. They are surfaced by `make lint` but kept
# out of the blocking `make ci` gate so existing findings don't break the build;
# new code should still keep them clean. Core linters (errcheck, govet,
# ineffassign, staticcheck, unused, misspell) always block.
GOLANGCI_WARN_LINTERS := errorlint,nilerr,unparam,prealloc

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

version:
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"

# test mirrors CI: race detector on, fresh run (no caching).
test:
	go test ./... -race -count=1

# test-short runs the short test suite with the race detector, matching the
# `go test ./... -race -short` step in .github/workflows/ci.yml.
test-short:
	go test ./... -race -short -count=1

# race is an alias focused on the race detector across the full suite.
race:
	go test -race ./... -count=1

test-cover:
	go test ./... -cover -count=1

# cover generates a coverage profile (CI parity) over internal/ packages,
# excluding webui to match .github/workflows/ci.yml.
cover:
	go test -coverprofile=coverage.out -covermode=atomic $(shell go list ./internal/... | grep -v '/webui$$') -short

# lint runs the full golangci-lint config: core (blocking) + warning-level
# linters. Use this to see every issue, including non-blocking warnings.
lint:
	golangci-lint run ./...

fmt:
	gofmt -l . | grep -q . && gofmt -w . || true

vet:
	go vet ./...

security:
	gosec -severity medium -confidence medium -conf .gosec.json ./...

clean:
	rm -f $(BINARY)

install: build
	cp $(BINARY) /usr/local/bin/

docker:
	docker build -t llm-box .

# Cross-compile for release
release: clean
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			echo "Building $(BINARY)-$$os-$$arch$$ext..."; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
				go build $(LDFLAGS) -o $(BINARY)-$$os-$$arch$$ext $(CMD); \
			sha256sum $(BINARY)-$$os-$$arch$$ext > $(BINARY)-$$os-$$arch$$ext.sha256; \
		done; \
	done

# ci approximates the GitHub Actions CI pipeline so developers can run the same
# checks locally before pushing:
#   - gofmt check (must be clean, no auto-rewrite)
#   - go vet
#   - golangci-lint (core linters block; warning-level linters excluded here so
#     existing findings don't fail the gate — run `make lint` to see them)
#   - race-detector tests (short suite, matching CI)
#   - coverage threshold (>= 60%, matching CI)
#   - build
#   - gosec security scan (best-effort; skipped if gosec is not installed)
ci: fmt-check vet lint-blocking test-short cover-check build security-check
	@echo "All CI checks passed!"

# fmt-check fails if any file is not gofmt-formatted (does not rewrite).
fmt-check:
	@out=$$(gofmt -l . | grep -v '^vendor/' | grep -v '^grok-mcp-server/'); \
	if [ -n "$$out" ]; then \
		echo "::error::gofmt would reformat:"; echo "$$out"; exit 1; \
	fi

# lint-blocking runs only the core (error-level) linters so the gate fails on
# real issues but not on the warning-level backlog.
lint-blocking:
	golangci-lint run --disable $(GOLANGCI_WARN_LINTERS) ./...

# cover-check enforces the same 60% coverage threshold as CI.
cover-check: cover
	@total=$$(go tool cover -func=coverage.out | grep total: | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${total}%"; \
	threshold=60; \
	echo "Threshold: $${threshold}%"; \
	pass=$$(awk -v total="$$total" -v threshold="$$threshold" 'BEGIN { print (total >= threshold) ? 1 : 0 }'); \
	if [ "$$pass" -eq 0 ]; then \
		echo "::error::Coverage $${total}% is below required threshold $${threshold}%"; exit 1; \
	else \
		echo "::notice::Coverage $${total}% meets threshold $${threshold}%"; \
	fi

# security-check runs gosec if available, otherwise is a no-op.
security-check:
	@command -v gosec >/dev/null 2>&1 && gosec -severity medium -confidence medium -conf .gosec.json ./... || \
		echo "::notice::gosec not installed, skipping security scan"

