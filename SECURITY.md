# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting Security Issues

If you discover a security vulnerability in llm-box, please report it immediately. We take security seriously and will respond promptly.

### How to Report

1. **Do NOT open a public issue** - This could expose the vulnerability to attackers.
2. Contact the maintainer directly via GitHub Issues with a security label.
3. Include the following details:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if you have one)

### Response Time

- We aim to acknowledge security reports within **24 hours**
- Critical vulnerabilities will be addressed within **72 hours**
- A fix will be released as soon as possible

## Security Guidelines for Contributors

### Prohibited Changes

The following changes will be **rejected** without review:

1. **Code Execution**: Any changes that execute arbitrary code without proper validation
2. **Shell Commands**: Changes that spawn shell processes with user-controlled input
3. **Network Access**: Unauthorized network requests or data exfiltration
4. **File System**: Write operations outside designated directories
5. **Environment Variables**: Reading/writing sensitive environment variables
6. **Dependency Changes**: Adding dependencies without security review
7. **Binary Execution**: Embedding or downloading executables

### Required Security Practices

1. **Input Validation**: All user input must be validated and sanitized
2. **Error Handling**: Errors must not expose sensitive information
3. **Logging**: Sensitive data (API keys, tokens) must never be logged
4. **Dependency Checks**: Run `go mod verify` before submitting PRs
5. **Static Analysis**: Run `go vet` and `gosec` on your changes

## Security Checklist for PR Reviewers

- [ ] No hardcoded secrets or credentials
- [ ] No shell injection vulnerabilities
- [ ] No path traversal vulnerabilities
- [ ] No network requests to unknown hosts
- [ ] All inputs are properly validated
- [ ] Error messages don't leak sensitive information
- [ ] Dependencies are pinned and verified
- [ ] No executable downloads or binary execution
- [ ] File operations are restricted to allowed paths

## Automated Security Checks

The following tools run automatically on all PRs:

- **go vet**: Static analysis for common Go issues
- **gosec**: Security-focused static analysis
- **gofmt**: Code formatting consistency
- **Unit Tests**: Ensure existing functionality isn't broken

## Vulnerability Disclosure Policy

We follow a coordinated disclosure policy:

1. Vulnerability reported
2. Maintainer acknowledges and assesses severity
3. Fix developed and tested
4. Fix released
5. Public disclosure (with credit to the reporter)

## Credits

Security vulnerabilities reported by the community will be acknowledged in the release notes.
