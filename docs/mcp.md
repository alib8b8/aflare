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
# Start MCP server
aflare mcp

# Start on custom port
aflare mcp --port 8082

# Enable verbose logging
aflare mcp --verbose
```

### Connect from AI Agent

The MCP server follows the JSON-RPC 2.0 protocol over stdin/stdout or HTTP.

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
      "version": "0.3.0"
    }
  }
}
```

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

### secrets_list

List available secrets.

**Parameters**:
- `group` (optional): Filter by secret group

**Example**:
```json
{
  "name": "secrets_list",
  "arguments": {
    "group": "llm"
  }
}
```

### secrets_add

Add a new secret.

**Parameters**:
- `group` (required): Secret group name
- `key` (required): Secret key name
- `value` (required): Secret value

**Example**:
```json
{
  "name": "secrets_add",
  "arguments": {
    "group": "api",
    "key": "service",
    "value": "sk-..."
  }
}
```

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

```bash
# Start MCP server in HTTP mode
aflare mcp --port 8082

# Call via HTTP
curl -X POST http://localhost:8082/v1/call \
  -H "Content-Type: application/json" \
  -d '{
    "name": "workflow_run",
    "arguments": {"file": "test.yaml"}
  }'
```

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
