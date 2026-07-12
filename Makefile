.PHONY: build test lint fmt vet security clean install release version

BINARY=llm-box
CMD=./cmd/llm-box
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X github.com/alib8b8/llm-box/internal/version.Version=$(VERSION) -X github.com/alib8b8/llm-box/internal/version.BuildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

version:
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -cover -count=1

lint: fmt vet

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

# Run all checks (CI equivalent)
ci: fmt vet test security build
	@echo "All CI checks passed!"
