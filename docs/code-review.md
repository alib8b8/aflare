# Code Review Checklist

Every PR must pass all checks below before merge.

## Automated Checks (CI)

| # | Check | Tool | Blocking |
|---|-------|------|----------|
| 1 | Formatting | `gofmt -l .` | Yes |
| 2 | Static analysis | `go vet ./...` | Yes |
| 3 | Linting | `golangci-lint` (errcheck, staticcheck, funlen, etc.) | Yes |
| 4 | Unit tests | `go test ./... -short` | Yes |
| 5 | Coverage threshold | `go test -cover` ≥ 60% | Yes |
| 6 | Race detector | `go test -race ./internal/agent/... ./internal/memory/...` | Yes |
| 7 | Vulnerability scan | `govulncheck ./...` | Yes |
| 8 | Benchmark regression | `benchstat` baseline vs current | Warn |

## Manual Review Checklist

### Security

| # | Check | How |
|---|-------|-----|
| 1 | Path traversal risk | `rg 'filepath\.Join.*(params\|input\|user)'` — user-controlled paths must be sanitized |
| 2 | Command injection risk | `rg 'exec\.Command.*(params\|input\|user)'` — never pass user input directly to shell |
| 3 | SSRF risk | `rg 'http\.(Get\|Post).*(params\|input\|user)'` — validate user-supplied URLs |
| 4 | Race conditions | `go test -race` — all map/slice access must be guarded |
| 5 | Error swallowing | `_ =` and bare `if err != nil { return }` are banned; errors must be wrapped or logged |

### Architecture

| # | Check | How |
|---|-------|-----|
| 6 | New capability implements all 4 methods | `rg 'func.*Capability\) (Init\|PreProcess\|PostProcess\|Shutdown)'` |
| 7 | New node is registered | `rg 'Register.*Node\{\}'` — must be in `registerChatNodes` or `RegisterBuiltins` |
| 8 | Plugin interface compatibility | New plugins must implement `Plugin` + `NodePlugin` (if node type) |

### Code Quality

| # | Check | How |
|---|-------|-----|
| 9 | Function length ≤ 80 lines | `golangci-lint` with `funlen` |
| 10 | No dead code | `golangci-lint` with `unused` |
| 11 | Commit message format | `feat:` / `fix:` / `chore:` / `docs:` / `refactor:` prefix required |
| 12 | Every PR has test changes | `git diff --stat` must include `*_test.go` |

### Testing

| # | Check | How |
|---|-------|-----|
| 13 | New capability has chain test | PreProcessAll → PostProcessAll with ≥3 fake capabilities |
| 14 | New node has E2E test | mock LLM + real registry + full ReAct cycle |
| 15 | Streaming paths are tested | onChunk / onToolCall / onToolResult callbacks verified |
| 16 | Concurrent safety tested | `go test -race` with goroutine count ≥ 10 |

## Commit Message Convention

```
<type>: <description>

Types:
  feat      — new feature
  fix       — bug fix
  chore     — maintenance, tooling, CI
  docs      — documentation only
  refactor  — code change that neither fixes nor adds a feature
  test      — adding tests only
  perf      — performance improvement
  security  — security fix

Examples:
  feat: add session persistence with /resume command
  fix: race condition in learning.json concurrent writes
  chore: add funlen linter with 80-line limit
  docs: add code review checklist
```

## Test Coverage Targets

| Package | Current | Target |
|---------|---------|--------|
| `internal/agent` | ~50% | 60% |
| `internal/nodes` | ~55% | 60% |
| `internal/workflow` | ~45% | 60% |
| `internal/memory` | ~40% | 60% |
| Overall | ~45% | 60% |