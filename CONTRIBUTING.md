# Contributing to llm-box

Welcome to llm-box! We appreciate your interest in contributing. This guide will help you get started.

## 📋 Getting Started

1. **Fork the repository**
2. **Clone your fork**: `git clone https://github.com/your-username/llm-box.git`
3. **Create a branch**: `git checkout -b feature/your-feature-name`
4. **Make changes**
5. **Test your changes**: Run `go test ./...`
6. **Submit a PR**

## 🎯 How to Contribute

### Bug Fixes
- Create a branch named `fix/issue-XX` where XX is the issue number
- Include a test that reproduces the bug
- Fix the bug

### New Features
- Create a branch named `feature/feature-name`
- Document your feature in the README
- Add tests
- Update any relevant documentation

### Documentation
- Create a branch named `docs/topic-name`
- Improve clarity and accuracy
- Ensure consistency with existing documentation

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

By contributing, you agree that your contributions will be licensed under the MIT License.
