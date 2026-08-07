# Node Reference

> Auto-generated from `Schema()` metadata. 94 nodes registered.

| Node | Description | Params |
|------|-------------|--------|
| [`agent`](#agent) | Autonomous agent node with ReAct reasoning loop and tool use capabilities | 9 |
| [`agent_browser`](#agent_browser) | Agent-optimized web browser for autonomous web navigation, content extraction, and research. Inspired by CitroLabs/eg... | 10 |
| [`agent_inbox`](#agent_inbox) | Query agent message inbox. Retrieve and manage cross-domain messages. | 4 |
| [`agent_message`](#agent_message) | Send cross-domain message to another agent by DID. Inspired by awiki.ai agent-native messaging. | 5 |
| [`andesgpt`](#andesgpt) | OPPO AndesGPT large model integration. Supports Tiny (端侧1B), Turbo (端云协同7B), Titan (云端100B+) sizes wi... | 9 |
| [`anthropic`](#anthropic) | Call Anthropic LLM API | 15 |
| [`antling`](#antling) | 蚂蚁百灵（Ant Ling）大模型集成。支持Ling-2.6通用系列、Ring-2.6推理系列、Ming全模态系列�... | 8 |
| [`app_launch`](#app_launch) | Launch a mobile/desktop app with optional parameters. Cross-platform app control for AI systems. Supports Android, iO... | 7 |
| [`baichuan`](#baichuan) | Call Baichuan LLM API | 15 |
| [`blockchain_audit`](#blockchain_audit) | Record workflow execution on blockchain for tamper-proof audit trails. Supports Ethereum, Hyperledger Fabric, and sim... | 7 |
| [`call`](#call) | Call another workflow file | 2 |
| [`clarify`](#clarify) | Pre-execution ambiguity checker: identifies unclear requirements and generates clarifying questions (ACQUIRE framework) | 8 |
| [`cli_session`](#cli_session) | 交互式CLI会话节点。支持上下文保持、命令历史、快捷键、流式输出和自动补全，提供类... | 5 |
| [`code_interpreter`](#code_interpreter) | Execute Python/Node.js/Rust code in a sandboxed environment with file I/O | 6 |
| [`code_knowledge_graph`](#code_knowledge_graph) | Semantic code knowledge graph with vector retrieval, 158 language support, MCP tool exposure, and token-efficient rev... | 13 |
| [`code_review`](#code_review) | Hybrid code review combining deterministic rule engine (NPE, thread-safety, security) with LLM deep analysis. Inspire... | 11 |
| [`combine`](#combine) | Combine multiple inputs into one | 1 |
| [`compress`](#compress) | Intelligent context compression with 6 algorithms: extractive, keyword, cluster, sliding_window, hybrid (headroom-ins... | 7 |
| [`condition`](#condition) | Evaluate conditional expressions (contains, equals, regex, empty) | 2 |
| [`context_fs`](#context_fs) | Unified context filesystem (OpenViking-inspired): ls/cat/write/rm/search over memory, profile, knowledge graph, and s... | 6 |
| [`coze`](#coze) | Call Coze LLM API | 15 |
| [`critic`](#critic) | Critic agent that reviews output, identifies issues, and suggests improvements | 8 |
| [`cross_app_action`](#cross_app_action) | Execute actions across multiple apps. Multi-app workflows for AI assistants. | 3 |
| [`deepseek`](#deepseek) | Call DeepSeek LLM API | 15 |
| [`device_state`](#device_state) | Query device state: battery, network, location, apps, storage. Context awareness for AI assistants. | 1 |
| [`doc_gen`](#doc_gen) | AI自动文档生成节点。自动生成和更新代码库文档，支持README、API文档、函数注释、模块�... | 6 |
| [`doc_parse`](#doc_parse) | Parse documents (PDF/images/HTML) into text, LaTeX, or HTML table format | 7 |
| [`engineer_skills`](#engineer_skills) | Pre-built engineering skill package with TypeScript/React/Node.js expertise. Supports skill matching, application, an... | 5 |
| [`evaluator`](#evaluator) | Evaluator agent that scores output against criteria with structured rubrics | 9 |
| [`execute`](#execute) | Execute shell commands (disabled in safe mode) | 3 |
| [`fastgpt`](#fastgpt) | Call FastGPT API | 5 |
| [`fetch_url`](#fetch_url) | Fetch content from a URL | 3 |
| [`file_read`](#file_read) | Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set... | 2 |
| [`file_watch`](#file_watch) | Polls a file or directory for create/modify/delete events and returns them as JSON. Suitable for log-monitor and file... | 6 |
| [`file_write`](#file_write) | Write content to a file | 2 |
| [`gemini`](#gemini) | Call Google Gemini LLM API | 15 |
| [`glm`](#glm) | Call GLM LLM API | 15 |
| [`harmony_ability`](#harmony_ability) | Launch HarmonyOS Ability with specified type. Supports page (UI), slice (partial UI), service (background), data (dat... | 5 |
| [`harmony_atomic_service`](#harmony_atomic_service) | Launch HarmonyOS Atomic Service. Lightweight, card-based services that run without installation. Supports launch, rou... | 4 |
| [`harmony_device_adapt`](#harmony_device_adapt) | Detect HarmonyOS device type (phone/tablet/foldable/TV/car/wearable) and generate UI adaptation guidance. Inspired by... | 6 |
| [`harmony_widget`](#harmony_widget) | Manage HarmonyOS desktop widgets (service cards). Add, update, remove, or query widget state on the home screen. | 5 |
| [`http_request`](#http_request) | Make HTTP requests with custom method, headers, and body | 12 |
| [`human_in_loop`](#human_in_loop) | Human approval gate — pauses workflow for human review and approval before continuing | 5 |
| [`ima`](#ima) | Call IMA Copilot LLM API | 15 |
| [`intent_router`](#intent_router) | Route user intents to appropriate handlers. Central dispatch for AI assistant commands. | 3 |
| [`internlm`](#internlm) | Call InternLM LLM API | 15 |
| [`json_parse`](#json_parse) | Parse and extract JSON data | 1 |
| [`kimi`](#kimi) | Call Kimi LLM API | 15 |
| [`knowledge_graph`](#knowledge_graph) | Knowledge graph node - extract entities/relations, build graph, query and traverse | 13 |
| [`llm_router`](#llm_router) | Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization | 5 |
| [`memory`](#memory) | AI Agent memory infrastructure with session-isolated persistent knowledge graph engine. Supports multi-session parall... | 16 |
| [`mimo`](#mimo) | Call MiMo LLM API | 15 |
| [`minimax`](#minimax) | Call MiniMax LLM API | 15 |
| [`mistral`](#mistral) | Call Mistral LLM API | 15 |
| [`moe_streaming`](#moe_streaming) | MoE (Mixture of Experts) streaming expert loading for running large models on consumer hardware | 7 |
| [`multimodal`](#multimodal) | Multimodal node for image analysis, OCR, and audio transcription using vision-capable LLMs | 10 |
| [`node_marketplace`](#node_marketplace) | Node marketplace - list, search, and categorize available workflow nodes | 4 |
| [`notify`](#notify) | Send notifications (stdout, stderr, slack, discord, telegram, webhook) | 10 |
| [`office`](#office) | Read .docx/.xlsx/.pptx documents (text, tables, slides) using pure-Go OOXML parsing | 5 |
| [`ollama`](#ollama) | Call Ollama local LLM server | 3 |
| [`ondevice_llm`](#ondevice_llm) | Run LLM inference locally on the device (no cloud required). Supports 1B-8B models with INT4/INT8 quantization, inclu... | 10 |
| [`openai`](#openai) | Call OpenAI LLM API | 15 |
| [`pipeline`](#pipeline) | Dependency-based parallel workflow executor: steps run as soon as their dependencies are met, no global barriers (Tun... | 2 |
| [`planner`](#planner) | Task decomposition agent that breaks complex goals into actionable steps | 9 |
| [`plugin_system`](#plugin_system) | 插件系统节点。支持从本地目录、Git仓库、URL和插件市场加载插件，提供安装/卸载/更新/... | 4 |
| [`power_manager`](#power_manager) | Control power consumption for on-device AI. Supports eco/balanced/high profiles with adaptive battery and thermal man... | 7 |
| [`preference`](#preference) | User preference memory: store, retrieve, and learn user habits across sessions (MemSlides-inspired user profiling) | 7 |
| [`qwen`](#qwen) | Call Qwen LLM API | 15 |
| [`rag`](#rag) | Retrieval Augmented Generation node - chunk documents, search by query, and assemble context | 7 |
| [`reflector`](#reflector) | Self-reflection agent that critiques output and iteratively improves it (Reflexion pattern) | 7 |
| [`researcher`](#researcher) | Research agent that fetches information from URLs and summarizes findings | 7 |
| [`robot_action`](#robot_action) | Embodied-intelligence action planner. Turns a natural-language instruction plus optional visual/proprioceptive state ... | 9 |
| [`robot_control`](#robot_control) | Plan and execute robot action sequences for embodied AI. Supports humanoid robots, mobile bases, robotic arms, drones... | 10 |
| [`screen_understanding`](#screen_understanding) | Understand screen content like an L3 agent: parse UI elements, identify actionable items, and generate interaction pl... | 5 |
| [`search_aggregate`](#search_aggregate) | Multi-platform search aggregator with real-signal ranking: Reddit/Twitter/YouTube/HN/GitHub, sorted by votes/comments... | 8 |
| [`send_notification`](#send_notification) | Send system notification with actions. Cross-platform notification for AI assistants. | 5 |
| [`sensenova`](#sensenova) | 商汤SenseNova日日新大模型集成。支持U1系列多模态模型（U1-Lite/U1-Pro）、Flash系列推理模型... | 9 |
| [`session_manager`](#session_manager) | Multi-session memory management. Create isolated sessions, fork a session from a parent, merge sessions, and share fa... | 11 |
| [`skill_distill`](#skill_distill) | Distill methodologies from books, videos, podcasts, and documents into callable skills. Supports workflow, decision, ... | 6 |
| [`skill_explorer`](#skill_explorer) | Discover, evaluate, and recommend skills from the ecosystem. Quality scoring, category browsing, and smart recommenda... | 5 |
| [`sql_query`](#sql_query) | Execute SQL via database/sql. The driver must be registered by the host program. Uses parameterized queries (? or $1)... | 8 |
| [`structured_output`](#structured_output) | LLM-driven structured output with local JSON Schema validation and self-correction retries | 11 |
| [`supervisor`](#supervisor) | Advanced supervisor with MoE routing, MindSearch deep research, 232+ domain specialists, and collaboration templates | 13 |
| [`system_event`](#system_event) | Listen for mobile system events (notification, call, SMS, location, battery, etc.) and trigger workflows | 7 |
| [`template_render`](#template_render) | Render Go templates with input data | 2 |
| [`test_node`](#test_node) | Test node for development purposes | 1 |
| [`transform`](#transform) | Transform text using string operations | 1 |
| [`ui_automate`](#ui_automate) | Automate UI interactions: click, type, scroll, swipe. Accessibility-based automation for AI assistants. | 7 |
| [`verify`](#verify) | Agent-as-a-Judge verifier that validates outputs, claims, and results against specified criteria | 10 |
| [`video_edit`](#video_edit) | AI-powered video editing workflow with smart cutting, merging, effects, subtitle generation, and storyboard creation. | 7 |
| [`voice_input`](#voice_input) | Voice input pipeline: VAD (Voice Activity Detection), wake word detection, and speech-to-text. Supports on-device rec... | 7 |
| [`voice_output`](#voice_output) | Voice AI toolchain with TTS, voice cloning, ASR speech recognition, transcription, diarization, and voice analysis. S... | 19 |
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

## agent_browser

Agent-optimized web browser for autonomous web navigation, content extraction, and research. Inspired by CitroLabs/ego-lite - zero-cost browser state sharing.

- **Input**: string - URL to visit or browser action to perform
- **Output**: string - Page content, extraction results, or browser status

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | visit | Browser action: visit|extract|links|screenshot|search|summary|connect_existing|import_cookies (default: visit) |
| `url` | string | No |  | Target URL (overrides input if provided) |
| `selector` | string | No |  | CSS selector for content extraction (optional) |
| `max_depth` | string | No | 1 | Maximum link follow depth for crawling (default: 1) |
| `output_format` | string | No | markdown | Output format: markdown|text|json|html (default: markdown) |
| `summary_length` | string | No | 2000 | Maximum summary length in characters (default: 2000) |
| `render_js` | string | No | false | Enable JavaScript rendering (default: false) |
| `use_session` | string | No | false | Reuse authenticated browser session (default: false) |
| `browser_profile` | string | No |  | Browser profile path for session reuse (optional) |
| `cdp_port` | string | No | 9222 | Chrome DevTools Protocol port (default: 9222) |

---

## agent_inbox

Query agent message inbox. Retrieve and manage cross-domain messages.

- **Input**: string - optional filter query
- **Output**: string - inbox messages in JSON

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | list | Action: list, read, delete, mark_read (default: list) |
| `message_id` | string | No |  | Message ID for read/delete/mark_read |
| `from_did` | string | No |  | Filter by sender DID |
| `limit` | string | No | 10 | Max messages to return (default: 10) |

---

## agent_message

Send cross-domain message to another agent by DID. Inspired by awiki.ai agent-native messaging.

- **Input**: string - message body or payload
- **Output**: string - send result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `to_did` | string | Yes |  | Receiver agent DID (e.g. did:awiki:agent123) |
| `from_did` | string | No |  | Sender agent DID |
| `subject` | string | No |  | Message subject/type |
| `priority` | string | No | normal | Priority: low, normal, high, urgent (default: normal) |
| `endpoint` | string | No |  | Target agent endpoint URL (optional, for direct send) |

---

## andesgpt

OPPO AndesGPT large model integration. Supports Tiny (端侧1B), Turbo (端云协同7B), Titan (云端100B+) sizes with PersonaX personalization and end-cloud collaboration.

- **Input**: string - user prompt or conversation message
- **Output**: string - model response with persona and collaboration metadata

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model_size` | string | No | turbo | Model size: tiny (端侧1B) / turbo (端云协同7B) / titan (云端100B+) (default: turbo) |
| `scene` | string | No | life | Application scene: life/imaging/productivity/creative/knowledge (default: life) |
| `persona_id` | string | No |  | PersonaX persona ID for personalized responses (千人千面) |
| `use_memory` | bool | No | true | Use PersonaX long-term memory (default: true) |
| `max_tokens` | int | No | 1024 | Max output tokens (default: 1024) |
| `temperature` | float | No | 0.7 | Sampling temperature 0.0-1.0 (default: 0.7) |
| `system_prompt` | string | No |  | System prompt |
| `stream` | bool | No | false | Enable streaming response (default: false) |
| `end_cloud_mode` | string | No | auto | End-cloud collaboration mode: auto/force_end/force_cloud (default: auto) |

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

## antling

蚂蚁百灵（Ant Ling）大模型集成。支持Ling-2.6通用系列、Ring-2.6推理系列、Ming全模态系列，通过OpenAI兼容API接入，覆盖聊天、代码、分析、创意、多模态等场景。

- **Input**: string - user prompt or task description
- **Output**: string - model response with reasoning and agent capabilities

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | ling-2.6-flash | Model: ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0 (default: ling-2.6-flash) |
| `scene` | string | No | chat | Scene: chat/code/analysis/creative/multimodal (default: chat) |
| `api_key` | string | No |  | Ant Ling API key (from chat.ant-ling.com/open) |
| `base_url` | string | No | https://api.ant-ling.com/v1 | API base URL (default: https://api.ant-ling.com/v1) |
| `max_tokens` | int | No | 2048 | Max output tokens (default: 2048) |
| `temperature` | float | No | 0.7 | Sampling temperature 0.0-2.0 (default: 0.7) |
| `system_prompt` | string | No |  | System prompt |
| `stream` | bool | No | false | Enable streaming response (default: false) |

---

## app_launch

Launch a mobile/desktop app with optional parameters. Cross-platform app control for AI systems. Supports Android, iOS, and HarmonyOS.

- **Input**: string - optional input to pass to the app
- **Output**: string - launch result and app session info

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `app` | string | Yes |  | App identifier: package name (Android), bundle ID (iOS), bundleName (HarmonyOS), app name (desktop) |
| `action` | string | No | open | Deep link action: open, search, share, edit, view (default: open) |
| `uri` | string | No |  | Deep link URI or URL scheme. HarmonyOS uses ohos:// or host:// scheme |
| `platform` | string | No |  | Target platform: android, ios, harmony, desktop (auto-detected if omitted) |
| `params` | string | No |  | JSON parameters to pass to the app |
| `wait` | string | No | true | Wait for app to fully launch: true/false (default: true) |
| `timeout` | string | No | 10s | Launch timeout (default: 10s) |

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

## blockchain_audit

Record workflow execution on blockchain for tamper-proof audit trails. Supports Ethereum, Hyperledger Fabric, and simulated chains. Aligns with WAIC Agent Interoperability Initiative.

- **Input**: string - workflow execution data or action description
- **Output**: string - blockchain transaction receipt with hash and timestamp

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `chain_type` | string | No | simulated | Blockchain type: ethereum/hyperledger/fabric/quorum/simulated (default: simulated) |
| `audit_level` | string | No | workflow | Audit granularity: workflow/node/parameter/full (default: workflow) |
| `workflow_id` | string | Yes |  | Unique workflow identifier |
| `node_id` | string | No |  | Node instance identifier |
| `actor_did` | string | No |  | W3C DID of the actor executing the workflow |
| `previous_hash` | string | No |  | Hash of previous audit record for chain linkage |
| `metadata` | string | No |  | Additional JSON metadata to include in audit record |

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

## clarify

Pre-execution ambiguity checker: identifies unclear requirements and generates clarifying questions (ACQUIRE framework)

- **Input**: string - the task or goal to analyze for ambiguity
- **Output**: string - JSON with clarification result: needs_clarification, questions, confidence, clarified_goal

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `threshold` | string | No | 70 | Confidence threshold 0-100, below this trigger clarification (default: 70) |
| `max_questions` | string | No | 5 | Max clarification questions to ask (default: 5) |
| `context` | string | No |  | Additional context about the task |
| `user_answers` | string | No |  | JSON object of user's answers to previous questions (question -> answer) |

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

## code_knowledge_graph

Semantic code knowledge graph with vector retrieval, 158 language support, MCP tool exposure, and token-efficient review. Supports incremental updates and persistent indexing.

- **Input**: string - optional query context or MCP tool call
- **Output**: string - JSON format with entities, relations, concepts, query results, or MCP tool response

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | Code path to analyze |
| `mode` | string | No | build_and_query | Mode: build/build_and_query/query_only/incremental/mcp_tool (default: build_and_query) |
| `query` | string | No |  | Query statement (text or vector) |
| `query_type` | string | No | semantic | Query type: semantic/symbol/path/relation (default: semantic) |
| `top_k` | int | No | 10 | Number of results to return (1-100, default: 10) |
| `threshold` | float | No | 0.7 | Similarity threshold 0.0-1.0 (default: 0.7) |
| `vector_dim` | int | No | 384 | Vector dimension (default: 384) |
| `mcp_tool` | string | No |  | MCP tool to call: list_entities/search_graph/analyze_dependencies/get_entity_details/list_relations/generate_summary |
| `entity_name` | string | No |  | Entity name for get_entity_details tool |
| `token_efficient` | bool | No | true | Enable token-efficient review mode (default: true) |
| `incremental_update` | bool | No | false | Use incremental update (only process changed files) |
| `use_cache` | bool | No | true | Use persistent cache index (default: true) |
| `force_rebuild` | bool | No | false | Force rebuild index from scratch (default: false) |

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

## compress

Intelligent context compression with 6 algorithms: extractive, keyword, cluster, sliding_window, hybrid (headroom-inspired, 60-95% token reduction)

- **Input**: string - text to compress
- **Output**: string - compressed text with metadata

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `algorithm` | string | No | hybrid | extract|keyword|cluster|sliding_window|hybrid (default: hybrid) |
| `ratio` | string | No | 0.2 | Target compression ratio 0.01-1.0, lower=more aggressive (default: 0.2) |
| `max_chars` | string | No | 4000 | Maximum output characters (default: 4000) |
| `preserve_headers` | string | No | true | Preserve section headers (default: true) |
| `preserve_numbers` | string | No | true | Preserve sentences with numbers/stats (default: true) |
| `output` | string | No | text | text|json|stats (default: text) |
| `keywords` | string | No | 0 | Also extract top N keywords (default: 0) |

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

## context_fs

Unified context filesystem (OpenViking-inspired): ls/cat/write/rm/search over memory, profile, knowledge graph, and skills via a single virtual path namespace.

- **Input**: string - content for write op, or query for search op (used when content/query params are absent)
- **Output**: string - file content (cat), JSON entries (ls/search), or status message (write/rm)

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | ls | ls|cat|write|rm|search (default: ls) |
| `path` | string | No |  | Virtual path, e.g. /mem/short/note, /profile/coding_style/lang, /kg/entities/foo, /skills/bar, or / for root listing |
| `content` | string | No |  | Content to write (defaults to input) |
| `query` | string | No |  | Search query (defaults to input) |
| `scope` | string | No |  | Search scope: /mem, /profile, /kg, /skills, or empty for all |
| `top_k` | int | No | 10 | Max results for search (default: 10) |

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

## cross_app_action

Execute actions across multiple apps. Multi-app workflows for AI assistants.

- **Input**: string - action description or context
- **Output**: string - cross-app action result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `workflow` | string | Yes |  | Workflow name: share_content, save_for_later, compare_prices |
| `apps` | string | No |  | Comma-separated apps involved |
| `data` | string | No |  | JSON data to pass between apps |

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

## device_state

Query device state: battery, network, location, apps, storage. Context awareness for AI assistants.

- **Input**: string - optional query filter
- **Output**: string - device state information in JSON

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `query` | string | No | all | State query: battery, network, location, apps, storage, all (default: all) |

---

## doc_gen

AI自动文档生成节点。自动生成和更新代码库文档，支持README、API文档、函数注释、模块文档、更新日志、教程和架构文档等多种类型，让代码库对AI Agent更友好。

- **Input**: string - 代码内容或文档生成指令
- **Output**: string - 生成的文档内容（markdown或JSON格式）

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `doc_type` | string | Yes |  | 文档类型：readme/api/function/module/changelog/tutorial/architecture |
| `path` | string | Yes |  | 代码路径（相对工作目录） |
| `language` | string | No | auto | 代码语言：go/python/javascript/typescript/auto（默认auto） |
| `output_format` | string | No | markdown | 输出格式：markdown/json（默认markdown） |
| `depth` | int | No | 3 | 文档深度1-5（默认3） |
| `auto_update` | bool | No | false | 是否自动更新现有文档（默认false） |

---

## doc_parse

Parse documents (PDF/images/HTML) into text, LaTeX, or HTML table format

- **Input**: string - document text (source=text), base64-encoded image/PDF (source=base64), or URL (source=URL)
- **Output**: string - parsed document content in the requested output format

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source` | string | No | text | Input source type: text|base64|URL (default: text) |
| `output_format` | string | No | text | Output format: text|latex|html_table (default: text) |
| `extract_tables` | bool | No | false | Extract markdown tables and return their count/content (default: false) |
| `extract_formulas` | bool | No | false | Extract LaTeX formulas ($...$, $$...$$) and return their list (default: false) |
| `lang` | string | No | auto | Document language hint: zh|en|auto (default: auto, passed to OCR API) |
| `api_endpoint` | string | No |  | OCR API endpoint URL (optional). When set with api_key, calls external OCR (e.g. OvisOCR2) |
| `api_key` | string | No |  | OCR API key (optional) |

---

## engineer_skills

Pre-built engineering skill package with TypeScript/React/Node.js expertise. Supports skill matching, application, and version management.

- **Input**: string - task description for skill matching
- **Output**: string - JSON with skills information

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | list | Action: list/match/apply/get (default: list) |
| `skill_category` | string | No |  | Skill category: frontend/backend/devops/architecture |
| `skill_name` | string | No |  | Skill name |
| `task_description` | string | No |  | Task description for matching (max 5000 chars) |
| `version` | string | No | 1.0.0 | Skill version |

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

## harmony_ability

Launch HarmonyOS Ability with specified type. Supports page (UI), slice (partial UI), service (background), data (data provider).

- **Input**: string - optional input data for the ability
- **Output**: string - ability launch result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `bundle_name` | string | Yes |  | HarmonyOS bundle name (e.g. com.example.myapplication) |
| `ability_name` | string | Yes |  | Ability class name (e.g. MainAbility) |
| `ability_type` | string | No | page | Ability type: page, slice, service, data (default: page) |
| `uri` | string | No |  | Deep link URI using ohos:// scheme |
| `params` | string | No |  | JSON parameters for the ability |

---

## harmony_atomic_service

Launch HarmonyOS Atomic Service. Lightweight, card-based services that run without installation. Supports launch, router, share, notify actions.

- **Input**: string - optional input data
- **Output**: string - atomic service launch result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `service_id` | string | Yes |  | Atomic service ID (e.g. com.example.service) |
| `action` | string | No | launch | Action: launch, router, share, notify (default: launch) |
| `card_id` | string | No |  | Service card ID for widget-style display |
| `params` | string | No |  | JSON parameters for the service |

---

## harmony_device_adapt

Detect HarmonyOS device type (phone/tablet/foldable/TV/car/wearable) and generate UI adaptation guidance. Inspired by HarmonyOS Agent Skills multi-device adaptation.

- **Input**: string - optional adaptation requirements
- **Output**: string - device info and adaptation plan in JSON

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `screen_width` | string | No |  | Screen width in pixels (optional, auto-detected if omitted) |
| `screen_height` | string | No |  | Screen height in pixels (optional) |
| `screen_density` | string | No |  | Screen density (dpi, optional) |
| `device_type` | string | No |  | Device type hint: phone_standard, phone_dual_fold, phone_triple_fold, tablet, smart_screen, car, wearable (auto-detected if omitted) |
| `fold_state` | string | No |  | Fold state for foldable devices: unfolded, half_folded, fully_folded (optional) |
| `orientation` | string | No | auto | Orientation: portrait, landscape, auto (default: auto) |

---

## harmony_widget

Manage HarmonyOS desktop widgets (service cards). Add, update, remove, or query widget state on the home screen.

- **Input**: string - optional widget content or query
- **Output**: string - widget operation result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | query | Action: add, update, remove, query (default: query) |
| `widget_id` | string | No |  | Widget identifier (required for update, remove, query) |
| `provider_bundle` | string | No |  | Provider bundle name for add action |
| `widget_name` | string | No |  | Widget ability name for add action |
| `data` | string | No |  | JSON data to update widget content |

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

## intent_router

Route user intents to appropriate handlers. Central dispatch for AI assistant commands.

- **Input**: string - user utterance or intent text
- **Output**: string - routed intent with handler assignment

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `mode` | string | No | classify | Routing mode: classify, route, execute (default: classify) |
| `domains` | string | No |  | Comma-separated domains: travel,food,shopping,entertainment,work,communication,system |
| `fallback` | string | No | general_assistant | Fallback handler when no match (default: general_assistant) |

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

## knowledge_graph

Knowledge graph node - extract entities/relations, build graph, query and traverse

- **Input**: string - text to extract knowledge from, or a query for an existing graph
- **Output**: string - knowledge graph data or query results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | extract | Action: extract, extract_llm, query, traverse, stats, visualize (default: extract). extract_llm calls an LLM for higher-quality entity/relation extraction. |
| `graph_path` | string | No |  | Path to save/load the graph JSON file |
| `query` | string | No |  | Query for search/traverse (entity name or relation type) |
| `max_depth` | string | No | 2 | Max traversal depth (default: 2) |
| `top_k` | string | No | 10 | Max results to return (default: 10) |
| `format` | string | No | markdown | Output format: json, markdown, mermaid (default: markdown) |
| `provider` | string | No | openai | LLM provider for extract_llm (default: openai) |
| `model` | string | No |  | LLM model name for extract_llm |
| `api_key` | string | No |  | LLM API key for extract_llm (or set <PROVIDER>_API_KEY env var) |
| `endpoint` | string | No |  | LLM API base URL for extract_llm |
| `language` | string | No | en | Prompt language hint for extract_llm: en or zh (default: en) |
| `session_id` | string | No |  | C-3: when set with memory_key, links extracted entities to that memory entry |
| `memory_key` | string | No |  | C-3: memory entry key to link extracted entities to |

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

## moe_streaming

MoE (Mixture of Experts) streaming expert loading for running large models on consumer hardware

- **Input**: string - optional input prompt for inference
- **Output**: string - JSON format with MoE loading metrics

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | Yes |  | Model name: colibri-744b-moe/qwen3-moe-72b/mixtral-8x7b/llama3.3-moe-70b |
| `max_experts_per_token` | int | No | 2 | Max experts per token (default: 2) |
| `expert_group_size` | int | No | 64 | Expert group size (default: 64) |
| `memory_limit_gb` | float | No | 24 | Memory limit in GB (default: 24) |
| `streaming` | bool | No | true | Enable streaming inference (default: true) |
| `quantization` | string | No | int4 | Quantization: int4/int8/fp16/fp32 (default: int4) |
| `load_strategy` | string | No | on_demand | Load strategy: on_demand/layered/preload (default: on_demand) |

---

## multimodal

Multimodal node for image analysis, OCR, and audio transcription using vision-capable LLMs

- **Input**: string - the question or instruction about the media
- **Output**: string - analysis result from the multimodal model

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `mode` | string | No | describe | Mode: image, ocr, describe, compare (default: describe) |
| `image_path` | string | No |  | Path to image file (local path or URL) |
| `image_paths` | string | No |  | Comma-separated paths for compare mode |
| `lang` | string | No | eng+chi_sim | OCR languages for tesseract (default: eng+chi_sim) |
| `provider` | string | No | openai | LLM provider with vision support (default: openai) |
| `model` | string | No | gpt-4o | Vision model name (default: gpt-4o) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint |
| `detail` | string | No | auto | Image detail level: low, high, auto (default: auto) |
| `output_format` | string | No | markdown | Output format: text, json, markdown (default: markdown) |

---

## node_marketplace

Node marketplace - list, search, and categorize available workflow nodes

- **Input**: string - search query (for search action)
- **Output**: string - node listing or search results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | list | Action: list, search, categories, count, details (default: list) |
| `category` | string | No |  | Filter by category: llm, agent, io, transform, flow, data, utility |
| `format` | string | No | markdown | Output format: text, markdown, json (default: markdown) |
| `node_name` | string | No |  | Node name for details action |

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

## office

Read .docx/.xlsx/.pptx documents (text, tables, slides) using pure-Go OOXML parsing

- **Input**: string - path to the office document (.docx/.xlsx/.pptx)
- **Output**: string - extracted content in the requested output format

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | No |  | Path to the office document (overrides input if set) |
| `format` | string | No |  | Source format hint: docx|xlsx|pptx (default: inferred from extension) |
| `output` | string | No | markdown | Output format: text|markdown|json (default: markdown) |
| `sheet` | string | No |  | xlsx only: sheet name to read (default: all sheets) |
| `max_rows` | int | No | 1000 | xlsx only: max rows per sheet (default: 1000, 0 = unlimited) |

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

## ondevice_llm

Run LLM inference locally on the device (no cloud required). Supports 1B-8B models with INT4/INT8 quantization, including SenseNova U1 open-source models

- **Input**: string - user prompt or conversation context
- **Output**: string - model response with metadata

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | Yes |  | Model name (e.g., qwen2-1.5b, minicpm-2b, phi-3-mini) |
| `model_path` | string | No |  | Path to model files directory |
| `backend` | string | No | llama.cpp | Inference backend: llama.cpp/mlc-llm/onnx/ncnn/mnn/paddle-lite (default: llama.cpp) |
| `quantization` | string | No | int4 | Quantization: int4/int8/fp16/q4_0/q4_1/q5_0/q5_1/q8_0 (default: int4) |
| `max_tokens` | int | No | 512 | Max output tokens (default: 512) |
| `temperature` | float | No | 0.7 | Sampling temperature 0.0-2.0 (default: 0.7) |
| `system_prompt` | string | No |  | System prompt for the model |
| `context_size` | int | No | 4096 | Context window size (default: 4096) |
| `threads` | int | No |  | Number of CPU threads (default: auto) |
| `use_gpu` | bool | No | true | Use GPU/NPU acceleration if available (default: true) |

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

## plugin_system

插件系统节点。支持从本地目录、Git仓库、URL和插件市场加载插件，提供安装/卸载/更新/列出/启用/禁用等管理功能，支持沙箱隔离和版本管理。

- **Input**: string - 插件相关输入（可选，用于特定操作）
- **Output**: string - JSON格式的插件操作结果

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | Yes |  | 操作：install/uninstall/update/list/enable/disable/info |
| `plugin_id` | string | No |  | 插件ID |
| `source` | string | No |  | 插件来源：local/git/url/market |
| `version` | string | No |  | 版本号 |

---

## power_manager

Control power consumption for on-device AI. Supports eco/balanced/high profiles with adaptive battery and thermal management

- **Input**: string - command or query
- **Output**: string - power status and recommendations

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `profile` | string | No | balanced | Power profile: eco/balanced/high (default: balanced) |
| `max_inference_hz` | float | No | 2.0 | Max inference calls per second (default: 2.0) |
| `min_battery_pct` | int | No | 20 | Auto-switch to eco when battery below this % (default: 20) |
| `thermal_limit_c` | float | No | 75.0 | Thermal throttle threshold in Celsius (default: 75.0) |
| `adaptive_mode` | bool | No | true | Enable adaptive power management (default: true) |
| `battery_aware` | bool | No | true | Monitor battery level (default: true) |
| `thermal_aware` | bool | No | true | Monitor CPU temperature (default: true) |

---

## preference

User preference memory: store, retrieve, and learn user habits across sessions (MemSlides-inspired user profiling)

- **Input**: string - input depends on operation (value to set, key to get, etc.)
- **Output**: string - result of the operation

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | get | get|set|learn|summary|category|prompt_addon (default: get) |
| `user_id` | string | No | default | User identifier (default: default) |
| `category` | string | No | custom | coding_style|output_format|model_choice|verbosity|language|safety|workflow|custom |
| `key` | string | No |  | Preference key name |
| `value` | string | No |  | Preference value (for set/learn operations) |
| `confidence` | string | No |  | Confidence 0-1, default 0.6 for learn, 1.0 for set |
| `source` | string | No | explicit | Where this preference came from (explicit|learned|config) |

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

## rag

Retrieval Augmented Generation node - chunk documents, search by query, and assemble context

- **Input**: string - the query to search for
- **Output**: string - assembled context from relevant document chunks

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source` | string | Yes |  | Source: file path, directory path, or text content |
| `source_type` | string | No | auto | Type of source: file, dir, text (default: auto) |
| `chunk_size` | string | No | 1000 | Chunk size in characters (default: 1000) |
| `chunk_overlap` | string | No | 200 | Chunk overlap in characters (default: 200) |
| `top_k` | string | No | 5 | Number of top chunks to retrieve (default: 5) |
| `search_method` | string | No | keyword | Search method: keyword, hybrid (default: keyword) |
| `include_metadata` | string | No | true | Include chunk metadata in output (default: true) |

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

## robot_action

Embodied-intelligence action planner. Turns a natural-language instruction plus optional visual/proprioceptive state into a structured action sequence. Defaults to a deterministic simulator; set backend=api to call an external VLA server.

- **Input**: string - natural-language instruction (e.g. "make a sandwich")
- **Output**: string - JSON action plan

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `backend` | string | No | simulate | simulate (default) | api |
| `instruction` | string | No |  | Instruction text. Overrides input when set. |
| `observation` | string | No |  | JSON describing the environment (objects, positions, sensor readings). Optional. |
| `image_base64` | string | No |  | Base64-encoded current camera frame (api backend only). Optional. |
| `robot_type` | string | No | arm | Robot form factor: arm | mobile | humanoid | gripper (default arm) |
| `max_steps` | string | No | 20 | Maximum action steps to plan (default 20) |
| `api_endpoint` | string | No |  | VLA model HTTP endpoint (api backend). Required when backend=api. |
| `api_key` | string | No |  | Bearer token for the VLA endpoint (optional). |
| `timeout` | string | No | 30 | Per-request timeout in seconds for the api backend (default 30) |

---

## robot_control

Plan and execute robot action sequences for embodied AI. Supports humanoid robots, mobile bases, robotic arms, drones. Decomposes natural language tasks into low-level robot commands.

- **Input**: string - natural language task description
- **Output**: string - robot action plan with safety checks

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `robot_type` | string | No | humanoid | Robot type: humanoid/mobile_base/arm/drone/dog/wheelchair (default: humanoid) |
| `robot_id` | string | No |  | Unique robot identifier |
| `action` | string | No |  | Specific action: move/pick/place/rotate/grasp/release/navigate/scan/speak/wait |
| `target_object` | string | No |  | Target object to interact with |
| `target_location` | string | No |  | Target location (x,y,z or named place) |
| `speed` | float | No | 0.5 | Movement speed 0.0-1.0 (default: 0.5) |
| `force_limit` | float | No | 10.0 | Max force in Newtons (default: 10.0) |
| `safety_zone_m` | float | No | 0.5 | Safety zone radius in meters (default: 0.5) |
| `visual_feedback` | bool | No | true | Use visual feedback for verification (default: true) |
| `tactile_feedback` | bool | No | true | Use tactile feedback for grasping (default: true) |

---

## screen_understanding

Understand screen content like an L3 agent: parse UI elements, identify actionable items, and generate interaction plans. Supports mobile app screens, web pages, and system UIs.

- **Input**: string - screen description or OCR text dump
- **Output**: string - structured screen analysis with interaction plan

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `platform` | string | No | android | Screen platform: android/ios/harmony/web (default: android) |
| `action` | string | No | analyze | Goal action: analyze/interact/navigate (default: analyze) |
| `target_element` | string | No |  | Target UI element to interact with |
| `target_app` | string | No |  | Target app package name for navigation |
| `ocr_text` | string | No |  | OCR-extracted text from screen |

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

## send_notification

Send system notification with actions. Cross-platform notification for AI assistants.

- **Input**: string - notification body text
- **Output**: string - notification send result

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | string | Yes |  | Notification title |
| `body` | string | No |  | Notification body (overrides input) |
| `priority` | string | No | default | Priority: low, default, high, max (default: default) |
| `actions` | string | No |  | JSON array of action buttons |
| `sound` | string | No | default | Notification sound: default, none, or file path |

---

## sensenova

商汤SenseNova日日新大模型集成。支持U1系列多模态模型（U1-Lite/U1-Pro）、Flash系列推理模型，涵盖聊天、代码、图像、文档、数据分析等场景。

- **Input**: string - user prompt or conversation message
- **Output**: string - model response with multi-modal capabilities

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | flash-lite | Model: u1-lite/u1-lite-moe/u1-pro/flash-lite/flash/flash-pro (default: flash-lite) |
| `scene` | string | No | chat | Scene: chat/code/image/document/data/workflow (default: chat) |
| `api_key` | string | No |  | SenseNova API key (from sensenova.cn) |
| `max_tokens` | int | No | 2048 | Max output tokens (default: 2048) |
| `temperature` | float | No | 0.7 | Sampling temperature 0.0-2.0 (default: 0.7) |
| `system_prompt` | string | No |  | System prompt |
| `stream` | bool | No | false | Enable streaming response (default: false) |
| `enable_tools` | bool | No | true | Enable tool calling (default: true) |
| `vision` | bool | No | false | Enable vision capabilities (for U1 models) (default: false) |

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

## skill_distill

Distill methodologies from books, videos, podcasts, and documents into callable skills. Supports workflow, decision, analysis, creative, prompt, and checklist skill types.

- **Input**: string - source content to distill
- **Output**: string - JSON with distilled skill structure

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source_type` | string | No | article | Source type: book/video/podcast/article/documentation/conversation (default: article) |
| `distill_type` | string | No | workflow | Distill type: workflow/decision/analysis/creative/prompt/checklist (default: workflow) |
| `content` | string | No |  | Source content text (max 100000 chars) |
| `skill_name` | string | No |  | Target skill name |
| `max_steps` | int | No | 10 | Max number of steps (default: 10) |
| `quality` | string | No | standard | Quality level: basic/standard/expert (default: standard) |

---

## skill_explorer

Discover, evaluate, and recommend skills from the ecosystem. Quality scoring, category browsing, and smart recommendations. Inspired by awesome-claude-skills.

- **Input**: string - search query for skills or empty to list all
- **Output**: string - skills listing with quality scores and recommendations

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | list | Action: list|search|recommend|evaluate|categories (default: list) |
| `category` | string | No | all | Filter by category: development,productivity,research,creative,business,all (default: all) |
| `min_quality` | string | No | 50 | Minimum quality score 0-100 (default: 50) |
| `limit` | string | No | 20 | Maximum results (default: 20) |
| `sort_by` | string | No | quality | Sort by: quality|name|category (default: quality) |

---

## sql_query

Execute SQL via database/sql. The driver must be registered by the host program. Uses parameterized queries (? or $1) to prevent SQL injection. Read-only by default (SELECT/SHOW/EXPLAIN/PRAGMA only); set read_only=false to allow DML/DDL. Supports a 'schema' action that lists tables and columns.

- **Input**: string - SQL query (when action=query and no `sql` param, input is used as the query)
- **Output**: string - JSON array of rows (query), or schema description (schema action)

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | query | query (default) | schema | tables |
| `driver` | string | Yes |  | database/sql driver name (e.g. sqlite3, postgres, mysql) |
| `dsn` | string | Yes |  | Data source name (driver-specific). For SQLite: path to .db file. |
| `sql` | string | No |  | SQL statement. Use ? (mysql/sqlite) or $1,$2 (postgres) placeholders for `args`. |
| `args` | string | No |  | JSON array of bind parameters, e.g. ["foo", 42]. Optional. |
| `read_only` | string | No | true | Reject writes if true (default). Set false to allow INSERT/UPDATE/DELETE/DDL. |
| `max_rows` | string | No | 1000 | Max rows to return (default 1000). Protects against huge result sets. |
| `timeout` | string | No | 30 | Query timeout in seconds (default 30). |

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

## system_event

Listen for mobile system events (notification, call, SMS, location, battery, etc.) and trigger workflows

- **Input**: string - filter pattern or event configuration JSON
- **Output**: string - matched event data in JSON format

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `event_type` | string | Yes |  | Event type to listen for (notification/incoming_call/sms_received/alarm_triggered/location_changed/battery_low/battery_charging/screen_on/screen_off/app_foreground/bluetooth_connected/wifi_connected/headphone_connected) |
| `trigger_mode` | string | No | immediate | Trigger mode: immediate/debounce/throttle (default: immediate) |
| `debounce_ms` | int | No | 1000 | Debounce interval in milliseconds (default: 1000) |
| `filter_app` | string | No |  | Filter by app package name (for notification/app_foreground events) |
| `filter_keyword` | string | No |  | Filter by keyword in event content |
| `battery_threshold` | int | No | 20 | Battery level threshold 0-100 (for battery_low event) |
| `location_radius_m` | int | No | 100 | Location change radius in meters (default: 100) |

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

## test_node

Test node for development purposes

- **Input**: string - test input
- **Output**: string - test output with input and message

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `message` | string | No | Hello from test node! | Test message (default: Hello from test node!) |

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

## ui_automate

Automate UI interactions: click, type, scroll, swipe. Accessibility-based automation for AI assistants.

- **Input**: string - text to type or context for interaction
- **Output**: string - UI automation result and state changes

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | Yes |  | UI action: click, long_click, type, scroll, swipe, pinch, drag, wait_element, screenshot |
| `selector` | string | No |  | Element selector: id, text, content_desc, class |
| `x` | string | No |  | X coordinate for swipe/click (0-100% of screen width) |
| `y` | string | No |  | Y coordinate for swipe/click (0-100% of screen height) |
| `direction` | string | No |  | Scroll/swipe direction: up, down, left, right |
| `text` | string | No |  | Text to type (overrides input) |
| `duration` | string | No | 300 | Action duration in ms (default: 300) |

---

## verify

Agent-as-a-Judge verifier that validates outputs, claims, and results against specified criteria

- **Input**: string - the content to verify (used as claim if claim param is empty)
- **Output**: string - verification result with pass/fail, score, or detailed analysis

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `claim` | string | No |  | The claim or output to verify |
| `evidence` | string | No |  | Evidence or context to verify against |
| `criteria` | string | No |  | Verification criteria (comma-separated or natural language) |
| `verifier_type` | string | No | factual | Type: factual, code_correctness, security, logic, consistency, custom (default: factual) |
| `output_format` | string | No | detailed | Output: pass_fail, score, detailed, json (default: detailed) |
| `rubric` | string | No |  | Custom scoring rubric for verification (optional) |

---

## video_edit

AI-powered video editing workflow with smart cutting, merging, effects, subtitle generation, and storyboard creation.

- **Input**: string - input video file path or comma-separated list
- **Output**: string - JSON with video processing results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | smart_cut | Operation: smart_cut/merge/effects/subtitle/storyboard/upscale (default: smart_cut) |
| `input_files` | string | No |  | Comma-separated input video file paths |
| `output_path` | string | No | ./output.mp4 | Output file path |
| `style` | string | No | cinematic | Style: cinematic/creative/minimal/tech (default: cinematic) |
| `duration` | float | No |  | Target duration in seconds |
| `resolution` | string | No | 1080p | Resolution: 720p/1080p/4k (default: 1080p) |
| `language` | string | No | 中文 | Subtitle language: 中文/英文/日文/韩文 (default: 中文) |

---

## voice_input

Voice input pipeline: VAD (Voice Activity Detection), wake word detection, and speech-to-text. Supports on-device recognition for privacy.

- **Input**: string - raw audio data (base64) or audio file path
- **Output**: string - recognized text with confidence and metadata

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `mode` | string | No | full_asr | Pipeline mode: vad_only/wake_word/full_asr (default: full_asr) |
| `wake_word` | string | No | hey_box | Wake word to detect: hey_box/hello_box/hi_box/ok_box/box_box (default: hey_box) |
| `language` | string | No | zh | Recognition language: zh/en/ja/ko/fr/de/es (default: zh) |
| `vad_mode` | string | No | adaptive | VAD sensitivity: fixed/adaptive/aggressive (default: adaptive) |
| `max_duration_sec` | int | No | 30 | Max audio duration in seconds (default: 30) |
| `offline` | bool | No | true | Use on-device recognition only (default: true) |
| `confidence_threshold` | float | No | 0.7 | Minimum confidence 0.0-1.0 (default: 0.7) |

---

## voice_output

Voice AI toolchain with TTS, voice cloning, ASR speech recognition, transcription, diarization, and voice analysis. Supports SenseVoice, CosyVoice, Fish Speech, Edge TTS, Piper, Bark, Whisper, Vosk for complete voice studio capabilities.

- **Input**: string - text to synthesize, reference audio for cloning, or audio base64 for ASR transcription
- **Output**: string - JSON with audio_base64, duration, text transcript, or voice analysis results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `engine` | string | No | sensevoice | Engine: sensevoice/cosyvoice/fish-speech/edge-tts/piper/bark/whisper/vosk (default: sensevoice) |
| `operation` | string | No | tts | Operation type: tts/clone/emotion/multi-speaker/asr/transcribe/diarize/voice-analyze (default: tts) |
| `text` | string | No |  | Text to convert to speech (max 4000 chars) |
| `voice` | string | No | default | Voice name (default: default) |
| `style` | string | No | natural | Voice style: natural/professional/friendly/excited/calm/storytelling (default: natural) |
| `speed` | float | No | 1.0 | Speech speed 0.5-2.0 (default: 1.0) |
| `pitch` | float | No | 1.0 | Speech pitch 0.5-2.0 (default: 1.0) |
| `reference_audio` | string | No |  | Reference audio base64 for voice cloning |
| `output_format` | string | No | mp3 | Output format: mp3/wav/ogg (default: mp3) |
| `audio_input` | string | No |  | Audio base64 input for ASR/transcription/diarization |
| `language` | string | No | auto | Language for ASR: zh/zh-CN/zh-TW/en/en-US/ja/ko/fr/de/es/auto (default: auto) |
| `model_size` | string | No | base | Whisper model size: tiny/base/small/medium/large (default: base) |
| `enable_diarization` | bool | No | false | Enable speaker diarization (default: false) |
| `enable_timestamps` | bool | No | false | Include word-level timestamps (default: false) |
| `creator_mode` | string | No | podcast | Creator mode for create operation: podcast/audio-book/narration/jingles/ad/education (default: podcast) |
| `background_music` | bool | No | false | Enable background music (default: false) |
| `intro` | bool | No | false | Include intro/outro (default: false) |
| `chapter_count` | int | No | 1 | Number of chapters for audio-book (default: 1) |
| `host_voice` | string | No | default | Host/narrator voice for podcast (default: default) |

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

