# Custom Nodes

llm-box allows you to build custom nodes in any programming language. Custom nodes extend the workflow engine with new functionality.

## Overview

Custom nodes are external scripts that communicate with llm-box via stdin/stdout using JSON protocol. This allows you to write nodes in:

- Python
- Node.js
- Bash
- Go
- Any language that can read/write JSON

## Interface Specification

### stdin Input

llm-box sends JSON input to the custom node via stdin:

```json
{
  "input": "input text from previous step",
  "params": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

### stdout Output

The custom node must return JSON output via stdout:

```json
{
  "output": "result text to pass to next step"
}
```

### Environment Variables

llm-box sets the following environment variables:

| Variable | Description |
|----------|-------------|
| `LLM_BOX_NODE_NAME` | Name of the node |
| `LLM_BOX_WORKFLOW_NAME` | Name of the current workflow |
| `LLM_BOX_STEP_INDEX` | Zero-based step index |
| `LLM_BOX_SECRETS_PASSWORD` | Secrets password (if set) |

### Sensitive Data Filtering

Sensitive parameters (API keys, passwords, secrets) are automatically filtered from the params passed to external nodes. The following key patterns are filtered:

- `api_key`
- `key`
- `secret`
- `password`
- `token`
- `credential`

## Directory Structure

Custom nodes are stored in `~/.llm-box/nodes/`:

```
~/.llm-box/nodes/
├── my_custom_node/
│   ├── node.json      # Node metadata
│   └── main.py        # Node implementation
├── database_query/
│   ├── node.json
│   └── query.js
└── shell_command/
    ├── node.json
    └── execute.sh
```

## Node Metadata (node.json)

```json
{
  "name": "my_custom_node",
  "description": "A custom node that does X",
  "version": "1.0.0",
  "author": "John Doe",
  "entrypoint": "main.py",
  "input_type": "string",
  "output_type": "string",
  "params": {
    "required_param": {
      "type": "string",
      "description": "A required parameter",
      "required": true
    },
    "optional_param": {
      "type": "string",
      "description": "An optional parameter",
      "required": false,
      "default": "default_value"
    }
  }
}
```

### Metadata Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique node name (used in workflow YAML) |
| `description` | Yes | Human-readable description |
| `version` | Yes | Semantic version |
| `author` | No | Author name |
| `entrypoint` | Yes | Script to execute |
| `input_type` | Yes | Input type (string, json, binary) |
| `output_type` | Yes | Output type (string, json, binary) |
| `params` | No | Parameter schema |

## Quick Start

### Step 1: Create Node Directory

```bash
mkdir -p ~/.llm-box/nodes/my_custom_node
```

### Step 2: Create Metadata

```json
// ~/.llm-box/nodes/my_custom_node/node.json
{
  "name": "my_custom_node",
  "description": "Echo input with custom prefix",
  "version": "1.0.0",
  "author": "John Doe",
  "entrypoint": "main.py",
  "input_type": "string",
  "output_type": "string",
  "params": {
    "prefix": {
      "type": "string",
      "description": "Prefix to add to input",
      "required": false,
      "default": "Result: "
    }
  }
}
```

### Step 3: Create Implementation

```python
#!/usr/bin/env python3
# ~/.llm-box/nodes/my_custom_node/main.py

import sys
import json

def main():
    # Read input from stdin
    payload = json.loads(sys.stdin.read())
    
    # Extract input and params
    input_data = payload.get("input", "")
    params = payload.get("params", {})
    
    # Get prefix with default
    prefix = params.get("prefix", "Result: ")
    
    # Process
    result = prefix + input_data
    
    # Write output to stdout
    print(json.dumps({"output": result}))

if __name__ == "__main__":
    main()
```

### Step 4: Make Executable

```bash
chmod +x ~/.llm-box/nodes/my_custom_node/main.py
```

### Step 5: Use in Workflow

```yaml
name: test-custom-node
steps:
  - node: my_custom_node
    params:
      prefix: "Processed: "
    input: "Hello World"
```

## Multi-Language Examples

### Python

```python
#!/usr/bin/env python3
import sys
import json

def main():
    payload = json.loads(sys.stdin.read())
    input_data = payload["input"]
    params = payload.get("params", {})
    
    # Your logic here
    result = f"Python processed: {input_data}"
    
    print(json.dumps({"output": result}))

if __name__ == "__main__":
    main()
```

### Node.js

```javascript
#!/usr/bin/env node
const fs = require('fs');

const payload = JSON.parse(fs.readFileSync(0, 'utf-8'));
const input = payload.input;
const params = payload.params || {};

// Your logic here
const result = `Node.js processed: ${input}`;

console.log(JSON.stringify({ output: result }));
```

### Bash

```bash
#!/bin/bash
read input

# Parse JSON input
INPUT_DATA=$(echo "$input" | jq -r '.input')
PARAM=$(echo "$input" | jq -r '.params.param // "default"')

# Your logic here
RESULT="Bash processed: $INPUT_DATA"

# Output JSON
echo "{\"output\": \"$RESULT\"}"
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type Input struct {
    Input  string            `json:"input"`
    Params map[string]string `json:"params"`
}

type Output struct {
    Output string `json:"output"`
}

func main() {
    var in Input
    json.NewDecoder(os.Stdin).Decode(&in)
    
    // Your logic here
    result := fmt.Sprintf("Go processed: %s", in.Input)
    
    out := Output{Output: result}
    json.NewEncoder(os.Stdout).Encode(out)
}
```

## Testing Custom Nodes

### Manual Testing

```bash
echo '{"input": "test", "params": {"prefix": "Test: "}}' | python3 ~/.llm-box/nodes/my_custom_node/main.py
```

### Using llm-box run-node

```bash
llm-box run-node my_custom_node --input "Hello" --params '{"prefix": "Output: "}'
```

### Integration Testing

```yaml
name: test-custom-node
steps:
  - node: file_read
    params:
      path: "test_input.txt"
    id: input

  - node: my_custom_node
    params:
      prefix: "Processed: "
    input: "{{step.input}}"
    id: custom

  - node: file_write
    params:
      path: "test_output.txt"
    input: "{{step.custom}}"
```

## Debugging

### Logging

Write debug output to stderr:

```python
import sys

def debug(message):
    print(f"DEBUG: {message}", file=sys.stderr)
```

### Error Handling

Return errors via stdout with "error" field:

```json
{
  "error": "Failed to process input: invalid format"
}
```

### Viewing Logs

```bash
tail -f ~/.llm-box/logs/audit.log | grep my_custom_node
```

## Advanced Features

### Using Secrets

Custom nodes can access secrets via the expression engine:

```yaml
steps:
  - node: my_custom_node
    params:
      api_key: "{{secret.api.service}}"
```

**Note**: Sensitive parameters are filtered from the params passed to external nodes. Access secrets through environment variables instead:

```bash
SECRET=$(llm-box secrets get --group api --key service)
```

### Using Environment Variables

```python
import os

workflow_name = os.environ.get("LLM_BOX_WORKFLOW_NAME", "unknown")
step_index = os.environ.get("LLM_BOX_STEP_INDEX", "0")
```

### Binary Data

For binary data, encode as base64:

```python
import base64

# Read binary input
binary_data = base64.b64decode(input_data)

# Process...

# Output binary as base64
output_b64 = base64.b64encode(result).decode('utf-8')
```

## Performance Tips

1. **Keep It Light**: Avoid heavy dependencies
2. **Cache Results**: Cache expensive operations
3. **Use Efficient Libraries**: Choose fast libraries for processing
4. **Avoid Network Calls**: Minimize external API calls
5. **Batch Processing**: Process data in batches when possible

## Security Best Practices

1. **Validate Inputs**: Always validate input data
2. **Sanitize Output**: Sanitize output to prevent injection
3. **Limit Permissions**: Run nodes with minimal permissions
4. **Handle Secrets Carefully**: Don't log secrets
5. **Avoid Shell Execution**: Don't execute user input as shell commands

## Publishing Custom Nodes

### Package Structure

```
my-custom-node/
├── node.json
├── main.py
├── README.md
└── test/
    ├── test_input.json
    └── test_output.json
```

### Sharing

1. Create a GitHub repository
2. Add documentation in README.md
3. Include test cases
4. Tag releases with semantic versioning

### Installation

Users can install your node by cloning to `~/.llm-box/nodes/`:

```bash
git clone https://github.com/yourname/my-custom-node ~/.llm-box/nodes/my-custom-node
```

## Troubleshooting

### Node Not Found

1. Check directory structure: `ls ~/.llm-box/nodes/my_custom_node/`
2. Verify `node.json` exists and has correct format
3. Check entrypoint script is executable
4. Verify node name matches exactly

### Execution Failed

1. Run node manually with test input
2. Check stderr for error messages
3. Verify dependencies are installed
4. Check script syntax

### Timeout

1. Increase timeout in workflow: `timeout: "60s"`
2. Optimize node execution time
3. Break long operations into smaller steps

### JSON Parsing Error

1. Ensure output is valid JSON
2. Check for trailing commas
3. Verify UTF-8 encoding
4. Use proper escaping for special characters
