# llm-box Roadmap

This document outlines the planned features and timeline for llm-box.

---

## v0.1 - Initial Release ✅

**Released**: June 2026

**Features**:
- [x] Basic workflow creation from plain English
- [x] Workflow execution engine
- [x] Beautiful terminal UI
- [x] 6 built-in nodes (fetch_url, file_write, ollama, execute, notify, transform)
- [x] Single static binary
- [x] Cross-platform support (Linux/macOS/Windows)
- [x] 10 example workflows
- [x] Documentation

---

## v0.2 - Plugin System (Q3 2026)

**Planned features**:
- [x] More built-in nodes
  - [x] `file_read`
  - [ ] `http_request` (more flexible than fetch_url)
  - [ ] `git` (clone, pull, commit, etc.)
  - [x] `json_parse`
  - [ ] `csv_parse`
  - [x] `template_render`
- [ ] Plugin system for easy custom node installation
- [ ] Workflow template library (15+ templates)
- [ ] Workflow sharing via URL
- [ ] Workflow validation and linting

---

## v0.3 - Team Features (Q4 2026)

**Planned features**:
- [ ] Team workflow repository (share workflows with your team)
- [ ] Workflow versioning and history
- [ ] Cloud sync (optional - still 100% local by default)
- [ ] Workflow variables and secrets management
- [ ] Workflow testing framework

---

## v0.4 - Enterprise Features (Q1 2027)

**Planned features**:
- [ ] Access control
- [ ] Audit logging
- [ ] Scheduled workflows (no need for cron/systemd)
- [ ] Webhook triggers
- [ ] Workflow monitoring dashboard
- [ ] Role-based access control (RBAC)

---

## v1.0 - Stable Production Release (Q3 2027)

**Planned features**:
- [ ] Production-ready, stable API
- [ ] Comprehensive documentation
- [ ] 50+ built-in nodes
- [ ] Plugin registry
- [ ] LTS (long-term support) release
- [ ] Official Docker image
- [ ] Package manager packages (Homebrew, apt, yum, etc.)

---

## Future Ideas (Post-1.0)

These are ideas for future releases, not yet committed to a timeline:

- [ ] Web UI (optional, but still terminal-first)
- [ ] Workflow visualizer
- [ ] AI-assisted workflow creation
- [ ] Integration with GitHub Actions
- [ ] Integration with GitLab CI
- [ ] Integration with popular tools (Slack, GitHub Issues, etc.)
- [ ] Workflow performance profiling and optimization tools

---

## How to Influence the Roadmap

- Open a [Discussion](https://github.com/alib8b8/llm-box/discussions) with your idea
- File an [Issue](https://github.com/alib8b8/llm-box/issues) with a feature request
- Comment on existing issues to show your support
- Contribute code! PRs are welcome

---

## Release Cadence

- Minor releases (0.1 → 0.2, etc.) every 3-4 months
- Patch releases (0.1.0 → 0.1.1) as needed for bug fixes
- Security patches released within 48 hours of discovery

---

## Versioning

llm-box uses [Semantic Versioning](https://semver.org/):
- MAJOR version for incompatible API changes
- MINOR version for new functionality in a backward-compatible manner
- PATCH version for backward-compatible bug fixes

Note: Until v1.0, anything may change at any time. However, we will do our best to maintain compatibility as much as possible.
