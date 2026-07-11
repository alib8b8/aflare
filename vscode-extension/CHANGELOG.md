# Changelog

All notable changes to the **llm-box** VS Code extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-07-11

### Added
- Initial release of the llm-box VS Code extension.
- `llm-box: Create Workflow from Description` command that calls `llm-box create` and opens the generated YAML file.
- `llm-box: Run Workflow` and `llm-box: Run Current Workflow File` commands with editor-title and context-menu integration; output is shown in an output channel or an integrated terminal.
- `llm-box: Validate Workflow` and `llm-box: Validate Current Workflow` commands that surface validation warnings inline.
- `llm-box: List Available Nodes` command plus a **Nodes** tree view populated from `llm-box list`.
- `llm-box: Install Node` and `llm-box: Uninstall Node` commands.
- `llm-box: Registry Sync`, `llm-box: Registry List`, and `llm-box: Registry Search` commands.
- `llm-box: Open Workflow File` command and a **Workflows** tree view that scans the workspace for `.yaml`/`.yml` files.
- `llm-box: Refresh` command to reload both tree views.
- Activity bar view container with a sidebar icon.
- Eight YAML snippets for common workflow patterns (basic workflow, fetch_url, file_write, execute, transform, http_request, notify, and an LLM-summary pipeline).
- Configuration settings for `llm-box.executablePath`, `llm-box.safeMode`, `llm-box.outputChannel`, and `llm-box.language`.
- CLI wrapper (`LlmBoxCli`) with helpful error messages, including a dedicated message when the `llm-box` executable cannot be found.

[0.4.0]: https://github.com/alib8b8/llm-box/releases/tag/vscode-v0.4.0
