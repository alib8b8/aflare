# Contributing to llm-box

Welcome! We're excited you want to contribute to llm-box. This guide will help you get started.

---

## Code of Conduct

Be kind and respectful. We're all here to build something great together.

---

## Ways to Contribute

There are many ways to contribute, not just code!

### 1. Code Contributions
- Fix bugs
- Add new features
- Improve performance
- Add new built-in nodes

### 2. Documentation
- Fix typos
- Improve explanations
- Write tutorials
- Add examples

### 3. Community
- Answer questions in Discussions
- Help other users
- Share your workflows
- Write blog posts or make videos

### 4. Bug Reports & Feature Requests
- File issues when you find bugs
- Suggest new features
- Share feedback

---

## Quick Start for Contributors

### Step 1: Fork & Clone

```bash
# Fork the repo on GitHub, then clone your fork
git clone https://github.com/YOUR-USERNAME/llm-box.git
cd llm-box

# Add the upstream repo
git remote add upstream https://github.com/alib8b8/llm-box.git
```

### Step 2: Set Up Development Environment

```bash
# Install Go 1.21+ if you don't have it
# https://golang.org/doc/install

# Download dependencies
go mod download

# Run tests
go test ./...

# Build locally
go build -o llm-box ./cmd/llm-box
./llm-box --help
```

### Step 3: Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### Step 4: Make Your Changes

Make your changes, then:

```bash
# Run tests
go test ./...

# Format your code
go fmt ./...
```

### Step 5: Commit & Push

```bash
git add .
git commit -m "feat: add amazing new feature"
git push origin feature/your-feature-name
```

### Step 6: Open a Pull Request

1. Go to your fork on GitHub
2. Click "Compare & pull request"
3. Fill out the PR template
4. Submit!

---

## Building Custom Nodes

You can build custom nodes in any language!

### Node Protocol

A node is just an executable that:
1. Reads input from `stdin` (JSON)
2. Writes output to `stdout` (plain text or JSON)
3. Exits with 0 on success, non-zero on failure

### Example Node in Python

Create `nodes/hello-world/main.py`:
```python
#!/usr/bin/env python3
import sys
import json

def main():
    # Read input from stdin
    input_data = json.load(sys.stdin)
    name = input_data.get("name", "World")

    # Write output to stdout
    print(f"Hello, {name}!")

if __name__ == "__main__":
    main()
```

Create `nodes/hello-world/metadata.yaml`:
```yaml
name: hello_world
description: "A simple hello world node"
entry: main.py
inputs:
  - name: name
    description: "Name to say hello to"
```

Now you can use it in your workflows:
```yaml
name: Hello World
steps:
  - node: hello_world
    params:
      name: "llm-box"
```

See the [nodes template](../nodes/_template/) for a complete starting point.

---

## Pull Request Guidelines

- **Write clear titles**: Use conventional commits format
  - `feat: add new feature`
  - `fix: fix bug`
  - `docs: update docs`
  - `refactor: refactor code`
- **Describe your changes**: Explain what you did and why
- **Include tests**: For new features or bug fixes, add tests
- **Keep it focused**: One PR per feature or bug fix

---

## Issue Guidelines

- **Use the templates**: Bug reports and feature requests have templates
- **Be specific**: Include steps to reproduce bugs
- **Include versions**: llm-box version, OS, etc.

---

## Community

- Ask questions in [Discussions](https://github.com/alib8b8/llm-box/discussions)
- Report bugs in [Issues](https://github.com/alib8b8/llm-box/issues)

---

## Getting Help

If you need help, don't hesitate to ask! Open a Discussion and we'll help you out.

---

## Recognition

All contributors are recognized in the CONTRIBUTORS file and in release notes.

Thank you for contributing to llm-box! 🎉
