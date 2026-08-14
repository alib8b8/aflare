# aflare Quick Start

aflare is a **local-first** automation agent — your data stays on your machine.

## Why aflare?

- **ReAct reasoning loop** — think, call a tool, observe, answer
- **300+ skill templates** across 16 domains
- **Deterministic workflow execution** — DAG scheduling, WAL crash recovery, Saga compensation
- **7 pluggable capabilities** — reflection, human-in-the-loop, utility, and more

## Try it now

```bash
aflare run examples/content-processor.yaml
```

This very file (`examples/post.md`) is the input. The workflow reads it,
converts it to HTML, and writes `post.html`. No LLM, no network, no config
required — a true zero-config first run.
