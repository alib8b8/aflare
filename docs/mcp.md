# MCP (Model Context Protocol) Integration

aflare supports the Model Context Protocol (MCP) for connecting external tools and services to AI workflows.

## Overview

MCP allows aflare to act as a tool server that can be called by AI agents. This enables:

- External AI applications to use aflare workflows
- Integration with AI IDEs and chat interfaces
- Programmatic access to aflare capabilities

## Quick Start

### Start MCP Server

```bash
# Start MCP server (stdio transport, the default)
aflare mcp

# Equivalent flag form
aflare --mcp-server

# HTTP transport (token required) — see "HTTP Mode" below
aflare mcp --port 8082 --token "$MCP_TOKEN"
```

### Connect from AI Agent

The MCP server follows the JSON-RPC 2.0 protocol over stdin/stdout (stdio
transport) or HTTP (see "HTTP Mode" below).

## Protocol Specification

### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {}
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "capabilities": {
      "tools": {
        "listChanged": false
      }
    },
    "serverInfo": {
      "name": "aflare",
      "version": "0.7.0"
    }
  }
}
```

`serverInfo.version` echoes the installed aflare version (shown illustratively
as `0.7.0` above); query `aflare version` for the exact value on your host.

### List Tools

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

### Call Tool

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "workflow_run",
    "arguments": {
      "file": "my-workflow.yaml",
      "timeout_seconds": 30
    }
  }
}
```

## Available Tools

### workflow_run

Execute a workflow file.

**Parameters**:
- `file` (required): Path to workflow YAML file
- `timeout_seconds` (optional): Timeout in seconds (default: 30, max: 300)

**Example**:
```json
{
  "name": "workflow_run",
  "arguments": {
    "file": "data-processing.yaml"
  }
}
```

### workflow_create

Generate a workflow from a description.

**Parameters**:
- `description` (required): Natural language description of the workflow
- `name` (optional): Name for the generated workflow

**Example**:
```json
{
  "name": "workflow_create",
  "arguments": {
    "description": "Fetch data from API, summarize with GPT-4, save to file",
    "name": "data-summarizer"
  }
}
```

### workflow_list

List workflow files in a directory.

**Parameters**:
- `directory` (optional): Directory to search (default: current directory)

**Example**:
```json
{
  "name": "workflow_list",
  "arguments": {
    "directory": "./workflows"
  }
}
```

### Other tools

aflare exposes many more MCP tools than the workflow trio above. The
authoritative list is available at runtime via `tools/list`; the groups are:

| Group | Tools |
|-------|-------|
| Workflow | `workflow_run`, `workflow_create`, `workflow_list`, `workflow_validate` |
| Nodes | `node_list`, `node_info` |
| History | `history_list` |
| Memory | `memory_store`, `memory_retrieve`, `memory_search`, `memory_stats`, `memory_list_sessions` |
| Code knowledge graph | `code_graph_index`, `code_graph_query`, `code_graph_stats` |
| Context | `context_compress` |
| Search | `search_aggregated` |
| Geospatial | `geospatial_query` |
| Preferences | `preference_get`, `preference_set` |

Backwards-compatible aliases (`create_workflow`, `run_workflow`,
`run_workflow_yaml`, `list_nodes`) are also accepted by `tools/call`.

> **Note:** secrets are **not** exposed as MCP tools. Manage them with the
> `aflare secrets` CLI (`set` / `get` / `list`) so secret values never transit
> the MCP JSON-RPC channel.

## Configuration

### CLI Options

| Option | Default | Description |
|--------|---------|-------------|
| `--port` | (stdin/stdout) | HTTP port to listen on |
| `--host` | `127.0.0.1` | Host address to bind |
| `--verbose` | false | Enable verbose logging |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AFLARE_MCP_PORT` | Default HTTP port |
| `AFLARE_SECRETS_PASSWORD` | Required for secrets operations |

## Integration Examples

### LangChain Integration

```python
from langchain.tools import StructuredTool

def run_workflow(file_path: str, timeout: int = 30) -> str:
    import requests
    response = requests.post(
        "http://localhost:8082/tools/call",
        json={
            "name": "workflow_run",
            "arguments": {"file": file_path, "timeout_seconds": timeout}
        }
    )
    return response.json()["result"]["content"][0]["text"]

workflow_tool = StructuredTool.from_function(run_workflow)
```

### OpenAI Assistant Integration

```python
from openai import OpenAI

client = OpenAI()

assistant = client.beta.assistants.create(
    name="Workflow Assistant",
    instructions="Use the workflow_run tool to execute aflare workflows",
    model="gpt-4o",
    tools=[{
        "type": "function",
        "function": {
            "name": "workflow_run",
            "description": "Execute a workflow file",
            "parameters": {
                "type": "object",
                "properties": {
                    "file": {"type": "string", "description": "Path to workflow file"},
                    "timeout_seconds": {"type": "integer", "description": "Timeout in seconds"}
                },
                "required": ["file"]
            }
        }
    }]
)
```

### HTTP Mode

The MCP server can also run over HTTP instead of stdio. HTTP mode is
**token-required**: the listener is a network surface, so unlike loopback
stdio there is no auth-free mode.

```bash
# Start MCP server in HTTP mode (binds 127.0.0.1 by default)
aflare mcp --port 8082 --token "$MCP_TOKEN"

# Or take the token from the environment
AFLARE_MCP_TOKEN=... aflare mcp --port 8082
```

| Flag | Description |
|------|-------------|
| `--port <port>` | Enable HTTP mode and listen on this port |
| `--host <host>` | Bind address (default `127.0.0.1`; `0.0.0.0` must be explicit) |
| `--token <token>` | Required auth token (or `AFLARE_MCP_TOKEN`) |

Every request must carry the token in the `X-MCP-Token` header.

**Endpoint 1: `POST /mcp`** — standard JSON-RPC 2.0, the same message format
the stdio transport accepts (one request per POST, one response per request):

```bash
curl -X POST http://localhost:8082/mcp \
  -H "Content-Type: application/json" \
  -H "X-MCP-Token: $MCP_TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

**Endpoint 2: `POST /v1/call`** — simplified direct tool call without the
JSON-RPC envelope; the response body is the tool result:

```bash
curl -X POST http://localhost:8082/v1/call \
  -H "Content-Type: application/json" \
  -H "X-MCP-Token: $MCP_TOKEN" \
  -d '{
    "name": "workflow_run",
    "arguments": {"file": "test.yaml"}
  }'
```

Notes:

- Request bodies are capped at 1 MiB; oversized bodies get `413`.
- In-flight requests are capped at 100 concurrent executions (the same
  cap the webhook server enforces); excess requests get `503` with a
  retry hint.
- Request reads time out after 5 minutes — a trickling client cannot
  hold a connection open indefinitely (tool responses are not affected:
  they may legitimately take minutes to produce).
- For production, prefer `AFLARE_MCP_TOKEN` over `--token`: command-line
  arguments are visible to every local user via `ps` / `/proc/<pid>/cmdline`,
  while the environment is only readable by the same user.
- Tool-level failures return HTTP 200 with a JSON-RPC error object (so
  clients can parse errors uniformly); transport failures use 4xx status
  codes (401 unauthorized, 405 wrong method, 400 malformed body, 503 busy).

## Security

- Secrets are redacted from error messages
- File paths in errors are sanitized (replaced with `~`)
- For production, use HTTPS and network restrictions
- Limit MCP server access to trusted applications

## Troubleshooting

### Connection Issues

1. Ensure the MCP server is running
2. Check the port configuration
3. Verify network connectivity

### Tool Call Errors

1. Check the tool name and parameters
2. Verify workflow file exists
3. Check secrets password is set
4. Review server logs for details

### Timeout Issues

1. Increase `timeout_seconds` parameter
2. Optimize workflow execution time
3. Check for long-running steps

## Best Practices

1. **Use Timeouts**: Always set reasonable timeouts for tool calls
2. **Validate Inputs**: Validate workflow files before calling
3. **Handle Errors**: Implement error handling for failed workflow executions
4. **Secure Communications**: Use HTTPS in production environments
5. **Limit Permissions**: Run MCP server with minimal permissions
