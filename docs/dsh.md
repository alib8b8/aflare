# DeepSeek Harness (DSH) Integration

aflare integrates with [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)
(DSH, MIT-licensed, developer preview) in two complementary ways:

- **Option A — MCP bridge (zero code):** DSH ships a built-in MCP client
  (`@deepseek-ai/dsh-mcp-client`). aflare already runs a stdio MCP server
  (`aflare mcp`), so a single `cordis.yml` entry exposes all 27 aflare tools
  to the DSH agent as `mcp__aflare__<tool>`.
- **Option B — native Cordis plugin (TypeScript):** the plugin skeleton in
  [`integrations/dsh-plugin/`](../integrations/dsh-plugin/) registers curated
  aflare CLI tools natively in the DSH tool registry for a first-class UI
  experience.

Both options run entirely on your machine — the DSH agent spawns aflare
locally, and workflow data never leaves your host. This matches the aflare
local-first / data-stays-home guarantee.

## Option A: MCP bridge (recommended first step)

### Prerequisites

- `aflare` installed and on `PATH` (verify with `aflare version`)
- DSH running (e.g. `npx @deepseek-ai/dsh web`)

### Configure

Add one entry to the DSH `cordis.yml` profile (or use the `--patch` overlay
shown below):

```yaml
- id: mcp-aflare
  name: '@deepseek-ai/dsh-mcp-client'
  config:
    serverName: aflare
    transport: stdio
    command: aflare
    args: ['mcp']
```

As a patch overlay (no profile edit needed), save as `aflare-patch.yml`
next to your DSH checkout and start with `pnpm dsh web --patch ./aflare-patch.yml`:

```yaml
- insert:
    - id: mcp-aflare
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: aflare
        transport: stdio
        command: aflare
        args: ['mcp']
```

### Verify

In a DSH session, ask:

```
List my available aflare tools, then call the aflare version tool.
```

The model should see `mcp__aflare__list_nodes`, `mcp__aflare__run_workflow_yaml`,
`mcp__aflare__template_list`, `mcp__aflare__memory_search`, and the rest of the
[MCP tool set](mcp.md#available-tools). Two tools make a good smoke test:

- `mcp__aflare__list_nodes` — discover the 20+ workflow nodes
- `mcp__aflare__run_workflow_yaml` — execute an inline workflow, e.g.:

```
Run this aflare workflow:
steps:
  - node: echo
    params:
      message: "hello from DSH"
```

### One-paste install prompt

Paste the block below into a DSH chat to let the DSH agent wire everything up
itself (same pattern popularized by other MCP integrations):

```
Install the aflare MCP server for me, step by step:
1. Run `aflare version`. If aflare is not installed, stop and tell me to
   install it first (https://github.com/alib8b8/aflare).
2. Locate the cordis.yml profile you are currently running with.
3. Add this plugin entry (one instance, stdio transport):
   - id: mcp-aflare
     name: '@deepseek-ai/dsh-mcp-client'
     config:
       serverName: aflare
       transport: stdio
       command: aflare
       args: ['mcp']
4. Reload the configuration, then verify by listing tools and calling
   mcp__aflare__list_nodes.
5. Report which aflare tools are now available.
```

### Security notes

- The MCP server inherits aflare's existing path protections: workflow files
  must stay inside the working directory and use `.yaml`/`.yml`
  (see `validateWorkflowFilePath`).
- Secrets resolved by workflow nodes stay in the aflare process; nothing is
  forwarded to DSH beyond final tool output text.
- For untrusted workflows, start aflare with safe mode (`AFLARE_SAFE_MODE=1`)
  before launching `aflare mcp` — the restriction applies to every MCP call.

## Option B: native Cordis plugin

See [`integrations/dsh-plugin/`](../integrations/dsh-plugin/) for a TypeScript
Cordis plugin that registers curated tools (`aflare_run`, `aflare_generate`,
`aflare_template_run`, ...) backed by the `aflare` CLI. Choose this when you
want the tools to appear as native DSH tools (no `mcp__` prefix, full
Trajectory integration) or plan to publish to npm.

## Which one should I use?

| | Option A (MCP) | Option B (plugin) |
|---|---|---|
| Setup effort | One YAML entry | Clone + build (or npm install once published) |
| Tool surface | All 27 MCP tools | Curated subset, tuned prompts |
| Naming | `mcp__aflare__*` | `aflare_*` |
| Maintenance | None (tracks `aflare mcp`) | Plugin repo, but survives DSH MCP changes |

Start with Option A; add Option B when you want a polished, shareable preset.
