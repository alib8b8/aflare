# Web UI Editor

llm-box includes a built-in web-based workflow editor with visualization capabilities.

## Quick Start

### Start the Web UI

```bash
# Start on default port (8081)
llm-box webui

# Start on custom port
llm-box webui --port 8080

# Start with custom host (accessible from network)
llm-box webui --host 0.0.0.0 --port 8081

# Start with specific workflows directory
llm-box webui --workflows-dir ./workflows
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
│  │ LLM Box         │  │ Toolbar (format, validate, save)│  │
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
| `LLM_BOX_WEBUI_HOST` | Default host |
| `LLM_BOX_WEBUI_PORT` | Default port |
| `LLM_BOX_WORKFLOWS_DIR` | Default workflows directory |

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

1. Check if the server is running: `llm-box webui`
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
Description=llm-box WebUI
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/llm-box webui --host 0.0.0.0 --port 8081
Restart=on-failure

[Install]
WantedBy=multi-user.target
```
