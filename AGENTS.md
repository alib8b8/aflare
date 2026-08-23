# AGENTS.md

## Toolchain
- Go 1.26 (go.mod pins `go 1.26.0` + `toolchain go1.26.7`; CI uses go-version 1.26.7). Never
  downgrade below the pinned patch — stdlib vulnerabilities are surfaced by govulncheck.
- golangci-lint v2.13.1 — must match CI. Binaries built with older Go (e.g. v2.12.2 built with go1.25) cannot lint this repo.

## CI gate — must pass locally before any commit
Run all of these; all must be green:
```bash
gofmt -l .            # must output nothing
go vet ./...
golangci-lint run --timeout 5m
go test ./... -race -short
govulncheck ./...     # blocking in CI — reachable dependency vulns fail the build
```
Coverage must stay ≥ 60% overall and per-package (agent / workflow / memory at
60%, nodes at 50% — thresholds are enforced in .github/workflows/ci.yml).

## Commit policy (GitHub + GitCode)
- All changes go through a pull request — never push directly to main.
- A PR may only be merged after CI is green (ci.yml + pr-review.yml, lint is
  blocking) AND the code-review checklist in docs/code-review.md has been
  applied (security / architecture / code-quality / testing sections).
- GitCode receives code only via the CI-gated mirror (sync-gitcode.yml):
  it mirrors main exclusively after the CI workflow succeeds on that commit.
- Commit message format: `<type>: <description>` with types
  feat / fix / chore / docs / refactor / test / perf / security
  (see docs/code-review.md "Commit Message Convention").

## Loop conventions
- Report-only week one (L1) before enabling auto-fix (L2)
- Loop state files (LOOP.md / STATE.md / loop-budget.md / loop-constraints.md /
  loop-run-log.md) are gitignored and maintained locally by the loop operator;
  do not commit them
