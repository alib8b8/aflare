# llm-box

A zero-code AI workflow engine that runs in your terminal.

## Features

- 📝 Define workflows with simple YAML
- 🔄 Chain nodes together to process text data
- 📟 Real-time TUI for monitoring execution
- 🔌 Extensible - community can add new nodes easily
- 💻 Completely offline, powered by Ollama

## Quick Start

```bash
# Run an example Ollama workflow (requires local Ollama)
llm-box examples/ollama_chat.yaml
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

## Project Structure

```
llm-box/
├── cmd/
│   └── llm-box/
│       └── main.go          # Entry point
├── internal/
│   ├── workflow/            # Workflow parsing & execution
│   │   ├── types.go         # Core data structures
│   │   └── parser.go        # YAML parser
│   ├── nodes/               # Built-in node implementations
│   └── tui/                 # Terminal user interface
├── nodes/                   # Community-contributed nodes
├── examples/                # Example workflow files
└── go.mod
```

## License

MIT
