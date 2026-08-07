# Contributing to aflare

Welcome to aflare! We appreciate your interest in contributing. This guide will help you get started.

## 🌟 Good First Issues

New to the project? Start here — these issues are specifically curated for first-time contributors:

| Category | Description | How to Find |
|----------|-------------|-------------|
| 🐛 **Bug Fixes** | Small, well-defined bugs with clear reproduction steps | [`good first issue` label](https://github.com/alib8b8/aflare/labels/good%20first%20issue) |
| 📝 **Documentation** | Typos, clarifications, missing examples | [`documentation` label](https://github.com/alib8b8/aflare/labels/documentation) |
| ✅ **Tests** | Add test coverage for low-coverage packages | See "Test Coverage" section below |
| 🔧 **New Nodes** | Build a new utility or external node | [Custom Nodes Guide](docs/custom-nodes.md) |
| 🌐 **i18n** | Add or improve translations | [`i18n` label](https://github.com/alib8b8/aflare/labels/i18n) |

### Test Coverage Targets

Help us reach our 85% coverage goal! These packages need the most love:

| Package | Current Coverage | Difficulty |
|---------|-----------------|------------|
| `internal/distributed` | ~12% | Medium — HTTP handlers, mock servers |
| `internal/nodes` | ~56% | Easy-Medium — unit tests for utility nodes |
| `internal/version` | ~56% | Easy — version string parsing |
| `internal/webui` | 0% | Medium — HTTP handler tests |

**Pro tip**: Start with `internal/version` — it's small and has no external dependencies!

## 📋 Getting Started

1. **Fork the repository**
2. **Clone your fork**: `git clone https://github.com/your-username/aflare.git`
3. **Install dependencies**: `go mod download`
4. **Build locally**: `go build -o aflare ./cmd/aflare`
5. **Create a branch**: `git checkout -b feature/your-feature-name`
6. **Make changes**
7. **Run tests**: `go test ./... -short`
8. **Submit a PR**

## 🎯 How to Contribute

### Bug Fixes
- Create a branch named `fix/issue-XX` where XX is the issue number
- Include a test that reproduces the bug (regression test)
- Fix the bug and verify the test passes
- Update the [changelog](CHANGELOG.md) with `fix: description`

### New Features
- Create a branch named `feature/feature-name`
- Document your feature in the appropriate `docs/` file
- Add comprehensive tests (unit + integration where applicable)
- Update the [changelog](CHANGELOG.md) with `feat: description`

### Documentation
- Create a branch named `docs/topic-name`
- Improve clarity and accuracy
- Ensure consistency with existing documentation
- No code changes required — perfect for first-time contributors!

### Performance Improvements
- Create a branch named `perf/description`
- Include benchmark results before and after your change
- See [Performance Benchmarks](#performance-benchmarks) section below

## 🔍 Code Review Process

We take code quality seriously. Here's what to expect:

### Review Steps

1. **Automated checks** (CI) — runs on every PR:
   - Build passes
   - All tests pass
   - `gofmt` formatting check
   - `go vet` static analysis
   - `gosec` security scan
   - CodeQL analysis
   - Coverage threshold check (60% minimum enforced in CI; 85% aspirational goal)

2. **Human review** — after CI passes:
   - A CODEOWNER will review within 48 hours (weekdays)
   - They'll check for correctness, style, security, and test coverage
   - You may receive feedback and revision requests

3. **Approval & merge**:
   - At least 1 approval required
   - All review comments resolved
   - Branch is up-to-date with `main`
   - Squash merge is used for clean history

### What Reviewers Look For

| Category | What we check |
|----------|--------------|
| **Correctness** | Does it work as intended? Edge cases handled? |
| **Tests** | Is there sufficient test coverage? Regression tests for bugs? |
| **Security** | No injection vulnerabilities? Input validation? |
| **Style** | Follows Go conventions? `gofmt`-ed? Readable? |
| **Documentation** | Docs updated? Code comments on public APIs? |
| **Performance** | No unnecessary allocations? Efficient algorithms? |

### Tips for a Smooth Review

- Keep PRs small and focused (ideally < 400 lines)
- Write clear PR descriptions (use the template)
- Add comments for non-obvious code
- Respond to review comments with explanations or changes
- Be patient — reviewers are volunteers too!

## 📝 Code Guidelines

### Go Code Style

1. **Formatting**: Run `gofmt -w .` before submitting
2. **Linting**: Run `go vet ./...` before submitting
3. **Testing**: All code should have unit tests
4. **Error Handling**: Always handle errors properly
5. **Comments**: Document public functions and types
6. **Naming**: Use descriptive names, follow Go conventions

### Security Guidelines

See [SECURITY.md](SECURITY.md) for detailed security requirements:

- No hardcoded secrets or credentials
- No shell injection vulnerabilities
- No path traversal vulnerabilities
- All inputs must be validated
- Error messages must not leak sensitive information

### Prohibited Changes

The following changes will be **rejected**:

- Code that executes arbitrary commands
- Code that reads/writes sensitive system files
- Code that makes unauthorized network requests
- Code that logs sensitive information
- Unnecessary dependency additions

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific tests
go test ./internal/nodes/...

# Run with verbose output
go test -v ./...
```

### Writing Tests

- All new code must have tests
- Tests should be deterministic
- Avoid network calls in unit tests
- Use table-driven tests when appropriate

## 🚀 Performance Benchmarks

We track performance to ensure aflare stays fast. Here's how to run and write benchmarks.

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run benchmarks for a specific package
go test -bench=. -benchmem ./internal/workflow/

# Run with more iterations for stable results
go test -bench=. -benchmem -benchtime=5s ./internal/expression/

# Compare against baseline (install benchstat first)
go install golang.org/x/perf/cmd/benchstat@latest
go test -bench=. -benchmem -count=5 ./internal/workflow/ > new.txt
benchstat old.txt new.txt
```

### Current Benchmark Suite

| Package | Benchmark | Description |
|---------|-----------|-------------|
| `internal/workflow` | `BenchmarkExpressionEngine` | Variable substitution speed |
| `internal/cache` | `BenchmarkCacheGet` | Cache read performance |
| `internal/scheduler` | `BenchmarkScheduler` | Task scheduling throughput |

### Writing Benchmarks

Follow these guidelines when adding benchmarks:

1. **Name clearly**: `BenchmarkFunctionName_Description`
2. **Use `b.N`**: Loop `b.N` times for accurate measurement
3. **Reset timer**: Use `b.ResetTimer()` if setup is needed
4. **Report allocations**: Always run with `-benchmem`
5. **Keep them fast**: Benchmarks should complete in < 10s

Example:

```go
func BenchmarkGenerateWorkflow_Simple(b *testing.B) {
    desc := "fetch example.com and save to file"
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := GenerateWorkflow(desc)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Performance Goals

| Component | Target | Notes |
|-----------|--------|-------|
| Workflow parsing | < 1ms | For a 10-step workflow |
| Expression evaluation | < 100μs | Per variable substitution |
| Step dispatch | < 1ms | From scheduled to started |
| Memory per workflow | < 1MB | For a typical 5-step workflow |

Submit a PR with `perf:` prefix if you improve any benchmark by 10% or more!

## 🔄 Pull Request Process

1. **Create a PR** from your branch to `main`
2. **CI Checks**: Wait for CI to pass (lint, test, security, build)
3. **Review**: A CODEOWNER will review your PR
4. **Approval**: Need at least 1 approval
5. **Merge**: PR will be merged after approval

### PR Checklist

Before submitting your PR:

- [ ] All tests pass
- [ ] Code follows Go style guidelines
- [ ] Security checks pass
- [ ] Documentation is updated
- [ ] No hardcoded secrets
- [ ] No debug print statements

### PR Template

Your PR description should include:

```
## What

Describe what you changed.

## Why

Explain why this change is needed.

## Changes

List the changes made:
1. Changed X
2. Added Y
3. Fixed Z

## Testing

How to test:
- Run `go test ./...`
- Test case 1
- Test case 2

## Checklist

- [ ] Tests pass
- [ ] Documentation updated
- [ ] Security checks pass
```

## 🚫 Code of Conduct

All contributors must follow our code of conduct:

- Be respectful and inclusive
- Use welcoming language
- Focus on constructive feedback
- Avoid personal attacks
- Be collaborative

## 📞 Getting Help

If you need help:

1. Check existing issues and PRs
2. Open a new issue
3. Ask in discussions

## 📜 License

By contributing, you agree that your contributions will be licensed under the GNU Affero General Public License v3.0.
