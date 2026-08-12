# Web UI Editor

aflare includes a built-in web-based workflow editor with visualization capabilities.

## Quick Start

### Start the Web UI

```bash
# Start on default port (8081)
aflare webui

# Start on custom port
aflare webui --port 8080

# Start with custom host (accessible from network)
aflare webui --host 0.0.0.0 --port 8081

# Start with specific workflows directory
aflare webui --workflows-dir ./workflows
```

### Access the UI

Open your browser and navigate to:

```
http://localhost:8081
```

## Features

### Workflow Management

- **Create**: Create new workflows from scratch
- **Load**: Load existing workflows from the file system
- **Save**: Save workflows to YAML files
- **Delete**: Delete workflows (with confirmation)

### Visual Workflow Editor

- **YAML Editor**: Syntax-highlighted YAML editor
- **Real-time Validation**: Validate workflows as you edit
- **Visual Preview**: Preview workflow diagrams in multiple formats

### Visualization Formats

| Format | Description |
|--------|-------------|
| **Mermaid** | Interactive flowchart diagrams |
| **JSON** | Structured data for custom rendering |
| **DOT** | Graphviz DOT format |
| **ASCII** | Text-based diagram for terminal |

## UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Sidebar                    Main Area                      │
│  ┌─────────────────┐  ┌─────────────────────────────────┐  │
│  │ Aflare          │  │ Toolbar (format, validate, save)│  │
│  ├─────────────────┤  ├─────────────────────────────────┤  │
│  │ Workflow List   │  │ Tabs (Editor / Preview)         │  │
│  │ - workflow1     │  │                                 │  │
│  │ - workflow2     │  │ Editor:                         │  │
│  │ - workflow3     │  │   [YAML text area]              │  │
│  ├─────────────────┤  │                                 │  │
│  │ + New Workflow  │  │ Preview:                        │  │
│  └─────────────────┘  │   [Mermaid/JSON/DOT/ASCII]      │  │
│                       ├─────────────────────────────────┤  │
│                       │ Status Bar (validation, steps)  │  │
│                       └─────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## API Endpoints

The Web UI exposes REST API endpoints for programmatic access:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/workflows` | GET | List all workflows |
| `/api/workflow` | GET | Get workflow by name |
| `/api/workflow` | POST | Save workflow |
| `/api/workflow` | DELETE | Delete workflow |
| `/api/validate` | POST | Validate workflow YAML |
| `/api/visualize` | POST | Generate visualization |

### Example: Validate Workflow

```bash
curl -X POST http://localhost:8081/api/validate \
  -H "Content-Type: application/json" \
  -d '{"workflow": "name: test\nsteps:\n  - node: agent\n    params:\n      model: gpt-4o"}'
```

### Example: Generate Mermaid Diagram

```bash
curl -X POST "http://localhost:8081/api/visualize?format=mermaid" \
  -H "Content-Type: application/json" \
  -d '{"workflow": "name: test\nsteps:\n  - node: agent\n    params:\n      model: gpt-4o"}'
```

## Configuration

### CLI Options

| Option | Default | Description |
|--------|---------|-------------|
| `--host` | `127.0.0.1` | Host address to bind |
| `--port` | `8081` | Port to listen on |
| `--workflows-dir` | Current directory | Directory to load/save workflows |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AFLARE_WEBUI_HOST` | Default host |
| `AFLARE_WEBUI_PORT` | Default port |
| `AFLARE_WORKFLOWS_DIR` | Default workflows directory |
| `AFLARE_METRICS` | Set to `1` to enable the Prometheus `/metrics` endpoint (disabled by default) |
| `AFLARE_PPROF` | Set to `1` to enable the `/debug/pprof/` profiling endpoints (disabled by default) |

## Prometheus Metrics

The Web UI server can expose a Prometheus `/metrics` endpoint that scrapes
aflare's internal statistics (node/workflow execution, security blocks, LLM
calls, cache hits). The endpoint is **disabled by default** for security: it is
unauthenticated (Prometheus scrapers usually carry no auth token) and exposes
runtime statistics, so it should only be enabled on a trusted network or behind
a reverse proxy.

### Enabling the endpoint

Set `AFLARE_METRICS=1` before starting the server:

```bash
AFLARE_METRICS=1 aflare webui --host 0.0.0.0 --port 8081
```

The endpoint is then available at `http://localhost:8081/metrics`. It is
rate-limited (token bucket, ~5 req/s) to protect against scraper floods and is
**not** behind the `X-Auth-Token` middleware, matching the Prometheus scrape
convention.

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: "aflare"
    static_configs:
      - targets: ["localhost:8081"]
    metrics_path: /metrics
```

### Exposed metrics

| Metric | Type | Labels | Source |
|--------|------|--------|--------|
| `aflare_node_executions_total` | counter | `node_name`, `status` | `Registry.ExecuteWithStats` |
| `aflare_node_execution_duration_seconds` | histogram | `node_name` | `Registry.ExecuteWithStats` |
| `aflare_workflow_executions_total` | counter | `status` | workflow executor |
| `aflare_workflow_execution_duration_seconds` | histogram | — | workflow executor |
| `aflare_security_blocks_total` | counter | `block_type` | `SecurityStats.RecordBlock` |
| `aflare_cache_hits_total` | counter | — | `CacheStats` (pull via `CollectSnapshot`) |
| `aflare_cache_misses_total` | counter | — | `CacheStats` (pull via `CollectSnapshot`) |
| `aflare_llm_calls_total` | counter | `provider`, `model`, `status` | workflow LLM trace |
| `aflare_llm_tokens_total` | counter | `provider`, `model`, `type` | workflow LLM trace |
| `aflare_llm_cost_usd_total` | counter | `provider`, `model` | workflow LLM trace |
| `aflare_node_calls` | gauge | `node_name` | `Registry` stats snapshot |
| `aflare_node_errors` | gauge | `node_name` | `Registry` stats snapshot |
| `aflare_security_blocks` | gauge | `block_type` | `SecurityStats` snapshot |

Hot-path counters (node/workflow/security/LLM) are incremented inline at the
execution sites. Cache counters and the snapshot gauges are pulled on each
scrape from the existing internal stats accumulators via `CollectSnapshot`.

> **Note:** Labelled counters only appear in the output once a label
> combination has been touched (e.g. `aflare_node_executions_total` shows up
> after the first node execution). Plain counters (cache hits/misses) are
> always emitted, even at zero.

## Usage Tips

### Keyboard Shortcuts

- `Ctrl+S`: Save workflow (via Save button)
- `Ctrl+Enter`: Validate workflow

### Best Practices

1. **Validate First**: Always validate your workflow before running
2. **Use Descriptive Names**: Give workflows meaningful names
3. **Save Regularly**: The UI doesn't auto-save
4. **Preview Diagrams**: Use the preview to verify workflow structure
5. **Check Status Bar**: The status bar shows validation results and step count

## Security

- The Web UI is intended for local development use
- By default, it only binds to `127.0.0.1` (localhost)
- For production, use a reverse proxy (nginx, Caddy) with TLS
- No authentication is built-in; secure with firewall rules

## Troubleshooting

### UI Not Accessible

1. Check if the server is running: `aflare webui`
2. Verify the port: `netstat -tlnp | grep 8081`
3. Check firewall rules
4. Try accessing with `--host 0.0.0.0`

### Workflows Not Loading

1. Check the workflows directory
2. Ensure files have `.yaml` or `.yml` extension
3. Verify file permissions

### Mermaid Preview Not Rendering

1. Check browser console for errors
2. Ensure internet connection (Mermaid CDN)
3. Try refreshing the page

## Systemd Service

For production deployments:

```ini
[Unit]
Description=aflare WebUI
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/aflare webui --host 0.0.0.0 --port 8081
Restart=on-failure

[Install]
WantedBy=multi-user.target
```
