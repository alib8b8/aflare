# Node Reference

> Auto-generated from `Schema()` metadata. 52 nodes registered.

| Node | Description | Params |
|------|-------------|--------|
| [`agent`](#agent) | Autonomous agent node with ReAct reasoning loop and tool use capabilities | 9 |
| [`anthropic`](#anthropic) | Call Anthropic LLM API | 15 |
| [`ascend`](#ascend) | Call Ascend LLM API | 15 |
| [`baichuan`](#baichuan) | Call Baichuan LLM API | 15 |
| [`call`](#call) | Call another workflow file | 2 |
| [`cambricon`](#cambricon) | Call Cambricon MLU LLM API | 15 |
| [`cli_session`](#cli_session) | 交互式CLI会话节点。支持上下文保持、命令历史、快捷键、流式输出和自动补全，提供类... | 5 |
| [`code_interpreter`](#code_interpreter) | Execute Python/Node.js/Rust code in a sandboxed environment with file I/O | 6 |
| [`code_review`](#code_review) | Hybrid code review combining deterministic rule engine (NPE, thread-safety, security) with LLM deep analysis. Inspire... | 11 |
| [`combine`](#combine) | Combine multiple inputs into one | 1 |
| [`condition`](#condition) | Evaluate conditional expressions (contains, equals, regex, empty) | 2 |
| [`coze`](#coze) | Call Coze LLM API | 15 |
| [`critic`](#critic) | Critic agent that reviews output, identifies issues, and suggests improvements | 8 |
| [`deepseek`](#deepseek) | Call DeepSeek LLM API | 15 |
| [`drone`](#drone) | Control MAVLink-compatible drones (PX4/ArduPilot) via HTTP bridge. Supports arm/disarm, takeoff, land, RTL, waypoint ... | 17 |
| [`evaluator`](#evaluator) | Evaluator agent that scores output against criteria with structured rubrics | 9 |
| [`execute`](#execute) | Execute shell commands (disabled in safe mode) | 3 |
| [`fastgpt`](#fastgpt) | Call FastGPT API | 5 |
| [`fetch_url`](#fetch_url) | Fetch content from a URL | 3 |
| [`file_read`](#file_read) | Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set... | 2 |
| [`file_watch`](#file_watch) | Polls a file or directory for create/modify/delete events and returns them as JSON. Suitable for log-monitor and file... | 6 |
| [`file_write`](#file_write) | Write content to a file | 2 |
| [`gemini`](#gemini) | Call Google Gemini LLM API | 15 |
| [`glm`](#glm) | Call GLM LLM API | 15 |
| [`http_request`](#http_request) | Make HTTP requests with custom method, headers, and body | 12 |
| [`human_in_loop`](#human_in_loop) | Human approval gate — pauses workflow for human review and approval before continuing | 5 |
| [`hygon`](#hygon) | Call Hygon DCU LLM API | 15 |
| [`ima`](#ima) | Call IMA Copilot LLM API | 15 |
| [`internlm`](#internlm) | Call InternLM LLM API | 15 |
| [`json_parse`](#json_parse) | Parse and extract JSON data | 1 |
| [`kimi`](#kimi) | Call Kimi LLM API | 15 |
| [`llm_router`](#llm_router) | Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization | 5 |
| [`memory`](#memory) | AI Agent memory infrastructure with session-isolated persistent knowledge graph engine. Supports multi-session parall... | 16 |
| [`mimo`](#mimo) | Call MiMo LLM API | 15 |
| [`minimax`](#minimax) | Call MiniMax LLM API | 15 |
| [`mistral`](#mistral) | Call Mistral LLM API | 15 |
| [`notify`](#notify) | Send notifications (stdout, stderr, slack, discord, telegram, webhook) | 10 |
| [`ollama`](#ollama) | Call Ollama local LLM server | 3 |
| [`openai`](#openai) | Call OpenAI LLM API | 15 |
| [`pipeline`](#pipeline) | Dependency-based parallel workflow executor: steps run as soon as their dependencies are met, no global barriers (Tun... | 2 |
| [`planner`](#planner) | Task decomposition agent that breaks complex goals into actionable steps | 9 |
| [`qwen`](#qwen) | Call Qwen LLM API | 15 |
| [`reflector`](#reflector) | Self-reflection agent that critiques output and iteratively improves it (Reflexion pattern) | 7 |
| [`researcher`](#researcher) | Research agent that fetches information from URLs and summarizes findings | 7 |
| [`search_aggregate`](#search_aggregate) | Multi-platform search aggregator with real-signal ranking: Reddit/Twitter/YouTube/HN/GitHub, sorted by votes/comments... | 8 |
| [`session_manager`](#session_manager) | Multi-session memory management. Create isolated sessions, fork a session from a parent, merge sessions, and share fa... | 11 |
| [`structured_output`](#structured_output) | LLM-driven structured output with local JSON Schema validation and self-correction retries | 11 |
| [`supervisor`](#supervisor) | Advanced supervisor with MoE routing, MindSearch deep research, 232+ domain specialists, and collaboration templates | 13 |
| [`template_render`](#template_render) | Render Go templates with input data | 2 |
| [`transform`](#transform) | Transform text using string operations | 1 |
| [`xverse`](#xverse) | Call XVERSE LLM API | 15 |
| [`yi`](#yi) | Call Yi LLM API | 15 |

---

## agent

Autonomous agent node with ReAct reasoning loop and tool use capabilities

- **Input**: string - the task or question for the agent
- **Output**: string - the agent's final answer

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider: ollama, openai, deepseek, glm, kimi, qwen, mistral, yi (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key (for cloud providers) |
| `endpoint` | string | No |  | API endpoint URL |
| `system` | string | No |  | System prompt / role definition for the agent |
| `tools` | string | No | fetch_url,json_parse | Comma-separated list of tools to enable: fetch_url,http_request,file_read,file_write,json_parse,transform,combine,ollama,openai,code_interpreter,execute |
| `max_iterations` | string | No | 10 | Maximum number of ReAct iterations (default: 10) |
| `enable_thinking` | string | No | false | Enable deep thinking / chain-of-thought mode (default: false) |
| `show_thinking` | string | No | true | Show the thinking chain in output (default: true) |

---

## anthropic

Call Anthropic LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | claude-3-5-sonnet-latest | Model name (default: claude-3-5-sonnet-latest) |
| `api_key` | string | No |  | Anthropic API key (or set ANTHROPIC_API_KEY env var) |
| `endpoint` | string | No | https://api.anthropic.com/v1 | API base URL (default: https://api.anthropic.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## ascend

Call Ascend LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | qwen2.5-7b | Model name (default: qwen2.5-7b) |
| `api_key` | string | No |  | Ascend API key (or set ASCEND_API_KEY env var) |
| `endpoint` | string | No | http://localhost:8080/v1 | API base URL (default: http://localhost:8080/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## baichuan

Call Baichuan LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | Baichuan4 | Model name (default: Baichuan4) |
| `api_key` | string | No |  | Baichuan API key (or set BAICHUAN_API_KEY env var) |
| `endpoint` | string | No | https://api.baichuan-ai.com/v1 | API base URL (default: https://api.baichuan-ai.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## call

Call another workflow file

- **Input**: string - input data to pass to the called workflow
- **Output**: string - output from the called workflow

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `workflow` | string | Yes |  | Path to the workflow file to call |
| `vars` | string | No |  | JSON or key=value pairs to pass as workflow variables (e.g. topic=AI,model=gpt-4) |

---

## cambricon

Call Cambricon MLU LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | qwen2.5-7b | Model name (default: qwen2.5-7b) |
| `api_key` | string | No |  | Cambricon MLU API key (or set CAMBRICON_API_KEY env var) |
| `endpoint` | string | No | http://localhost:8081/v1 | API base URL (default: http://localhost:8081/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## cli_session

交互式CLI会话节点。支持上下文保持、命令历史、快捷键、流式输出和自动补全，提供类似Claude Code的流畅CLI体验。

- **Input**: string - 用户输入的命令或消息
- **Output**: string - JSON格式的会话响应

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | auto | 使用的模型（默认auto，由路由层选择） |
| `session_id` | string | No |  | 会话ID（自动生成或指定） |
| `max_history` | int | No | 50 | 最大历史记录数（默认50） |
| `streaming` | bool | No | true | 流式输出（默认true） |
| `theme` | string | No | dark | 主题（light/dark，默认dark） |

---

## code_interpreter

Execute Python/Node.js/Rust code in a sandboxed environment with file I/O

- **Input**: string - stdin for the code (optional)
- **Output**: string - stdout, stderr, and generated files

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `code` | string | Yes |  | Code to execute |
| `language` | string | No | python | Programming language: python, nodejs, rust (default: python) |
| `timeout` | string | No |  | Execution timeout (default based on security level) |
| `work_dir` | string | No |  | Working directory for code execution (default: temp dir) |
| `save_outputs` | string | No | true | If true, save output files to work_dir (default: true) |
| `network` | string | No | false | Allow network access during execution (L0/L1 only, default: false) |

---

## code_review

Hybrid code review combining deterministic rule engine (NPE, thread-safety, security) with LLM deep analysis. Inspired by alibaba/open-code-review.

- **Input**: string - the code to review
- **Output**: string - structured code review with findings and suggestions

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `language` | string | No |  | Programming language (auto-detected if not specified) |
| `focus` | string | No | all | Review focus: all, bugs, security, style, performance (default: all) |
| `severity` | string | No | medium | Minimum severity: low, medium, high, critical (default: medium) |
| `use_rules` | string | No | true | Run deterministic rule engine before LLM (default: true) |
| `use_llm` | string | No | true | Run LLM deep analysis (default: true) |
| `auto_clarify` | string | No | false | Run ACQUIRE-style clarification before review (default: false) |
| `clarify_threshold` | string | No | 70 | Confidence threshold for auto-clarification 0-100 (default: 70) |

---

## combine

Combine multiple inputs into one

- **Input**: string - input text to format
- **Output**: string - formatted output

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `format` | string | No | text | Output format: text, markdown, csv, json (default: text) |

---

## condition

Evaluate conditional expressions (contains, equals, regex, empty)

- **Input**: string - the text to evaluate against
- **Output**: string - 'true' or 'false'

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `expr` | string | Yes |  | Condition expression (e.g. contains:foo, equals:bar, regex:^test, empty, not_empty) |
| `condition` | string | No |  | Alias for expr |

---

## coze

Call Coze LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No |  | Model name (default: ) |
| `api_key` | string | No |  | Coze API key (or set COZE_API_KEY env var) |
| `endpoint` | string | No | https://api.coze.cn/v1 | API base URL (default: https://api.coze.cn/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## critic

Critic agent that reviews output, identifies issues, and suggests improvements

- **Input**: string - the content to be reviewed and critiqued
- **Output**: string - structured critique with issues and improvement suggestions

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `role` | string | No | general | Critic role: general, code, writing, security, design (default: general) |
| `criteria` | string | No |  | Custom evaluation criteria (comma-separated) |
| `output_format` | string | No | markdown | Output format: markdown, json, bullet_points (default: markdown) |
| `suggest_improvements` | string | No | true | Whether to suggest improvements: true/false (default: true) |

---

## deepseek

Call DeepSeek LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | deepseek-chat | Model name (default: deepseek-chat) |
| `api_key` | string | No |  | DeepSeek API key (or set DEEPSEEK_API_KEY env var) |
| `endpoint` | string | No | https://api.deepseek.com/v1 | API base URL (default: https://api.deepseek.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## drone

Control MAVLink-compatible drones (PX4/ArduPilot) via HTTP bridge. Supports arm/disarm, takeoff, land, RTL, waypoint mission upload, telemetry polling, patrol, survey, orbit, follow-me, and camera capture. Requires a MAVSDK server (drone_bridge.py) running on the drone's companion computer or ground station.

- **Input**: string - natural language instruction or command description
- **Output**: string - JSON drone control result with telemetry and status

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `drone_model` | string | No | PX4 | Drone firmware: PX4, ArduPilot, ArduCopter, ArduPlane, ArduRover, generic (default: PX4) |
| `drone_id` | string | No |  | Unique drone identifier (default: auto-generated) |
| `bridge_host` | string | No | 127.0.0.1 | MAVSDK bridge host IP (default: 127.0.0.1) |
| `bridge_port` | string | No | 8080 | MAVSDK bridge port (default: 50051 for gRPC, 8080 for HTTP) |
| `bridge_token` | string | No |  | Bearer token for bridge authentication (default: none) |
| `action` | string | No |  | Action: arm, disarm, takeoff, land, rtl, hold, goto, mission_start, mission_pause, mission_resume, mission_upload, mission_clear, set_mode, get_telemetry, get_status, get_gps, get_battery, camera, deliver, patrol, survey, orbit, follow |
| `mode` | string | No | simulate | Backend mode: simulate (default) | mavsdk | http |
| `target_altitude_m` | float | No | 10 | Target altitude in meters (default: 10) |
| `target_latitude` | float | No |  | Target latitude for goto/mission waypoints |
| `target_longitude` | float | No |  | Target longitude for goto/mission waypoints |
| `mission_type` | string | No | waypoint | Mission type: waypoint, survey, corridor, orbit, patrol (default: waypoint) |
| `waypoints` | string | No |  | JSON array of waypoints [{lat, lon, alt}] for mission_upload |
| `flight_speed_ms` | float | No | 5 | Flight speed in m/s (default: 5) |
| `safety_altitude_m` | float | No | 3 | Minimum safety altitude in meters (default: 3) |
| `max_flight_time_s` | string | No | 300 | Maximum flight time in seconds (default: 300) |
| `geofence_radius_m` | float | No | 200 | Geofence radius in meters (default: 200) |
| `timeout` | string | No | 15 | HTTP request timeout in seconds (default: 15) |

---

## evaluator

Evaluator agent that scores output against criteria with structured rubrics

- **Input**: string - the content to evaluate
- **Output**: string - evaluation scores and justification

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `rubric` | string | No | quality | Evaluation rubric: quality, correctness, completeness, clarity, custom (default: quality) |
| `criteria` | string | No |  | Custom criteria for evaluation (comma-separated dimensions) |
| `scale` | string | No | 1-10 | Score scale: 1-5, 1-10, percentage (default: 1-10) |
| `threshold` | string | No |  | Pass/fail threshold score (e.g. 7) |
| `output_format` | string | No | json | Output format: json, markdown, score_only (default: json) |

---

## execute

Execute shell commands (disabled in safe mode)

- **Input**: string - stdin for the command
- **Output**: string - stdout and stderr of the command

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `command` | string | Yes |  | Shell command to execute |
| `dry_run` | string | No | false | If true, preview the command without executing (default: false) |
| `timeout` | string | No | 5m | Command timeout, e.g. 30s, 5m, 1h (default: 5m) |

---

## fastgpt

Call FastGPT API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `api_key` | string | No |  | FastGPT API key (or set FASTGPT_API_KEY env var) |
| `app_id` | string | No |  | FastGPT app ID |
| `chat_id` | string | No |  | Chat ID for conversation continuity |
| `endpoint` | string | No | https://fastgpt.in/api/v1 | API base URL (or set FASTGPT_BASE_URL env var) |
| `system` | string | No |  | System prompt |

---

## fetch_url

Fetch content from a URL

- **Input**: string - optional URL (overrides url param)
- **Output**: string - content of the URL

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `url` | string | No |  | URL to fetch |
| `mode` | string | No | text | Extraction mode: text, markdown, html, main_content |
| `timeout` | int | No | 30 | Request timeout in seconds |

---

## file_read

Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set redact=false to disable.

- **Input**: string - not used
- **Output**: string - file content (with secrets redacted by default)

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | File path to read from |
| `redact` | string | No | true | Redact secrets in output: true (default) / false. When true, .env and credential files are fully masked; other files have known secret patterns masked. |

---

## file_watch

Polls a file or directory for create/modify/delete events and returns them as JSON. Suitable for log-monitor and file-organizer workflows.

- **Input**: string - not used
- **Output**: string - JSON with watched_path, duration, events_collected, and events array

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | 要监控的文件或目录路径 |
| `duration` | string | No | 30s | 监控持续时间（如 30s, 5m） |
| `interval` | string | No | 1s | 轮询间隔（如 1s, 500ms） |
| `events` | string | No | create,modify,delete | 关注的事件类型，逗号分隔：create,modify,delete |
| `pattern` | string | No | * | 文件名 glob 匹配模式 |
| `max_events` | string | No | 1000 | 最大收集事件数（防 DoS） |

---

## file_write

Write content to a file

- **Input**: string - content to write to the file
- **Output**: string - confirmation message

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | File path to write to |
| `mode` | string | No | write | Write mode: write (default) or append |

---

## gemini

Call Google Gemini LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | gemini-2.0-flash | Model name (default: gemini-2.0-flash) |
| `api_key` | string | No |  | Google Gemini API key (or set GEMINI_API_KEY env var) |
| `endpoint` | string | No | https://generativelanguage.googleapis.com/v1beta/openai | API base URL (default: https://generativelanguage.googleapis.com/v1beta/openai) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## glm

Call GLM LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | glm-4 | Model name (default: glm-4) |
| `api_key` | string | No |  | GLM API key (or set GLM_API_KEY env var) |
| `endpoint` | string | No | https://open.bigmodel.cn/api/paas/v4 | API base URL (default: https://open.bigmodel.cn/api/paas/v4) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## http_request

Make HTTP requests with custom method, headers, and body

- **Input**: string - request body (overrides body param)
- **Output**: string - response body

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `url` | string | Yes |  | Target URL |
| `method` | string | No | GET | HTTP method: GET, POST, PUT, DELETE, PATCH |
| `headers` | string | No |  | JSON-encoded headers |
| `body` | string | No |  | Request body |
| `timeout` | int | No | 30 | Request timeout in seconds |
| `rate_limit_rps` | float | No | 0 | Max requests per second per host (0=unlimited) |
| `rate_limit_burst` | int | No |  | Token-bucket burst size (default=ceil(rate_limit_rps)) |
| `rate_limit_key` | string | No |  | Explicit bucket key overriding URL.Host; set when multiple domain aliases resolve to the same backend so they share one bucket (M-9) |
| `max_retries` | int | No | 0 | Max retry attempts on transient failures (default 0=no retry) |
| `retry_backoff_ms` | int | No | 100 | Initial retry backoff in ms (default 100) |
| `retry_max_backoff_ms` | int | No | 5000 | Max retry backoff cap in ms (default 5000) |
| `retry_on_status` | string | No | 429,500,502,503,504 | Comma-separated retryable status codes (default 429,500,502,503,504) |

---

## human_in_loop

Human approval gate — pauses workflow for human review and approval before continuing

- **Input**: string - the content/data to present for human review
- **Output**: string - approved content (or original if approved)

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `mode` | string | No | file | Approval mode: file, env, stdin, auto_approve (default: file) |
| `approval_file` | string | No | .aflare-approval | Path to approval flag file (mode=file) |
| `approval_env` | string | No | AFLARE_APPROVED | Environment variable to check for approval (mode=env) |
| `prompt` | string | No |  | Custom prompt message for the human reviewer |
| `on_approve` | string | No | original | What to output on approve: original, modified, passthrough (default: original) |

---

## hygon

Call Hygon DCU LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | qwen2.5-7b | Model name (default: qwen2.5-7b) |
| `api_key` | string | No |  | Hygon DCU API key (or set HYGON_API_KEY env var) |
| `endpoint` | string | No | http://localhost:8082/v1 | API base URL (default: http://localhost:8082/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## ima

Call IMA Copilot LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No |  | Model name (default: ) |
| `api_key` | string | No |  | IMA Copilot API key (or set IMA_API_KEY env var) |
| `endpoint` | string | No |  | API base URL (default: ) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## internlm

Call InternLM LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | internlm3-latest | Model name (default: internlm3-latest) |
| `api_key` | string | No |  | InternLM API key (or set INTERNLM_API_KEY env var) |
| `endpoint` | string | No | https://internlm-chat.intern-ai.org.cn/api/v1 | API base URL (default: https://internlm-chat.intern-ai.org.cn/api/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## json_parse

Parse and extract JSON data

- **Input**: string - JSON string to parse
- **Output**: string - extracted value or pretty-printed JSON

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | No |  | Dot-notation path to extract (e.g. data.items.[0].name). If omitted, pretty-prints entire JSON. |

---

## kimi

Call Kimi LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | kimi-k3 | Model name (default: kimi-k3) |
| `api_key` | string | No |  | Kimi API key (or set KIMI_API_KEY env var) |
| `endpoint` | string | No | https://api.moonshot.cn/v1 | API base URL (default: https://api.moonshot.cn/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## llm_router

Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization

- **Input**: string - user message content to send to LLM
- **Output**: string - AI response from the selected provider

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `system` | string | No |  | System prompt for the LLM |
| `strategy` | string | No | priority | Routing strategy: priority, cost, latency, pareto, round_robin, random (default: priority) |
| `max_retries` | string | No | 3 | Maximum number of fallback attempts (default: 3) |
| `show_provider` | string | No | false | Show which provider was used in output (default: false) |
| `show_stats` | string | No | false | Show router statistics in output (default: false) |

---

## memory

AI Agent memory infrastructure with session-isolated persistent knowledge graph engine. Supports multi-session parallel memory, short/medium/long term memory, cross-session long-term memory, and memory usage monitoring.

- **Input**: string - memory content to store or query for retrieval
- **Output**: string - JSON with memory operations result, entries, or statistics

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | store | Operation: store/retrieve/delete/search/summary/forget/transfer/merge/inkling_retrieve/list_sessions/session_stats/global_stats/link_kg/expand_kg/compress (default: store) |
| `session_id` | string | No | default | Session ID for isolated memory (default: default) |
| `key` | string | No |  | Memory key for storage/retrieval/link_kg |
| `value` | string | No |  | Memory value/content |
| `level` | string | No | medium | Memory level: short/medium/long (default: medium) |
| `type` | string | No | fact | Memory type: fact/concept/experience/preference/relationship/task/context (default: fact) |
| `tags` | string | No |  | Comma-separated tags for categorization |
| `ttl_hours` | int | No | 72 | Time to live in hours (default: 72) |
| `confidence` | float | No | 0.8 | Confidence level 0.0-1.0 (default: 0.8) |
| `query` | string | No |  | Search query for retrieval/search/expand_kg operations |
| `top_k` | int | No | 10 | Number of results to return (1-100, default: 10) |
| `threshold` | float | No | 0.5 | Similarity threshold 0.0-1.0 (default: 0.5) |
| `source` | string | No |  | Source identifier for the memory |
| `kg_entities` | string | No |  | link_kg: comma-separated KG entity names to link to the memory key |
| `token_budget` | int | No | 2000 | compress: max tokens to retain after compression (default: 2000) |
| `min_confidence` | float | No | 0.5 | compress: entries below this confidence are candidates for compression (default: 0.5) |

---

## mimo

Call MiMo LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | mimo-v2.5-pro | Model name (default: mimo-v2.5-pro) |
| `api_key` | string | No |  | MiMo API key (or set MIMO_API_KEY env var) |
| `endpoint` | string | No | https://api.xiaomimimo.com/v1 | API base URL (default: https://api.xiaomimimo.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## minimax

Call MiniMax LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | abab6.5s-chat | Model name (default: abab6.5s-chat) |
| `api_key` | string | No |  | MiniMax API key (or set MINIMAX_API_KEY env var) |
| `endpoint` | string | No | https://api.minimax.chat/v1 | API base URL (default: https://api.minimax.chat/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## mistral

Call Mistral LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | mistral-large-latest | Model name (default: mistral-large-latest) |
| `api_key` | string | No |  | Mistral API key (or set MISTRAL_API_KEY env var) |
| `endpoint` | string | No | https://api.mistral.ai/v1 | API base URL (default: https://api.mistral.ai/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## notify

Send notifications (stdout, stderr, slack, discord, telegram, webhook)

- **Input**: string - message to notify (used if message param is empty)
- **Output**: string - the notification message

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `channel` | string | No | stdout | Notification channel: stdout, stderr, slack, discord, telegram, webhook (default: stdout) |
| `message` | string | No |  | Notification message (overrides input) |
| `url` | string | No |  | Webhook URL for slack/discord/webhook, or Telegram API base (required for external channels) |
| `webhook_url` | string | No |  | Deprecated: use url instead |
| `token` | string | No |  | Bot token (required when channel=telegram) |
| `chat_id` | string | No |  | Telegram chat ID (required when channel=telegram) |
| `username` | string | No |  | Discord webhook username (optional) |
| `method` | string | No | POST | HTTP method for webhook: GET/POST/PUT (default: POST) |
| `headers` | string | No |  | JSON headers for webhook (optional) |
| `body` | string | No |  | Custom body for webhook (optional) |

---

## ollama

Call Ollama local LLM server

- **Input**: string - user prompt content (used when prompt param is not provided)
- **Output**: string - model response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | llama3 | Model name (default: llama3) |
| `endpoint` | string | No | http://localhost:11434 | Ollama server URL (default: http://localhost:11434) |
| `prompt` | string | No |  | Prompt to send to Ollama (if not provided, uses input) |

---

## openai

Call OpenAI LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | gpt-3.5-turbo | Model name (default: gpt-3.5-turbo) |
| `api_key` | string | No |  | OpenAI API key (or set OPENAI_API_KEY env var) |
| `endpoint` | string | No | https://api.openai.com/v1 | API base URL (default: https://api.openai.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## pipeline

Dependency-based parallel workflow executor: steps run as soon as their dependencies are met, no global barriers (Tunix-inspired async rollout)

- **Input**: string - YAML or JSON pipeline configuration with steps and dependencies
- **Output**: string - JSON with execution results, timings, and errors

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `timeout` | string | No | 300 | Timeout in seconds (default: 300) |
| `format` | string | No | auto | Input format: json|yaml|auto (default: auto) |

---

## planner

Task decomposition agent that breaks complex goals into actionable steps

- **Input**: string - the complex task or goal to plan for
- **Output**: string - JSON array of planned steps with descriptions and dependencies

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `max_steps` | string | No | 10 | Maximum number of steps to plan (default: 10) |
| `context` | string | No |  | Additional context or constraints for the plan |
| `auto_clarify` | string | No | false | Run ACQUIRE-style clarification before planning (default: false). If task is ambiguous, ask clarifying questions first. |
| `clarify_threshold` | string | No | 70 | Confidence threshold for auto-clarification 0-100 (default: 70) |
| `clarify_max_questions` | string | No | 3 | Max clarification questions (default: 3) |

---

## qwen

Call Qwen LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | qwen-turbo | Model name (default: qwen-turbo) |
| `api_key` | string | No |  | Qwen API key (or set QWEN_API_KEY env var) |
| `endpoint` | string | No | https://dashscope.aliyuncs.com/compatible-mode/v1 | API base URL (default: https://dashscope.aliyuncs.com/compatible-mode/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## reflector

Self-reflection agent that critiques output and iteratively improves it (Reflexion pattern)

- **Input**: string - the initial output to reflect on and improve
- **Output**: string - the improved final output

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `iterations` | string | No | 2 | Number of reflection iterations (1-5, default: 2) |
| `goal` | string | No |  | The original goal/task the output was trying to achieve |
| `reflection_focus` | string | No | all | What to reflect on: accuracy, completeness, quality, all (default: all) |

---

## researcher

Research agent that fetches information from URLs and summarizes findings

- **Input**: string - the research topic or question
- **Output**: string - structured research summary with sources

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `urls` | string | No |  | Comma-separated URLs to research (if not provided, agent will use input topic) |
| `depth` | string | No | basic | Research depth: basic, detailed, comprehensive (default: basic) |
| `output_format` | string | No | markdown | Output format: markdown, json, summary (default: markdown) |

---

## search_aggregate

Multi-platform search aggregator with real-signal ranking: Reddit/Twitter/YouTube/HN/GitHub, sorted by votes/comments/shares instead of editorial SEO (last30days-skill inspired)

- **Input**: string - search query
- **Output**: string - JSON or formatted ranked results with signal data

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sources` | string | No | reddit,hn,github,news | Comma-separated sources: reddit,twitter,youtube,hn,github,google,weibo,zhihu,bilibili,linkedin,news,finance,academic,shopping,geopolitical,infrastructure,globalevents,energy,supplychain (default: reddit,hn,github,news) |
| `region` | string | No | global | Region filter: global,us,eu,asia,cn,mena (default: global) |
| `category` | string | No | all | Category filter: politics,economy,technology,military,energy,health,all (default: all) |
| `limit` | string | No | 10 | Max results per source (default: 10) |
| `time_range` | string | No | week | Time range: day|week|month|year|all (default: week) |
| `sort_by` | string | No | signal | signal|relevance|time (default: signal) |
| `min_score` | string | No | 0 | Minimum combined signal score filter (default: 0) |
| `output` | string | No | markdown | json|markdown|text (default: markdown) |

---

## session_manager

Multi-session memory management. Create isolated sessions, fork a session from a parent, merge sessions, and share facts across sessions via the shared namespace.

- **Input**: string - value to store (for shared_put action)
- **Output**: string - JSON result of the action

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | Yes | list | create | switch | list | delete | merge | shared_get | shared_put | recall | search |
| `session_id` | string | No |  | Target session id (required for most actions) |
| `parent` | string | No |  | Parent session id to inherit memory from (create action only) |
| `src` | string | No |  | Source session id (merge action only) |
| `dst` | string | No |  | Destination session id (merge action only) |
| `key` | string | No |  | Memory key (shared_get/shared_put/recall actions) |
| `value` | string | No |  | Memory value (shared_put action). Overrides input when set. |
| `level` | string | No | short | Memory level: short|medium|long (default short) |
| `type` | string | No | fact | Memory type tag (default fact) |
| `query` | string | No |  | Search query (search action) |
| `top_k` | string | No | 10 | Max search results (default 10) |

---

## structured_output

LLM-driven structured output with local JSON Schema validation and self-correction retries

- **Input**: string - user instruction describing what to produce
- **Output**: string - JSON string validated against schema

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `schema` | string | Yes |  | JSON Schema (draft-07 subset) the output must conform to. Must be a JSON object with "type":"object" at the root. |
| `schema_name` | string | No | output | Optional schema name shown to the model in the prompt (default: "output") |
| `provider` | string | No | openai | Provider name for default model/endpoint resolution (default: openai) |
| `model` | string | No |  | Model name (default: provider default) |
| `api_key` | string | No |  | LLM API key (or set <PROVIDER>_API_KEY env var) |
| `endpoint` | string | No |  | LLM API base URL (default: provider default endpoint) |
| `system` | string | No |  | Additional system prompt prepended to the JSON instruction |
| `temperature` | string | No | 0 | Sampling temperature 0.0-2.0 (default: 0.0 for deterministic output) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `max_retries` | string | No | 2 | Max self-correction retries on parse/validation failure (default: 2) |
| `format_output` | string | No | false | If "true", pretty-print the JSON output (default: false, compact) |

---

## supervisor

Advanced supervisor with MoE routing, MindSearch deep research, 232+ domain specialists, and collaboration templates

- **Input**: string - the overall goal or task to supervise
- **Output**: string - structured task plan with delegation and synthesis

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key for cloud providers |
| `endpoint` | string | No |  | API endpoint URL |
| `specialists` | string | No | planner,researcher,critic,evaluator | Comma-separated list of specialist agents: planner,researcher,critic,code_review,evaluator,reflector,legal_expert,medical_expert,educational_expert,financial_expert,creative_writer,data_analyst |
| `strategy` | string | No | sequential | Strategy: sequential, parallel, hierarchical, mindsearch, moe, agency, swarm (default: sequential) |
| `output_format` | string | No | json | Output format: json, markdown, summary (default: json) |
| `domain` | string | No | general | Domain specialization: general,legal,medical,education,finance,creative,tech,business (default: general) |
| `enable_moe` | string | No | false | Enable Mixture-of-Experts routing (default: false) |
| `max_depth` | string | No | 3 | Max decomposition depth for hierarchical/mindsearch (default: 3) |
| `subagent_prompts` | string | No | true | Inject per-specialist subagent prompt templates into the supervisor context (default: true). Borrows Grok Build's main/subagent prompt hierarchy. |
| `collaboration_template` | string | No |  | Collaboration template: software_development, product_design, data_science, marketing, research, legal_compliance, healthcare, education, finance, game_development, video_production, security_operations, cloud_infrastructure, content_creation, community_management, startup_acceleration, ai_development, design_system, event_management, translation_localization |
| `template_role` | string | No | team | Template role to use: team, workflow, review_cycle (default: team) |

---

## template_render

Render Go templates with input data

- **Input**: string - input data available as .input in template
- **Output**: string - rendered template output

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `template` | string | No |  | Inline template string |
| `template_file` | string | No |  | Path to template file (takes precedence over template) |

---

## transform

Transform text using string operations

- **Input**: string - text to transform
- **Output**: string - transformed text

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No |  | Transformation operation |

---

## xverse

Call XVERSE LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | XVERSE-7B-Chat | Model name (default: XVERSE-7B-Chat) |
| `api_key` | string | No |  | XVERSE API key (or set XVERSE_API_KEY env var) |
| `endpoint` | string | No | https://api.xverse.cn/v1 | API base URL (default: https://api.xverse.cn/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

## yi

Call Yi LLM API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | yi-lightning | Model name (default: yi-lightning) |
| `api_key` | string | No |  | Yi API key (or set YI_API_KEY env var) |
| `endpoint` | string | No | https://api.lingyiwanwu.com/v1 | API base URL (default: https://api.lingyiwanwu.com/v1) |
| `system` | string | No |  | System prompt |
| `temperature` | string | No |  | Sampling temperature 0.0-2.0 (default: provider default) |
| `max_tokens` | string | No |  | Max tokens to generate |
| `top_p` | string | No |  | Nucleus sampling probability mass 0.0-1.0 |
| `frequency_penalty` | string | No |  | Penalty for repeated tokens -2.0 to 2.0 |
| `presence_penalty` | string | No |  | Penalty for new tokens -2.0 to 2.0 |
| `stop` | string | No |  | Stop sequences (comma-separated, e.g. '\n,END') |
| `seed` | string | No |  | Random seed for deterministic sampling (int) |
| `response_format` | string | No |  | Structured output: 'json_object' or 'json_schema:<schema_json>' |
| `tools` | string | No |  | JSON array of tool definitions for function calling |
| `tool_choice` | string | No |  | Tool selection: 'none', 'auto', or JSON object |
| `user` | string | No |  | End-user identifier for provider-side abuse monitoring |

---

