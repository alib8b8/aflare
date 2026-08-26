# aflare Node Reference

Quick reference for the most common built-in nodes (a curated subset — run `aflare list` for the full catalog of 70 registered nodes, and see the generated `docs/nodes-reference.md` in the repository for complete parameter schemas).

## Utility Nodes

### fetch_url

Fetch content from a URL. Includes SSRF protection (blocks private IPs, validates DNS).

**Parameters:**
- `url` (required) — URL to fetch
- `mode` (optional) — `text` (default) or `raw`. `text` strips HTML to plain text; `raw` returns body as-is.

**Example:**
```yaml
- node: fetch_url
  params:
    url: "https://example.com/article"
    mode: "text"
```

### http_request

Full HTTP client. Any method, headers, body. Returns response body and status code.

**Parameters:**
- `url` (required) — Target URL
- `method` (optional, default `GET`) — HTTP method
- `headers` (optional) — Key-value map of headers. `Authorization` header is allowed.
- `body` (optional) — Request body string
- `timeout` (optional, default `30s`) — Request timeout

**Example:**
```yaml
- node: http_request
  params:
    url: "https://api.github.com/repos/alib8b8/aflare"
    method: "GET"
    headers:
      Authorization: "Bearer ghp_xxx"
      Accept: "application/vnd.github+json"
```

### file_read

Read content from a local file. Path traversal protected.

**Parameters:**
- `path` (required) — Path to the file to read (relative to connector root when `connector` is set)
- `connector` (optional) — Named connector (registered via `aflare connector add`); when set, the path resolves inside the connector's root, credentials stay out of the workflow, and connector ceilings (max_bytes etc.) apply

**Example:**
```yaml
- node: file_read
  params:
    path: "input.txt"
```

### file_write

Write content to a file. Atomic write (temp file + rename). Path traversal protected.

**Parameters:**
- `path` (required) — Path to the output file (relative to connector root when `connector` is set)
- `content` (optional) — Content to write; defaults to previous step output
- `connector` (optional) — Named connector; requires a read-write connector (read-only connectors reject writes)

**Example:**
```yaml
- node: file_write
  params:
    path: "output.md"
```

### execute

Run shell commands. Command injection protected (shell metacharacters blocked).
Optional allowlist via safe mode or config.

**Parameters:**
- `command` (required) — Command to execute
- `cwd` (optional) — Working directory
- `timeout` (optional, default `60s`) — Execution timeout

**Example:**
```yaml
- node: execute
  params:
    command: "git log --oneline -10"
    cwd: "/path/to/repo"
```

> **Note:** `execute` is disabled in `--safe-mode`. Prefer dedicated nodes when possible.

### json_parse

Extract fields from JSON using dot notation.

**Parameters:**
- `path` (required) — Dot-notation path (e.g., `result.items.[0].name`)
- `input` (optional) — JSON string; defaults to previous step output

**Example:**
```yaml
- node: json_parse
  params:
    path: "data.users.[0].email"
```

### template_render

Render Go templates with variables. SSTI-safe (dangerous functions removed).

**Parameters:**
- `template` (required) — Go template string
- `template_file` (optional) — Path to a template file (takes precedence over `template`)
- Any other param — passed to the template as a variable. Note: param values must be strings (no nested maps), and there is no `vars` param; name variables directly as sibling params of `template`.

**Example:**
```yaml
- node: template_render
  params:
    template: "Hello {{.name}}, your score is {{.score}}"
    name: "Alice"
    score: "95"
```

### transform

Transform text. Supports 14+ operations.

**Parameters:**
- `operation` (required) — One of: `uppercase`, `lowercase`, `trim`, `replace`, `regex`, `split`, `join`, `base64_encode`, `base64_decode`, `url_encode`, `url_decode`, `extract_emails`, `extract_urls`, `extract_numbers`, `markdown_to_html`
- Additional params depend on operation (e.g., `find`/`replace` for `replace`, `pattern` for `regex`)

**Examples:**
```yaml
- node: transform
  params:
    operation: uppercase

- node: transform
  params:
    operation: replace
    find: "foo"
    replace: "bar"

- node: transform
  params:
    operation: regex
    pattern: "\\d{4}-\\d{2}-\\d{2}"
```

> Regex uses `SafeRegexMatch` with 2s timeout, 512-char pattern limit, 1MB input limit.

### combine

Merge multiple inputs into one.

**Parameters:**
- `separator` (optional, default `\n`) — Separator between inputs

**Example:**
```yaml
- node: combine
  params:
    separator: "\n---\n"
```

### notify

Print or send notifications.

**Parameters:**
- `channel` (required) — `stdout` (print to terminal) or `stderr`
- `message` (required) — Notification message

**Example:**
```yaml
- node: notify
  params:
    channel: stdout
    message: "Workflow completed!"
```

## LLM Nodes (15+ providers)

All LLM nodes share common parameters:
- `prompt` (required) — Prompt text. Can reference previous step outputs via `{{step.<name>}}` (or `{{step.<N>}}`, 0-based index)
- `model` (optional) — Model name (provider-specific default if omitted)
- `temperature` (optional) — Sampling temperature
- `max_tokens` (optional) — Max tokens to generate

| Node | Provider | Default Model |
|------|----------|---------------|
| `ollama` | Local models via Ollama | llama3 |
| `deepseek` | DeepSeek API | deepseek-chat |
| `openai` | OpenAI-compatible (200+ via OpenRouter, SiliconFlow) | gpt-4o-mini |
| `qwen` | Alibaba Qwen / 通义千问 | qwen-plus |
| `glm` | Zhipu GLM / 智谱 | glm-4 |
| `kimi` | Moonshot Kimi | moonshot-v1-8k |
| `minimax` | MiniMax | abab6.5-chat |
| `mistral` | Mistral AI | mistral-large-latest |
| `yi` | 01.AI Yi / 零一万物 | yi-large |
| `baichuan` | Baichuan / 百川 | baichuan4 |
| `internlm` | InternLM / 书生 | internlm2.5-latest |
| `xverse` | XVerse / 元象 | xverse-65b |
| `mimo` | Xiaomi MiMo / 小米 | mimo-7b |
| `ima` | Tencent IMA / 腾讯 | ima-default |
| `fastgpt` | FastGPT | fastgpt-default |
| `coze` | ByteDance Coze (WIP, not functional yet) | — |

**Example:**
```yaml
- node: fetch_url
  name: fetch
  params:
    url: "https://example.com/article"
- node: ollama
  params:
    model: "llama3"
    prompt: "Summarize this article: {{step.fetch}}"
    temperature: 0.3
```

> API keys are read from environment variables (e.g., `DEEPSEEK_API_KEY`, `OPENAI_API_KEY`).
> Keys are never logged — sensitive data is redacted in audit logs.

## Control Nodes

### condition

Evaluate a condition expression against the step input (previous step output). Returns "true"/"false".

Supported expressions: `contains:<text>`, `equals:<value>`, `starts_with:<prefix>`, `ends_with:<suffix>`, `regex:<pattern>`, `empty`, `not_empty`, `true`, `false` — optionally prefixed with `not `.

**Parameters:**
- `expr` (required) — Condition expression (e.g. `contains:foo`, `regex:^test`, `not_empty`)
- `condition` (optional) — Alias for `expr`

**Example:**
```yaml
- node: condition
  params:
    expr: "contains:error"
```

Step-level conditional execution uses the same expression syntax on the `condition` field:
```yaml
- node: notify
  condition: "not_empty"
  params:
    channel: stdout
    message: "Data fetched successfully"
```

### call

Call another workflow file (nested workflows). Recursive depth tracked (max 10).

**Parameters:**
- `workflow` (required) — Path to the workflow YAML file to call
- `vars` (optional) — Variables to pass to the nested workflow, as a JSON string (`{"key":"value"}`) or comma-separated `key=value` pairs

**Example:**
```yaml
- node: call
  params:
    workflow: "sub-workflows/parse-data.yaml"
    vars: "input={{step.fetch}}"
```

## YAML Workflow Structure

```yaml
name: my-workflow
description: What this workflow does
vars:
  api_key: "your-api-key"    # workflow-level variables
steps:
  - node: fetch_url
    name: fetch
    params:
      url: "https://api.example.com/data"
  - node: json_parse
    params:
      path: "result.items.[0].name"
  - node: file_write
    params:
      path: "output.txt"
      content: "{{step.fetch}}"
  - node: notify
    condition: "not_empty"
    params:
      channel: stdout
      message: "Done!"
```

## Parallel Steps

Run multiple steps concurrently:

```yaml
steps:
  - parallel:
      - node: fetch_url
        params:
          url: "https://api1.example.com"
      - node: fetch_url
        params:
          url: "https://api2.example.com"
  - node: combine
    params:
      separator: "\n---\n"
```

## External Nodes

aflare supports external nodes installed via the registry:

```bash
# List available external nodes
aflare registry list

# Search for a node
aflare registry search weather

# Install an external node
aflare install weather-api

# Uninstall
aflare uninstall weather-api
```

Installed external nodes appear in `aflare list` and can be used like built-in nodes.
