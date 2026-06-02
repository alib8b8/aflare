# llm-box

A zero-code AI workflow engine that runs in your terminal.

## Features

- 📝 Define workflows with simple YAML
- 🔄 Chain nodes together to process text data
- 📟 Beautiful real-time TUI for monitoring execution
- 🔌 Extensible - community can add new nodes easily
- 💻 Completely offline, powered by Ollama

## Quick Start

```bash
# Run an example workflow (uses TUI if terminal is interactive)
llm-box examples/test_workflow.yaml

# Run with Ollama (requires local Ollama)
llm-box examples/ollama_chat.yaml

# Complete example: fetch URL → summarize with Ollama → save to file
llm-box examples/complete_workflow.yaml
```

## Built-in Nodes

### `ollama`
Calls your local Ollama models.

**Parameters:**
- `model`: Model name (default: `llama3`)
- `endpoint`: Ollama API endpoint (default: `http://localhost:11434`)

**Example:**
```yaml
- node: ollama
  params:
    model: "llama3"
```

### `fetch_url`
Fetches content from a URL.

**Parameters:**
- `url`: URL to fetch (or pass URL as input)

**Example:**
```yaml
- node: fetch_url
  params:
    url: "https://example.com"
```

### `file_write`
Writes content to a file.

**Parameters:**
- `path`: Output file path (required)

**Example:**
```yaml
- node: file_write
  params:
    path: "output.txt"
```

## Project Structure

```
llm-box/
├── cmd/
│   └── llm-box/
│       └── main.go          # Entry point
├── internal/
│   ├── workflow/            # Workflow parsing & execution
│   │   ├── types.go         # Core data structures
│   │   ├── parser.go        # YAML parser
│   │   └── executor.go      # Workflow execution engine
│   ├── nodes/               # Built-in node implementations
│   │   ├── node.go          # Node interface & registry
│   │   ├── test_node.go     # Test node
│   │   ├── ollama.go        # Ollama node
│   │   ├── fetch_url.go     # URL fetch node
│   │   └── file_write.go    # File write node
│   └── tui/                 # Terminal user interface
│       └── model.go         # Bubbletea TUI model
├── nodes/                   # Community-contributed nodes
├── examples/                # Example workflow files
├── go.mod
├── go.sum
└── README.md
```

## Technologies

- **Go** - Static binaries, easy distribution
- **Bubbletea** - Terminal UI framework
- **Lipgloss** - Terminal styling
- **Ollama** - Local LLM inference

## Contributing Nodes

Want to add a new node? It's easy!

1. **Create a node directory** in `nodes/` (e.g., `nodes/your_node_name/`)
2. **Add metadata.yaml**:
   ```yaml
   name: "your_node_name"
   description: "Your node's description"
   entry: "main.py"
   ```
3. **Write your entry script** - Python, Bash, Go, Rust, any language!
4. **Test it** - run `llm-box` to see your node loaded
5. **Submit a PR** - share it with the community!

### Node Protocol

Your node receives JSON via stdin:
```json
{
  "input": "text from previous node",
  "params": {"key": "value"}
}
```

Your node writes output to stdout (plain text).

See `nodes/README.md` for more details and examples!

## License

MIT
