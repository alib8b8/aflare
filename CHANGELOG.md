# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-07-04

### Added
- Multi-language support (9 languages: zh, en, ru, fr, ja, ko, es, ar, hi)
- Condition execution support for workflow steps
- Variable substitution (vars field) in workflows
- Atomic write operations for file_write node
- Workflow chaining via call node
- Dockerfile for containerized deployment
- Makefile for build automation
- GoReleaser configuration for cross-platform releases
- Homebrew tap support
- SHA256 checksum verification in install.sh
- Thread-safe Registry with mutex locks
- .gosec.json security scan configuration

### Changed
- Tightened directory permissions from 0755 to 0750
- Tightened file permissions from 0644 to 0600
- Ollama node now prioritizes prompt parameter over input
- notify node returns error for invalid channel instead of silent fallback

### Fixed
- SSRF protection (DNS resolution, IPv4-mapped IPv6 bypass, redirect validation)
- Path traversal protection (symlink resolution, dot-file rejection)
- Command injection protection (shell metacharacter blocking)
- template_render SSTI vulnerability - removed dangerous template functions
- Integer overflow in registry lowercase function
- Context leak in workflow executor (defer to immediate stepCancel)
- Keyword matching improved with word boundary checks and Chinese support
- gofmt formatting issues across 12 files

### Security
- Complete SSRF protection layer
- Path traversal protection for all file operations
- Command injection prevention for execute node
- Resource limits (file size, response body, retry/parallel/step counts)
- Recursive call depth tracking for workflow chaining
- Sensitive data filtering in audit logs
- External node API key protection

## [0.2.10] - 2026-06-16

### Added
- External node support with registry
- Node install/uninstall commands
- LLM node deduplication (llm_base.go)

### Fixed
- Various bug fixes and stability improvements

## [0.1.0] - 2026-06-02

### Added
- Initial release
- Core workflow engine with YAML-based step definition
- Built-in nodes: llm, execute, file_read, file_write, http_request, fetch_url
- Interactive TUI with bubbletea
- Workflow generation from natural language
- Ollama integration
- History tracking

[Unreleased]: https://github.com/alib8b8/aflare/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/alib8b8/aflare/compare/v0.2.10...v0.3.0
[0.2.10]: https://github.com/alib8b8/aflare/compare/v0.1.0...v0.2.10
[0.1.0]: https://github.com/alib8b8/aflare/releases/tag/v0.1.0
