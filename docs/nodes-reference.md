# Node Reference

> Auto-generated from `Schema()` metadata. 103 nodes registered.

| Node | Description | Params |
|------|-------------|--------|
| [`agent`](#agent) | Autonomous agent node with ReAct reasoning loop and tool use capabilities | 9 |
| [`agent_browser`](#agent_browser) | Agent-optimized web browser for autonomous web navigation, content extraction, and research. Inspired by CitroLabs/eg... | 7 |
| [`agent_inbox`](#agent_inbox) | Query agent message inbox. Retrieve and manage cross-domain messages. | 4 |
| [`agent_message`](#agent_message) | Send cross-domain message to another agent by DID. Inspired by awiki.ai agent-native messaging. | 5 |
| [`andesgpt`](#andesgpt) | OPPO AndesGPT large model integration. Supports Tiny (端侧1B), Turbo (端云协同7B), Titan (云端100B+) sizes wi... | 9 |
| [`anthropic`](#anthropic) | Call Anthropic LLM API | 4 |
| [`antling`](#antling) | 蚂蚁百灵（Ant Ling）大模型集成。支持Ling-2.6通用系列、Ring-2.6推理系列、Ming全模态系列�... | 8 |
| [`app_launch`](#app_launch) | Launch a mobile/desktop app with optional parameters. Cross-platform app control for AI systems. Supports Android, iO... | 7 |
| [`apply_patch`](#apply_patch) | 解析并应用 unified diff 格式补丁到文件。原子语义：全部校验通过后才应用。兼容 Codex 的... | 2 |
| [`baichuan`](#baichuan) | Call Baichuan LLM API | 4 |
| [`blockchain_audit`](#blockchain_audit) | Record workflow execution on blockchain for tamper-proof audit trails. Supports Ethereum, Hyperledger Fabric, and sim... | 7 |
| [`call`](#call) | Call another workflow file | 2 |
| [`clarify`](#clarify) | Pre-execution ambiguity checker: identifies unclear requirements and generates clarifying questions (ACQUIRE framework) | 8 |
| [`cli_session`](#cli_session) | 交互式CLI会话节点。支持上下文保持、命令历史、快捷键、流式输出和自动补全，提供类... | 5 |
| [`code_graph`](#code_graph) | Parse source code files to extract function definitions, call relationships, and import dependencies, then output a c... | 8 |
| [`code_interpreter`](#code_interpreter) | Execute Python/Node.js/Rust code in a sandboxed environment with file I/O | 6 |
| [`code_knowledge_graph`](#code_knowledge_graph) | Semantic code knowledge graph with vector retrieval, 158 language support, MCP tool exposure, and token-efficient rev... | 13 |
| [`code_review`](#code_review) | Hybrid code review combining deterministic rule engine (NPE, thread-safety, security) with LLM deep analysis. Inspire... | 11 |
| [`combine`](#combine) | Combine multiple inputs into one | 1 |
| [`compress`](#compress) | Intelligent context compression with 6 algorithms: extractive, keyword, cluster, sliding_window, hybrid (headroom-ins... | 7 |
| [`condition`](#condition) | Evaluate conditional expressions (contains, equals, regex, empty) | 2 |
| [`coze`](#coze) | WIP - Call ByteDance Coze API (not functional, API compatibility issues) | 4 |
| [`critic`](#critic) | Critic agent that reviews output, identifies issues, and suggests improvements | 8 |
| [`cross_app_action`](#cross_app_action) | Execute actions across multiple apps. Multi-app workflows for AI assistants. | 3 |
| [`deepseek`](#deepseek) | Call DeepSeek LLM API | 4 |
| [`device_state`](#device_state) | Query device state: battery, network, location, apps, storage. Context awareness for AI assistants. | 1 |
| [`doc_gen`](#doc_gen) | AI自动文档生成节点。自动生成和更新代码库文档，支持README、API文档、函数注释、模块�... | 6 |
| [`engineer_skills`](#engineer_skills) | Pre-built engineering skill package with TypeScript/React/Node.js expertise. Supports skill matching, application, an... | 5 |
| [`evaluator`](#evaluator) | Evaluator agent that scores output against criteria with structured rubrics | 9 |
| [`execute`](#execute) | Execute shell commands (disabled in safe mode) | 3 |
| [`fastgpt`](#fastgpt) | Call FastGPT API | 5 |
| [`fetch_url`](#fetch_url) | Fetch content from a URL | 3 |
| [`file_read`](#file_read) | Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set... | 2 |
| [`file_watch`](#file_watch) | Polls a file or directory for create/modify/delete events and returns them as JSON. Suitable for log-monitor and file... | 6 |
| [`file_write`](#file_write) | Write content to a file | 2 |
| [`gemini`](#gemini) | Call Google Gemini LLM API | 4 |
| [`glm`](#glm) | Call GLM LLM API | 4 |
| [`glob`](#glob) | 递归匹配文件路径，返回匹配的文件列表（每行一个）。兼容 Codex/OpenCode 的 glob 工具。 | 2 |
| [`grep`](#grep) | 递归搜索文件内容，返回匹配行（格式：file:line:content）。兼容 Codex/OpenCode 的 grep 工具。 | 5 |
| [`harmony_ability`](#harmony_ability) | Launch HarmonyOS Ability with specified type. Supports page (UI), slice (partial UI), service (background), data (dat... | 5 |
| [`harmony_atomic_service`](#harmony_atomic_service) | Launch HarmonyOS Atomic Service. Lightweight, card-based services that run without installation. Supports launch, rou... | 4 |
| [`harmony_device_adapt`](#harmony_device_adapt) | Detect HarmonyOS device type (phone/tablet/foldable/TV/car/wearable) and generate UI adaptation guidance. Inspired by... | 6 |
| [`harmony_widget`](#harmony_widget) | Manage HarmonyOS desktop widgets (service cards). Add, update, remove, or query widget state on the home screen. | 5 |
| [`http_request`](#http_request) | Make HTTP requests with custom method, headers, and body | 5 |
| [`human_in_loop`](#human_in_loop) | Human approval gate — pauses workflow for human review and approval before continuing | 5 |
| [`ima`](#ima) | Call IMA Copilot API | 4 |
| [`inference`](#inference) | Multi-backend local inference engine with unified interface across llama.cpp, ONNX, TensorRT, vLLM, MLC-LLM, NCNN, MN... | 6 |
| [`intent_router`](#intent_router) | Route user intents to appropriate handlers. Central dispatch for AI assistant commands. | 3 |
| [`internlm`](#internlm) | Call InternLM LLM API | 4 |
| [`json_parse`](#json_parse) | Parse and extract JSON data | 1 |
| [`kimi`](#kimi) | Call Kimi LLM API | 4 |
| [`knowledge_graph`](#knowledge_graph) | Knowledge graph node - extract entities/relations, build graph, query and traverse | 6 |
| [`list_dir`](#list_dir) | 列出目录内容，返回 name/type/size 列表。兼容 Codex 的 list_dir 工具。 | 3 |
| [`llm_router`](#llm_router) | Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization | 5 |
| [`mcp_bridge`](#mcp_bridge) | MCP（Model Context Protocol）协议桥接节点。支持工具调用和资源访问，提供标准化的MCP协议�... | 5 |
| [`mcp_server`](#mcp_server) | MCP服务器模式节点。让llm-box作为MCP服务器被其他Agent调用，支持HTTP/WebSocket协议，提供工�... | 6 |
| [`memory`](#memory) | AI Agent memory infrastructure with session-isolated persistent knowledge graph engine. Supports multi-session parall... | 13 |
| [`meta_orchestrator`](#meta_orchestrator) | Multi-model meta orchestrator with unified model routing and hierarchical agent network. Supports 22+ models across O... | 5 |
| [`mimo`](#mimo) | Call MiMo LLM API | 4 |
| [`minimax`](#minimax) | Call MiniMax LLM API | 4 |
| [`mistral`](#mistral) | Call Mistral LLM API | 4 |
| [`moe_streaming`](#moe_streaming) | MoE (Mixture of Experts) streaming expert loading for running large models on consumer hardware | 7 |
| [`multimodal`](#multimodal) | Multimodal node for image analysis, OCR, and audio transcription using vision-capable LLMs | 11 |
| [`node_marketplace`](#node_marketplace) | Node marketplace - list, search, and categorize available workflow nodes | 4 |
| [`notify`](#notify) | Send notifications (stdout, stderr, slack, discord, telegram, webhook) | 10 |
| [`ollama`](#ollama) | Call Ollama local LLM server | 3 |
| [`omniroute`](#omniroute) | AI gateway unified layer. Single endpoint access to 268+ providers, 500+ models. Supports Claude Code, Cursor, Cline,... | 10 |
| [`ondevice_llm`](#ondevice_llm) | Run LLM inference locally on the device (no cloud required). Supports 1B-8B models with INT4/INT8 quantization, inclu... | 10 |
| [`openai`](#openai) | Call OpenAI API | 4 |
| [`output_quality`](#output_quality) | Analyze output text for AI-generated traces and compute naturalness scores. Inspired by Nutlope/hallmark (57 anti-AI-... | 4 |
| [`pipeline`](#pipeline) | Dependency-based parallel workflow executor: steps run as soon as their dependencies are met, no global barriers (Tun... | 2 |
| [`planner`](#planner) | Task decomposition agent that breaks complex goals into actionable steps | 9 |
| [`plugin_system`](#plugin_system) | 插件系统节点。支持从本地目录、Git仓库、URL和插件市场加载插件，提供安装/卸载/更新/... | 4 |
| [`power_manager`](#power_manager) | Control power consumption for on-device AI. Supports eco/balanced/high profiles with adaptive battery and thermal man... | 7 |
| [`preference`](#preference) | User preference memory: store, retrieve, and learn user habits across sessions (MemSlides-inspired user profiling) | 7 |
| [`quality_guard`](#quality_guard) | AI content quality guard with detection, assessment, and enhancement capabilities. Identifies low-quality AI-generate... | 5 |
| [`qwen`](#qwen) | Call Qwen LLM API | 4 |
| [`rag`](#rag) | Retrieval Augmented Generation node - chunk documents, search by query, and assemble context | 7 |
| [`reflector`](#reflector) | Self-reflection agent that critiques output and iteratively improves it (Reflexion pattern) | 7 |
| [`researcher`](#researcher) | Research agent that fetches information from URLs and summarizes findings | 7 |
| [`robot_control`](#robot_control) | Plan and execute robot action sequences for embodied AI. Supports humanoid robots, mobile bases, robotic arms, drones... | 10 |
| [`router`](#router) | Classification agent that analyzes input and decides which processing path to take | 6 |
| [`screen_understanding`](#screen_understanding) | Understand screen content like an L3 agent: parse UI elements, identify actionable items, and generate interaction pl... | 5 |
| [`search_aggregate`](#search_aggregate) | Multi-platform search aggregator with real-signal ranking: Reddit/Twitter/YouTube/HN/GitHub, sorted by votes/comments... | 8 |
| [`self_heal`](#self_heal) | Self-diagnose and attempt automatic repair of project issues: build errors, formatting, missing deps, test failures, ... | 2 |
| [`send_notification`](#send_notification) | Send system notification with actions. Cross-platform notification for AI assistants. | 5 |
| [`sensenova`](#sensenova) | 商汤SenseNova日日新大模型集成。支持U1系列多模态模型（U1-Lite/U1-Pro）、Flash系列推理模型... | 9 |
| [`skill_distill`](#skill_distill) | Distill methodologies from books, videos, podcasts, and documents into callable skills. Supports workflow, decision, ... | 6 |
| [`skill_explorer`](#skill_explorer) | Discover, evaluate, and recommend skills from the ecosystem. Quality scoring, category browsing, and smart recommenda... | 5 |
| [`smart_router`](#smart_router) | Smart router that selects the best model/provider based on task analysis | 6 |
| [`supervisor`](#supervisor) | Advanced supervisor with MoE routing, MindSearch deep research, 232+ domain specialists, and collaboration templates | 13 |
| [`swarm_comm`](#swarm_comm) | Decentralized multi-agent swarm communication system with channels, agent registration, and message broadcasting. Ins... | 8 |
| [`system_event`](#system_event) | Listen for mobile system events (notification, call, SMS, location, battery, etc.) and trigger workflows | 7 |
| [`template_render`](#template_render) | Render Go templates with input data | 2 |
| [`test_node`](#test_node) | Test node for development purposes | 1 |
| [`transform`](#transform) | Transform text using string operations | 1 |
| [`ui_automate`](#ui_automate) | Automate UI interactions: click, type, scroll, swipe. Accessibility-based automation for AI assistants. | 7 |
| [`verify`](#verify) | Agent-as-a-Judge verifier that validates outputs, claims, and results against specified criteria | 10 |
| [`video_edit`](#video_edit) | AI-powered video editing workflow with smart cutting, merging, effects, subtitle generation, and storyboard creation. | 7 |
| [`voice_input`](#voice_input) | Voice input pipeline: VAD (Voice Activity Detection), wake word detection, and speech-to-text. Supports on-device rec... | 7 |
| [`voice_output`](#voice_output) | Voice AI toolchain with TTS, voice cloning, ASR speech recognition, transcription, diarization, and voice analysis. S... | 19 |
| [`xverse`](#xverse) | Call XVERSE LLM API | 4 |
| [`yi`](#yi) | Call Yi LLM API | 4 |

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
| `action` | string | No | visit | Browser action: visit|extract|links|screenshot|search|summary (default: visit) |
| `url` | string | No |  | Target URL (overrides input if provided) |
| `selector` | string | No |  | CSS selector for content extraction (optional) |
| `max_depth` | string | No | 1 | Maximum link follow depth for crawling (default: 1) |
| `output_format` | string | No | markdown | Output format: markdown|text|json|html (default: markdown) |
| `summary_length` | string | No | 2000 | Maximum summary length in characters (default: 2000) |
| `render_js` | string | No | false | Enable JavaScript rendering (default: false) |

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

## apply_patch

解析并应用 unified diff 格式补丁到文件。原子语义：全部校验通过后才应用。兼容 Codex 的 apply_patch 工具。

- **Input**: string - unified diff 补丁内容（也可通过 patch 参数传入）
- **Output**: string - 应用结果摘要

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `patch` | string | No |  | unified diff 补丁内容（与 input 二选一） |
| `backup` | string | No | true | 是否在应用前备份原文件到 .bak：true/false（默认 true） |

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
| `model` | string | No | auto | 使用的模型（默认auto，由meta_orchestrator选择） |
| `session_id` | string | No |  | 会话ID（自动生成或指定） |
| `max_history` | int | No | 50 | 最大历史记录数（默认50） |
| `streaming` | bool | No | true | 流式输出（默认true） |
| `theme` | string | No | dark | 主题（light/dark，默认dark） |

---

## code_graph

Parse source code files to extract function definitions, call relationships, and import dependencies, then output a code graph (JSON or Mermaid). Supports Go, Python, JavaScript, TypeScript. Features: persistent cache, incremental updates.

- **Input**: string - not used
- **Output**: string - code graph in JSON or Mermaid format

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | File or directory path to analyze (relative to working directory) |
| `language` | string | No | auto | Code language: go/python/javascript/typescript/auto (default: auto) |
| `output_format` | string | No | json | Output format: json or mermaid (default: json) |
| `max_files` | string | No | 100 | Max number of files to analyze (default: 100) |
| `max_file_size` | string | No | 1048576 | Max single file size in bytes (default: 1048576 = 1MB) |
| `use_cache` | string | No | true | Use persistent cache for incremental updates (default: true) |
| `refresh` | string | No | false | Force refresh cache, ignore existing cache (default: false) |
| `save_cache` | string | No | true | Save results to persistent cache (default: true) |

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

## coze

WIP - Call ByteDance Coze API (not functional, API compatibility issues)

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | Yes |  | Model name (required) |
| `api_key` | string | No |  | Coze API key (or set COZE_API_KEY env var) |
| `endpoint` | string | No | https://api.coze.cn/v1 | API base URL (default: https://api.coze.cn/v1) |
| `system` | string | No |  | System prompt |

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

---

## glob

递归匹配文件路径，返回匹配的文件列表（每行一个）。兼容 Codex/OpenCode 的 glob 工具。

- **Input**: string - 未使用
- **Output**: string - 匹配的文件路径列表（每行一个，相对搜索根目录）

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `pattern` | string | Yes |  | glob 模式，如 **/*.go 或 *.md |
| `path` | string | No | . | 搜索根目录（默认工作目录） |

---

## grep

递归搜索文件内容，返回匹配行（格式：file:line:content）。兼容 Codex/OpenCode 的 grep 工具。

- **Input**: string - 未使用
- **Output**: string - 匹配行列表（每行一条：file:line:content）

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `pattern` | string | Yes |  | 正则表达式 |
| `path` | string | No | . | 搜索目录（默认工作目录） |
| `glob` | string | No |  | 文件名过滤，如 *.go |
| `ignore_case` | string | No | false | 是否忽略大小写：true/false（默认 false） |
| `max_matches` | string | No | 1000 | 最大匹配数（默认 1000） |

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

---

## human_in_loop

Human approval gate — pauses workflow for human review and approval before continuing

- **Input**: string - the content/data to present for human review
- **Output**: string - approved content (or original if approved)

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `mode` | string | No | file | Approval mode: file, env, stdin, auto_approve (default: file) |
| `approval_file` | string | No | .llm-box-approval | Path to approval flag file (mode=file) |
| `approval_env` | string | No | LLM_BOX_APPROVED | Environment variable to check for approval (mode=env) |
| `prompt` | string | No |  | Custom prompt message for the human reviewer |
| `on_approve` | string | No | original | What to output on approve: original, modified, passthrough (default: original) |

---

## ima

Call IMA Copilot API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | Yes |  | Model name (required) |
| `api_key` | string | No |  | IMA API key (or set IMA_API_KEY env var) |
| `endpoint` | string | Yes |  | API base URL (or set IMA_API_BASE env var) |
| `system` | string | No |  | System prompt |

---

## inference

Multi-backend local inference engine with unified interface across llama.cpp, ONNX, TensorRT, vLLM, MLC-LLM, NCNN, MNN (T-Head SAIL-inspired)

- **Input**: string - prompt text for inference
- **Output**: string - inference result text

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | infer | infer|list|status|set_backend|load_model (default: infer) |
| `backend` | string | No | llama.cpp | Backend: llama.cpp|onnx|tensorrt|vllm|mlc-llm|ncnn|mnn|ollama (default: llama.cpp) |
| `model` | string | No |  | Model name or path |
| `max_tokens` | string | No | 512 | Max tokens to generate (default: 512) |
| `temperature` | string | No | 0.7 | Temperature 0-1 (default: 0.7) |
| `model_path` | string | No |  | Path to model file for load_model |

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
| `model` | string | No | moonshot-v1-8k | Model name (default: moonshot-v1-8k) |
| `api_key` | string | No |  | Kimi API key (or set KIMI_API_KEY env var) |
| `endpoint` | string | No | https://api.moonshot.cn/v1 | API base URL (default: https://api.moonshot.cn/v1) |
| `system` | string | No |  | System prompt |

---

## knowledge_graph

Knowledge graph node - extract entities/relations, build graph, query and traverse

- **Input**: string - text to extract knowledge from, or a query for an existing graph
- **Output**: string - knowledge graph data or query results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | extract | Action: extract, query, traverse, stats, visualize (default: extract) |
| `graph_path` | string | No |  | Path to save/load the graph JSON file |
| `query` | string | No |  | Query for search/traverse (entity name or relation type) |
| `max_depth` | string | No | 2 | Max traversal depth (default: 2) |
| `top_k` | string | No | 10 | Max results to return (default: 10) |
| `format` | string | No | markdown | Output format: json, markdown, mermaid (default: markdown) |

---

## list_dir

列出目录内容，返回 name/type/size 列表。兼容 Codex 的 list_dir 工具。

- **Input**: string - 未使用
- **Output**: string - 目录条目列表（每行一条：relpath	type	size）

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `path` | string | Yes |  | 目录路径 |
| `recursive` | string | No | false | 是否递归：true/false（默认 false） |
| `max_entries` | string | No | 1000 | 最大条目数（默认 1000） |

---

## llm_router

Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization

- **Input**: string - user message content to send to LLM
- **Output**: string - AI response from the selected provider

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `system` | string | No |  | System prompt for the LLM |
| `strategy` | string | No | priority | Routing strategy: priority, cost, latency, round_robin, random (default: priority) |
| `max_retries` | string | No | 3 | Maximum number of fallback attempts (default: 3) |
| `show_provider` | string | No | false | Show which provider was used in output (default: false) |
| `show_stats` | string | No | false | Show router statistics in output (default: false) |

---

## mcp_bridge

MCP（Model Context Protocol）协议桥接节点。支持工具调用和资源访问，提供标准化的MCP协议接口，包括工具列表、工具调用、资源列表、资源读取、提示词管理和服务器信息等功能。

- **Input**: string - MCP请求内容或指令
- **Output**: string - JSON格式的MCP响应结果

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | Yes |  | 操作类型：tools_list/tools_call/resources_list/resources_read/prompts_list/prompts_get/server_info |
| `tool_name` | string | No |  | 工具名称（调用工具时使用） |
| `tool_args` | string | No |  | 工具参数（JSON字符串，调用工具时使用） |
| `resource_uri` | string | No |  | 资源URI（读取资源时使用） |
| `server_url` | string | No |  | MCP服务器地址（可选） |

---

## mcp_server

MCP服务器模式节点。让llm-box作为MCP服务器被其他Agent调用，支持HTTP/WebSocket协议，提供工具暴露、会话管理和权限控制功能，兼容标准MCP协议。

- **Input**: string - MCP服务器相关输入（可选）
- **Output**: string - JSON格式的服务器操作结果

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | Yes |  | 操作：start/stop/status/restart |
| `port` | int | No | 8080 | 端口（默认8080，范围1024-65535） |
| `protocol` | string | No | http | 协议：http/websocket |
| `host` | string | No | 0.0.0.0 | 主机地址（默认0.0.0.0） |
| `expose_tools` | string | No |  | 要暴露的工具列表（逗号分隔） |
| `auth_token` | string | No |  | 认证token（可选，长度32-256） |

---

## memory

AI Agent memory infrastructure with session-isolated persistent knowledge graph engine. Supports multi-session parallel memory, short/medium/long term memory, cross-session long-term memory, and memory usage monitoring.

- **Input**: string - memory content to store or query for retrieval
- **Output**: string - JSON with memory operations result, entries, or statistics

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `operation` | string | No | store | Operation: store/retrieve/delete/search/summary/forget/transfer/merge/inkling_retrieve/list_sessions/session_stats/global_stats (default: store) |
| `session_id` | string | No | default | Session ID for isolated memory (default: default) |
| `key` | string | No |  | Memory key for storage/retrieval |
| `value` | string | No |  | Memory value/content |
| `level` | string | No | medium | Memory level: short/medium/long (default: medium) |
| `type` | string | No | fact | Memory type: fact/concept/experience/preference/relationship/task/context (default: fact) |
| `tags` | string | No |  | Comma-separated tags for categorization |
| `ttl_hours` | int | No | 72 | Time to live in hours (default: 72) |
| `confidence` | float | No | 0.8 | Confidence level 0.0-1.0 (default: 0.8) |
| `query` | string | No |  | Search query for retrieval/search operations |
| `top_k` | int | No | 10 | Number of results to return (1-100, default: 10) |
| `threshold` | float | No | 0.5 | Similarity threshold 0.0-1.0 (default: 0.5) |
| `source` | string | No |  | Source identifier for the memory |

---

## meta_orchestrator

Multi-model meta orchestrator with unified model routing and hierarchical agent network. Supports 22+ models across OpenAI, Anthropic, Google, AndesGPT, SenseNova, Ant Ling, and domestic providers.

- **Input**: string - the task or prompt to process
- **Output**: json - selected model, routing strategy, task type, hierarchy level, response, usage, latency_ms

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No |  | Model name (optional, overrides routing). Supported: gpt-4o, gpt-4, gpt-3.5-turbo, claude-3-opus, claude-3-sonnet, claude-3-haiku, gemini-pro, gemini-flash, andesgpt-tiny, andesgpt-turbo, andesgpt-titan, sensenova-flash-lite, sensenova-flash, sensenova-u1-lite, sensenova-u1-pro, deepseek-v2, qwen-max, ernie-4, ling-2.6-flash, ling-2.6-1t, ring-2.6-1t, ming-flash-omni-2.0 |
| `routing_strategy` | string | No | auto | Routing strategy: auto/fastest/cheapest/best_quality/privacy_first (default: auto) |
| `task_type` | string | No | analysis | Task type: code/writing/analysis/creative/data (default: analysis) |
| `max_depth` | int | No | 3 | Max hierarchy depth 1-5 (default: 3) |
| `use_hierarchy` | bool | No | true | Enable hierarchical agent network (default: true) |

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
| `audio_path` | string | No |  | Path to audio file for transcription |
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

## omniroute

AI gateway unified layer. Single endpoint access to 268+ providers, 500+ models. Supports Claude Code, Cursor, Cline, and llm-box. Auto-routes based on speed, cost, quality, or availability.

- **Input**: string - user prompt or task description
- **Output**: string - JSON with selected provider, model, response, usage, and routing info

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No |  | Target provider (optional, auto-selected if not specified) |
| `model` | string | No |  | Target model (optional, auto-selected if not specified) |
| `tool` | string | No | llm_box | Target tool: claude_code/cursor/cline/llm_box (default: llm_box) |
| `strategy` | string | No | auto | Routing strategy: auto/fastest/cheapest/best_quality/availability/custom_fallback (default: auto) |
| `api_key` | string | No |  | API key for selected provider |
| `base_url` | string | No |  | Custom API base URL |
| `max_tokens` | int | No | 2048 | Max output tokens (default: 2048) |
| `temperature` | float | No | 0.7 | Sampling temperature 0.0-2.0 (default: 0.7) |
| `fallback_providers` | string | No |  | Comma-separated fallback providers for custom_fallback strategy |
| `region` | string | No |  | Region for cloud providers (e.g., us-east-1) |

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

Call OpenAI API

- **Input**: string - user message content
- **Output**: string - AI response content

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `model` | string | No | gpt-3.5-turbo | Model name (default: gpt-3.5-turbo) |
| `api_key` | string | No |  | OpenAI API key (or set OPENAI_API_KEY env var) |
| `endpoint` | string | No | https://api.openai.com/v1 | API base URL (or set OPENAI_API_BASE env var) |
| `system` | string | No |  | System prompt |

---

## output_quality

Analyze output text for AI-generated traces and compute naturalness scores. Inspired by Nutlope/hallmark (57 anti-AI-taste detection checks). Detects template phrases, robotic structure, and generic content. Provides rewrite suggestions.

- **Input**: string - the text to analyze for AI traces and quality
- **Output**: string - quality report with scores, detected issues, and suggestions

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | analyze | Action: analyze|gate|suggest|checklist (default: analyze) |
| `min_score` | string | No | 60 | Minimum pass score 0-100 for gate action (default: 60) |
| `detail` | string | No | full | Detail level: brief|full (default: full) |
| `lang` | string | No | auto | Language hint: auto|zh|en (default: auto) |

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

## quality_guard

AI content quality guard with detection, assessment, and enhancement capabilities. Identifies low-quality AI-generated content and provides quality scoring.

- **Input**: string - content to assess
- **Output**: string - JSON with quality assessment results

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `content` | string | No |  | Content to assess (max 20000 chars) |
| `assessment_type` | string | No | overall | Assessment type: ai_detection/design_quality/code_quality/writing_quality/overall (default: overall) |
| `quality_threshold` | float | No | 0.7 | Quality threshold 0.0-1.0 (default: 0.7) |
| `auto_fix` | bool | No | false | Auto fix low-quality content (default: false) |
| `verbose` | bool | No | false | Detailed report (default: false) |

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

## router

Classification agent that analyzes input and decides which processing path to take

- **Input**: string - the input text to classify and route
- **Output**: string - JSON with classification result and routing decision

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `provider` | string | No | ollama | LLM provider (default: ollama) |
| `model` | string | No | llama3 | Model name (default: llama3) |
| `api_key` | string | No |  | API key |
| `endpoint` | string | No |  | API endpoint URL |
| `categories` | string | Yes |  | Comma-separated list of routing categories (e.g. bug,feature,question,spam) |
| `instructions` | string | No |  | Additional routing instructions or classification criteria |

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

## self_heal

Self-diagnose and attempt automatic repair of project issues: build errors, formatting, missing deps, test failures, version mismatches. Runs gofmt/go vet/go build/go test and auto-fixes where possible (xiaobei inspired autonomous repair mechanism).

- **Input**: string - optional: specific area to check (build|format|deps|test|all)
- **Output**: string - JSON or formatted heal report

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `auto_fix` | string | No | true | true|false, attempt automatic fixes (default: true) |
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

## smart_router

Smart router that selects the best model/provider based on task analysis

- **Input**: string - the task or query to analyze and route
- **Output**: string - the response from the selected model

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `fast_model` | string | No | ollama:llama3 | Fast/cheap model for simple tasks (default: ollama:llama3) |
| `medium_model` | string | No | ollama:llama3 | Medium model for average tasks (default: ollama:llama3) |
| `strong_model` | string | No | openai:gpt-4o | Strong model for complex tasks (default: openai:gpt-4o) |
| `system_prompt` | string | No |  | System prompt for the selected model |
| `show_routing` | string | No | false | Show routing decision in output (default: false) |
| `force_tier` | string | No |  | Force a specific tier: fast, medium, strong (optional) |

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

## swarm_comm

Decentralized multi-agent swarm communication system with channels, agent registration, and message broadcasting. Inspired by block/buzz (Nostr protocol) for human-AI collaborative spaces.

- **Input**: string - message content or command parameters
- **Output**: string - communication results, channel messages, or agent status

### Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `action` | string | No | read | Action: join|leave|send|read|list_channels|list_agents|create_channel|broadcast (default: read) |
| `agent_id` | string | No |  | Agent identifier for join/send actions |
| `agent_name` | string | No |  | Agent display name for registration |
| `agent_role` | string | No | agent | Agent role: researcher|developer|coordinator|reviewer|custom (default: agent) |
| `channel` | string | No | general | Target channel: general|tasks|research|announcements (default: general) |
| `message_type` | string | No | text | Message type: text|task|result|status|emergency (default: text) |
| `to_agent` | string | No |  | Direct message target agent ID (optional) |
| `limit` | string | No | 50 | Maximum messages to retrieve (default: 50) |

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

---

