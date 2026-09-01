<div align="center">
  <h1>aflare</h1>
  <p>
    <strong>English</strong> ·
    <a href="README.zh.md">简体中文</a>
  </p>
  <p><strong>AI Beyond Chat — Get Things Done</strong></p>
  <p><em>Local-first · Data Stays Local · Connect Your Own LLM / Files / Notes / Databases</em></p>

  <p>
    <a href="https://github.com/alib8b8/aflare/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/aflare/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status" />
    </a>
    <a href="https://github.com/alib8b8/aflare/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/aflare?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-AGPL%20v3.0-blue.svg?style=flat-square" alt="license" />
    </a>
  </p>
</div>

---

## Quick Start

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 | iex
```

<details>
<summary><b>Other install methods</b> (manual download / deb · rpm / GitHub Action)</summary>

```bash
# Manual binary download
#   GitHub:  https://github.com/alib8b8/aflare/releases
#   CN accelerated: https://ghproxy.com/https://github.com/alib8b8/aflare/releases
```

- `deb` / `rpm` packages are attached to each Release.
- The install script auto-switches to mirror-accelerated downloads on CN networks.

Run aflare workflows as CI steps (checksum-verified binary, no Docker build):

```yaml
- uses: alib8b8/aflare/action@v0.12.0
  with:
    workflow: .aflare/pr-review.yaml
```

See [action/README.md](action/README.md).

</details>

**Try it in 60 seconds:**

```bash
aflare doctor                      # environment self-check (zero-config)
aflare run examples/content-processor.yaml   # read post.md → HTML → write post.html
aflare init                        # configure an LLM (local Ollama or cloud provider)

aflare create "monitor BTC price, alert via Telegram when > 70000"
aflare run btc-monitor.yaml        # generate a workflow from keywords (add --ai for LLM generation)

aflare chat                        # interactive ReAct Agent chat
```

> Optional: install bubblewrap for full sandbox isolation (`code_interpreter` node) — `sudo apt install bubblewrap` / `brew install bubblewrap`.
>
> Market data in generated monitoring workflows comes from public quote APIs — for personal research only, not investment advice.

---

## What is aflare?

A **local-first automation Agent** and a **deterministic workflow engine** in a single binary. You explicitly grant access to your data (directories, note libraries, local databases), and the AI works deterministically inside the permission ceiling you define.

```
aflare chat / agent          aflare create
  ReAct Agent                  → YAML workflow
  (conversational)               ↓
       ↓                    DAG scheduled execution
  node tools                (WAL recovery · Saga · retry · audit)
```

Currently at **v0.12.0**, targeting local users first — local data lives on your machine, aflare is the deterministic and secure control layer between AI and that data.

---

## Key Features

- **Local-first, data stays local** — single binary, zero runtime deps, ~10–30MB RAM; workflows, history, memory and secrets all stay on local disk; fully offline-capable; no usage telemetry.
- **Connect your own LLM** — Ollama / vLLM / LM Studio / any OpenAI-compatible endpoint; loopback needs no API key; without an LLM, keyword matching keeps everything working offline. Multi-provider routing cuts spend and avoids lock-in: one OpenRouter endpoint for every vendor's models, or the native `llm_router` node routing by cost / latency with automatic fallback. See [LLM Routing](docs/openrouter.md).
- **Local data & API connectors** — named, explicitly-authorized connectors for directories, databases and HTTP APIs (`files` / `notes` / `sqlite` / `mysql` / `postgres` / `http`); credentials live only in the secrets store, permission ceilings can be tightened but never loosened. See [Connector API](docs/connector-api.md).
- **Deterministic runtime** — DAG parallel scheduling (TLA+ formally verified), WAL crash recovery + `--resume`, Saga transaction compensation, idempotency, retry / rate limit / circuit breaker. Every operation is traceable, replayable, verifiable.
- **Dual Agent + Workflow mode** — conversational ReAct Agent (`aflare chat`) and daemon Agent (`aflare agent`) share one core; 6 pluggable capabilities (reflection / human-in-the-loop / utility / memory / planning / workflow).
- **Agent interconnection & commanding** — aflare directs and supervises other agents: CLI channel (`codex` / `claude` / `gemini` or any generic CLI) and A2A protocol channel, with real delegation via the `supervisor` node and failure isolation per agent.
- **Security built in** — HMAC tamper-evident audit chain, AES-GCM encrypted secrets, automatic secret redaction, SSRF / path-traversal / command-injection defenses, outbound anomaly monitoring + auto circuit-break, four security levels (L0–L3).
- **Extensible ecosystem** — MCP Server / Client, custom nodes in Go, community plugins, [GitHub Action](action/README.md) for CI, 30+ built-in LLM providers, [OpenClaw plugin](contrib/openclaw/README.md) for the OpenClaw ecosystem.
- **Ready-to-run examples** — real-world workflow packs under [`examples/real-world/`](examples/real-world/): industrial monitoring (OpenFOAM divergence watchdog, similarity-RAG incident triage), DevOps CI pipelines, research, batch processing, and multi-agent role pipelines (analyst→researcher→trader→risk trading crew, digital-company marketing & sales departments).

---

## Security

Four security levels (`--security-level`): **L0** relaxed → **L3** maximum (L2 refuses unsandboxed `code_interpreter`; L3 disables it). CI runs `gofmt` / `go vet` / `gosec` / `govulncheck` on every PR.

[Security Guide →](SECURITY.md)

---

## Documentation

- [Getting Started](docs/getting-started.md) · [Tutorial](docs/tutorial.md) · [YAML Syntax](docs/getting-started.md#step-2-create-your-first-workflow)
- [Dataflow](docs/dataflow.md) · [Scheduling](docs/scheduling.md) · [MCP](docs/mcp.md) · [Plugins](docs/plugins.md) · [Connectors](docs/connector-api.md) · [LLM Routing](docs/openrouter.md)
- [Web UI](docs/webui.md) · [Visualizer](docs/visualizer.md) · [Custom Nodes](docs/custom-nodes.md)
- [API Reference](docs/api.md) · [Nodes Reference](docs/nodes-reference.md)
- [Deployment](docs/deployment.md) · [Docker](docs/docker.md) · [Multi-Tenancy](docs/tenants.md)
- [Troubleshooting](docs/troubleshooting.md) · [Changelog](CHANGELOG.md)

---

## Contributing

We welcome contributions! [Contributing →](CONTRIBUTING.md)

---

## License

GNU Affero General Public License v3.0 — [LICENSE](LICENSE)

---

<div align="center">
  <p>
    <a href="https://github.com/alib8b8/aflare">GitHub</a>
    ·
    <a href="https://github.com/alib8b8/aflare/issues">Issues</a>
    ·
    <a href="https://github.com/alib8b8/aflare/discussions">Discussions</a>
  </p>
</div>
