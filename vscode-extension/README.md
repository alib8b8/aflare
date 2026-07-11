# llm-box

Build terminal workflows using plain English. Describe what you want, **llm-box** generates the YAML and executes it — right inside VS Code.

`llm-box` turns natural-language descriptions into reproducible, auditable YAML workflows that chain together fetching URLs, running commands, calling LLMs, transforming data, and writing files. This extension brings the full `llm-box` CLI experience into the editor with a sidebar explorer, one-click run/validate, snippets, and a node manager.

![llm-box overview](images/overview.png)

---

## ✨ Features

- 🪄 **Create workflows from a description** — open the Command Palette, run `llm-box: Create Workflow from Description`, type what you want in plain English, and the generated YAML file opens in your editor.
- ▶️ **Run & validate with one click** — editor-title buttons let you run or validate the active `.yaml`/`.yml` file instantly. Right-click any workflow file in the Explorer for the same actions.
- 🗂️ **Workflow explorer** — a sidebar view lists every `.yaml`/`.yml` file in your workspace. Click to open, refresh to re-scan.
- 🧩 **Node browser** — a second sidebar view lists every node available to `llm-box` (fetch_url, file_write, ollama, deepseek, http_request, and more) with descriptions, pulled live from `llm-box list`.
- 📦 **Node & registry management** — install, uninstall, sync, list, and search the llm-box node registry without leaving VS Code.
- 🧱 **YAML snippets** — eight ready-made snippets for common node patterns (basic workflow, fetch URL, file write, execute, transform, HTTP request, notify, and a full LLM-summary pipeline).
- 🌐 **Multi-language CLI output** — pass `--lang` to the CLI automatically, supporting 9 languages.
- 🔒 **Safe mode** — run workflows with the `execute` node disabled, controlled by a single setting.

---

## 📋 Requirements

- The [`llm-box`](https://github.com/alib8b8/llm-box) CLI must be installed and available on your `PATH` (or configure its location in Settings).
- VS Code 1.85 or newer.

Install the CLI:

```bash
# Linux / macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh

# Or via Go
go install github.com/alib8b8/llm-box/cmd/llm-box@latest
```

Verify it works:

```bash
llm-box help
```

---

## 🚀 Installation

### From the Marketplace

1. Open the Extensions view (`Ctrl+Shift+X` / `Cmd+Shift+X`).
2. Search for **llm-box**.
3. Click **Install**.
4. Reload VS Code.

### From a VSIX

```bash
code --install llm-box-0.4.0.vsix
```

### From source

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box/vscode-extension
npm install
npm run package
```

---

## 📖 Usage

### Creating a workflow

1. Open the Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`).
2. Run **llm-box: Create Workflow from Description**.
3. Describe your workflow, for example:

   > `fetch the Hacker News front page and save the result to hn.txt`

4. The generated `.yaml` file opens automatically. Review and tweak it if needed.

![Create workflow](images/create-workflow.png)

### Running a workflow

- Open any `.yaml`/`.yml` file and click the **Run** (`▶`) button in the editor title bar, or
- Right-click a workflow file in the Explorer and choose **llm-box: Run Workflow**, or
- Run **llm-box: Run Workflow** from the Command Palette and pick a file.

By default the output is shown in the **llm-box** output channel. Disable `llm-box.outputChannel` to run it in an integrated terminal instead (useful for the interactive TUI).

### Validating a workflow

Click the **Validate** (`✓`) button in the editor title bar, or run **llm-box: Validate Workflow**. Warnings are shown inline as a notification.

### Browsing nodes

Open the **llm-box** activity bar icon. The **Workflows** view lists your YAML files; the **Nodes** view lists every node available to the CLI. Use the refresh button to reload either view.

### Installing a node

Run **llm-box: Install Node**, enter the node name (e.g. `weather_api`), and confirm. The Nodes view refreshes automatically.

### Searching the registry

Run **llm-box: Registry Search**, type a query, and the results are printed to the **llm-box** output channel.

---

## ⚙️ Configuration

Open **File → Preferences → Settings** and search for `llm-box`.

| Setting | Type | Default | Description |
| --- | --- | --- | --- |
| `llm-box.executablePath` | `string` | `""` | Path to the `llm-box` executable. Leave empty to auto-detect from `PATH`. |
| `llm-box.safeMode` | `boolean` | `true` | Run workflows in safe mode (disables the `execute` node). |
| `llm-box.outputChannel` | `boolean` | `true` | Show workflow output in a VS Code output channel. When `false`, runs in an integrated terminal. |
| `llm-box.language` | `enum` | `"en"` | Language for `llm-box` CLI output. One of: `en`, `zh`, `ru`, `fr`, `ja`, `ko`, `es`, `ar`, `hi`. |

---

## 🧱 Snippets

Type the prefix in a `.yaml` file and press `Tab`:

| Prefix | Inserts |
| --- | --- |
| `llm-box-basic` | Basic workflow skeleton (name + steps) |
| `llm-box-fetch-url` | A `fetch_url` step |
| `llm-box-file-write` | A `file_write` step |
| `llm-box-execute` | An `execute` step (disabled in safe mode) |
| `llm-box-transform` | A `transform` step |
| `llm-box-http-request` | An `http_request` step |
| `llm-box-notify` | A `notify` step |
| `llm-box-llm-summary` | A full fetch → LLM (ollama/deepseek) → file_write pipeline |

---

## ⚠️ Known Issues

- The Nodes view requires the `llm-box` CLI to be reachable; if it is not installed the view will simply be empty.
- When running in the output channel, interactive TUI features of the CLI are not available — switch to terminal mode (`llm-box.outputChannel: false`) for the full TUI.
- Generated workflow files are created in the CLI's working directory (the first workspace folder); make sure a folder is open.
- On Windows, configure `llm-box.executablePath` if `llm-box` is not on `PATH`.

Report issues at [github.com/alib8b8/llm-box/issues](https://github.com/alib8b8/llm-box/issues).

---

## 📝 Release Notes

### 0.4.0

Initial release of the **llm-box** VS Code extension.

- Workflow creation from a natural-language description.
- Run and validate workflows from the editor title bar, context menu, and Command Palette.
- Sidebar Workflow explorer and Node browser.
- Node install/uninstall and registry sync/list/search.
- Eight YAML snippets for common node patterns.
- Configurable executable path, safe mode, output channel, and CLI language.

See the [CHANGELOG](./CHANGELOG.md) for details.

---

## 📄 License

[MIT](https://github.com/alib8b8/llm-box/blob/main/LICENSE)
