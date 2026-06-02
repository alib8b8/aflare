# Getting Started with llm-box

Welcome to llm-box! This guide will get you up and running in under 5 minutes.

---

## Step 1: Installation

### Linux/macOS

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

### Windows

Download the latest binary from the [Releases page](https://github.com/alib8b8/llm-box/releases), then:

```powershell
Invoke-WebRequest -Uri "https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-windows-amd64.exe" -OutFile llm-box.exe
.\llm-box.exe --help
```

### Build from Source

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go install ./cmd/llm-box
```

Verify installation:

```bash
llm-box --help
```

You should see the llm-box help output.

---

## Step 2: Create Your First Workflow

Let's start with a simple workflow: fetch content from a URL and save it to a file.

```bash
llm-box create "fetch example.com and save to example.html"
```

This will generate a file called `example-workflow.yaml` with something like:

```yaml
name: Fetch and Save Example
steps:
  - node: fetch_url
    params:
      url: https://example.com
  - node: file_write
    params:
      path: example.html
```

---

## Step 3: Run Your Workflow

```bash
llm-box run example-workflow.yaml
```

You'll see the beautiful TUI showing progress:

```
╔══════════════════════════════════════════════════════════════╗
║ 🚀 llm-box - Fetch and Save Example                          ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║ 📝 Fetching URL...                                           ║
║    → fetch_url (https://example.com)                         ║
║                                                              ║
║ ✅ workflow completed in 0.8s                                ║
╚══════════════════════════════════════════════════════════════╝
```

Check the output file:

```bash
cat example.html
```

---

## Step 4: Explore More Workflows

Check out the [examples directory](https://github.com/alib8b8/llm-box/tree/main/examples) for 10 practical workflows you can use:

- Daily GitHub summary
- Research assistant
- Documentation generator
- Log monitor
- Release notes creator
- Data collector
- File organizer
- Content workflow
- DevOps automation
- Team reporting

Try them out and modify them to fit your needs!

---

## Step 5: Learn More

- Check out the full [Architecture](architecture.md) guide
- Read the [Examples](examples.md) doc
- See the [Roadmap](roadmap.md) for what's coming next
- Learn how to [Contribute](contributing.md)

---

## Need Help?

- Open a [Discussion](https://github.com/alib8b8/llm-box/discussions)
- File an [Issue](https://github.com/alib8b8/llm-box/issues)
- Check the [FAQ](../README.md#faq) in the README
