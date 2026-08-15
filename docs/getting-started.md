# Getting Started with aflare

Welcome to aflare! This guide will get you up and running in under 5 minutes.

---

## Step 1: Installation

### Linux/macOS

Download the install script from the repository and run it locally:

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh -o install.sh
bash install.sh
```

**国内网络加速**（推荐国内用户使用）：

```bash
curl -sL https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh -o install.sh
bash install.sh
```

### Windows

Download the latest binary from the [Releases page](https://github.com/alib8b8/aflare/releases), then:

```powershell
Invoke-WebRequest -Uri "https://github.com/alib8b8/aflare/releases/latest/download/aflare-windows-amd64.exe" -OutFile aflare.exe
.\aflare.exe --help
```

**国内网络加速**（PowerShell）：

```powershell
Invoke-WebRequest -Uri "https://ghproxy.com/https://github.com/alib8b8/aflare/releases/latest/download/aflare-windows-amd64.exe" -OutFile aflare.exe
.\aflare.exe --help
```

### Build from Source

```bash
git clone https://github.com/alib8b8/aflare.git
cd aflare
go install ./cmd/aflare
```

**国内网络加速**（使用 ghproxy 代理克隆）：

```bash
git clone https://ghproxy.com/https://github.com/alib8b8/aflare.git
cd aflare
go install ./cmd/aflare
```

Verify installation:

```bash
aflare --help
```

You should see the aflare help output.

---

## Step 2: Create Your First Workflow

Let's start with a simple workflow: fetch content from a URL and save it to a file.

```bash
aflare create "fetch example.com and save to example.html"
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
aflare run example-workflow.yaml
```

You'll see the beautiful TUI showing progress:

```
╔══════════════════════════════════════════════════════════════╗
║ 🚀 aflare - Fetch and Save Example                          ║
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

Check out the [examples directory](https://github.com/alib8b8/aflare/tree/main/examples) for 10 practical workflows you can use:

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

- Open a [Discussion](https://github.com/alib8b8/aflare/discussions)
- File an [Issue](https://github.com/alib8b8/aflare/issues)
- Check the [FAQ](../README.md#faq) in the README
