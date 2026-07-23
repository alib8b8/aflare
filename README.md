<div align="center">
  <h1>llm-box</h1>
  <p>🌍
    <strong>中文</strong> ·
    <a href="README.en.md">English</a> ·
    <a href="README.ru.md">Русский</a>
  </p>
  <p><strong>将自然语言转化为可执行工作流</strong></p>
  <p>面向终端的 Agent 工作流引擎 —— 确定性执行与 AI 智能体相结合。构建具备自主 Agent 节点、工具调用和多步推理能力的自驱动工作流。</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/llm-box/ci.yml?branch=main&style=flat-square&label=CI" alt="CI 状态" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="发布版本" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="许可证" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/release.yml">
      <img src="https://github.com/alib8b8/llm-box/actions/workflows/release.yml/badge.svg" alt="发布状态" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://img.shields.io/badge/AtomGit-GitCode-green?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0iIzI1MjUyNSIgZD0iTTIyIDJoLTJWMGgydi0yaDJ2MmgydjItMmgydjItMmgydjJ6bTAgMTZIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6bTAgLThIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6Ii8+PC9zdmc+" alt="GitCode" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/star/new_badge.svg" alt="AtomGit G-Star" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/star/badge.svg" alt="AtomGit Star" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/fork/badge.svg" alt="AtomGit Fork" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/download/badge.svg" alt="AtomGit Download" />
    </a>
  </p>

</div>

---

## 📋 目录

- [🚀 快速开始](#-快速开始)
- [✨ 核心特性](#-核心特性)
- [🤖 Agent 节点](#-agent-节点)
- [📱 鸿蒙 & 移动端节点](#-鸿蒙--移动端节点)
- [🔒 安全](#-安全)
- [🌐 生态](#-生态)
- [📚 文档](#-文档)
- [🛠️ CLI 命令](#️-cli-命令)
- [🏗️ 架构](#️-架构)
- [🗺️ 路线图](#️-路线图)
- [🤝 贡献指南](#-贡献指南)
- [📄 许可证](#-许可证)

---

## 🚀 快速开始

**一行命令安装：**

| macOS | Linux | Windows |
|-------|-------|---------|
| `brew install alib8b8/tap/llm-box` | `curl -fsSL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**🌏 中国用户 — 使用镜像加速下载：**

| macOS/Linux | Windows |
|-------------|---------|
| `curl -fsSL https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**或从 [GitCode Releases](https://gitcode.com/llm-box/llm-box/releases) / [GitHub Releases](https://github.com/alib8b8/llm-box/releases) 下载**

📖 [交互式下载页面 →](docs/download.html)

---

### 创建并运行你的第一个工作流

```bash
# 从自然语言生成工作流
llm-box create "总结今天的 AI 新闻"

# 或使用内置模板
llm-box create --template research-assistant

# 运行
llm-box run ai-news-summary.yaml
```

📖 [完整入门指南 →](docs/getting-started.md)

---

## ✨ 核心特性

| 类别 | 特性 |
|------|------|
| **工作流生成** | 自然语言 → YAML，基于关键词匹配，20+ 分类 100+ 内置模板 |
| **Agent 节点** | **35+** AI Agent 节点，支持 ReAct、思维链、工具调用和自主推理 |
| **边缘 AI 引擎** | ReAct 推理循环、三层持久化记忆（短期/工作/长期）、本地/云端模型路由、隐私分析器 |
| **技能自进化** | Agent 技能随使用自我提升 —— 自动追踪成功率、延迟、最佳实践、已知陷阱；自动优化提示词 |
| **鸿蒙适配** | 能力启动、原子服务、桌面卡片、7 种设备类型适配（手机/折叠屏/平板/电视/车机/手表） |
| **手机内置 AI** | 系统事件监听（通知/来电/短信/位置/电量）、端侧大模型推理（1B-8B，INT4/INT8，含 SenseNova U1）、自适应电源管理（节能/均衡/高性能，电池/温度感知）、屏幕理解、语音输入（VAD+唤醒+ASR）、语音输出（TTS+声音克隆） |
| **WAIC 对齐** | 面向 Agent 互操作性的区块链审计追踪、具身智能机器人控制（人形/机械臂/无人机）、L3 智能体手机能力 |
| **跨平台协议** | `intent://` 和 `ohos://` URI 协议、W3C DID 身份验证、跨域 Agent 消息 |
| **昇腾 NPU 适配** | 7-Agent 流水线（搜索→验证→适配→量化→优化→部署→文档）、CANN/MindIE/MindStudio 集成、INT8/FP8 量化、1 小时自动适配 |
| **代码智能** | 代码图谱（158 语言的 AST/调用图/依赖关系）、代码知识图谱（语义向量检索、实体/关系/概念提取、Token 优化）、Codex/OpenCode 兼容工具（glob/grep/list_dir/apply_patch） |
| **子代理架构** | 主/子代理提示词层级（17 个专家模板），借鉴 Grok Build 的 prompt.md + subagent_prompt.md 模式 |
| **分布式弹性** | 每节点熔断器（Closed/Open/HalfOpen 状态机）、故障工作节点自动隔离、熔断统计端点 |
| **隐私设计** | 文件读取时自动脱敏秘密（.env/密钥/令牌）、出站数据量监控与异常告警（防止 Grok-Build 式 27800 倍泄漏） |
| **文件监听** | 基于轮询的文件监听节点（创建/修改/删除事件），用于日志监控和文件整理工作流 |
| **TUI 渲染** | 终端 Markdown 渲染器（标题/代码/加粗/斜体/列表/引用/表格）+ Mermaid 转 ASCII 转换器（流程/时序图） |
| **元编排** | 多模型路由器（**22+ 模型**：OpenAI/Anthropic/Google/AndesGPT/SenseNova/**AntLing**/DeepSeek/Qwen）、5 种策略（自动/最快/最便宜/最佳质量/隐私优先）、层级 Agent 网络（主管→专家→工作者） |
| **MCP 协议** | MCP 桥接客户端（7 种操作、5 个内置工具）+ MCP 服务器模式（HTTP/WebSocket、工具暴露、会话管理、认证） |
| **质量守护** | 反 AI-slop 检测、5 种评估类型（AI 检测/设计/代码/写作/综合）、自动修复与质量阈值强制执行 |
| **工程师技能包** | 4 个领域 16 个预置技能：React/TypeScript/API/数据库/CI-CD/Docker/设计模式 |
| **技能蒸馏** | 从书籍/视频/播客/文章中提取方法论并转化为可调用技能：工作流/决策/分析/创意/提示词/检查清单 |
| **视频编辑** | AI 视频编辑：智能剪辑/合并/特效/字幕/故事板/超分、4 种风格、720p/1080p/4k |
| **AI 网关** | OmniRoute 统一层：**268+ 提供商**、6 种路由策略（自动/最快/最便宜/最佳质量/可用性/自定义降级）、健康检查机制、Claude Code/Cursor/Cline/llm-box 兼容、**Inkling MoE 原生支持** |
| **Agent 记忆** | 三层记忆基础设施（短期/中期/长期）：10 种操作（存储/检索/删除/搜索/摘要/遗忘/转移/合并/可视化/Inkling检索）、LRU 淘汰、跨会话长期记忆、**自动过期清理** |
| **语音 AI 工具链** | 完整语音工作室：TTS + 声音克隆 + **ASR 转录** + **说话人分离** + **语音分析** + 创作模式（播客/有声书/旁白/广告/教育）、11 种语言、5 种 ASR 引擎、**Inkling 多模态音频理解** |
| **Agent 团队化** | **232+ 个专业角色**覆盖 12+ 领域、Agency 工作流（8 阶段）、**19 个协作模板**、**Inkling MoE 专家团队** |
| **实用节点** | 40+ 内置节点：大模型提供商、获取、执行、转换、文件 I/O、JSON、通知、条件、组合、调用、模板 |
| **数据与知识** | RAG 检索、知识图谱提取/查询/遍历、智能模型路由器、多模态图像分析 |
| **代码与工具** | Python 代码解释器沙箱、节点市场、MCP 集成、插件系统 |
| **分布式执行** | 协调器/工作者架构、水平扩展、心跳监控、熔断器 |
| **调度** | 基于 Cron 的定时工作流、间隔触发器、CLI 管理 |
| **安全** | SSRF 防护、路径遍历防护、命令注入防御、AES-GCM 加密、审计日志、秘密脱敏、ANSI 注入防御、数据竞争修复、DoS 防护（订阅限制、日志轮转、内存上限）、**98+ 漏洞已审计** |
| **生态** | GitCode G-Star、鸿蒙 Agent 技能、ohpm SDK、OPPO X-OmniClaw 技能适配器、OPPO 小布技能、AndesGPT、SenseNova、**Ant Ling**、**Inkling / Thinking Machines** |
| **开发者体验** | Web UI 编辑器、工作流可视化器（Mermaid/JSON/DOT/ASCII）、支持 Markdown/Mermaid 的 TUI、9 种语言 |

---

## 🤖 Agent 节点

用于自主推理的专业 AI Agent 节点：

| 节点 | 描述 |
|------|-------------|
| `agent` | 通用 ReAct Agent，支持工具调用、思维链模式 |
| `planner` | 将任务拆解为分步计划 |
| `researcher` | 网络研究与信息收集 |
| `critic` | 评审并提供建设性反馈 |
| `evaluator` | 根据标准评估输出 |
| `reflector` | 反思过程并提出改进建议 |
| `supervisor` | 监管多 Agent 工作流，支持顺序/并行/层级/MoE/MindSearch/**Agency** 策略和 **232+ 领域专家**、**19 个协作模板** |
| `code_review` | 自动代码评审与建议 |
| `router` | 将输入路由到合适的处理器 |
| `human_in_loop` | 暂停等待人工审批 |
| `meta_orchestrator` | 多模型路由器，**22+ 模型**、5 种策略、层级 Agent 网络 |
| `code_knowledge_graph` | 语义代码知识图谱：158 种语言、向量检索、实体/关系/概念提取、**MCP 工具暴露**、**Token 高效审查**、**PR 分析**、**Inkling 代码审查** |
| `moe_streaming` | MoE 专家流式加载：消费级硬件运行 744B 模型，按需加载 |
| `cli_session` | 交互式终端会话，支持上下文持久化、流式输出、自动补全 |
| `plugin_system` | 插件扩展：从本地/git/网址/市场安装/卸载/更新，沙箱隔离 |
| `mcp_server` | MCP 服务器模式：通过 HTTP/WebSocket 暴露工具，支持会话管理和认证 |
| `mcp_bridge` | MCP 桥接客户端：7 种操作、5 个内置工具、协议兼容 |
| `quality_guard` | 反 AI-slop 检测：5 种评估类型、自动修复、质量阈值强制执行 |
| `engineer_skills` | 16 个预置技能：React/TypeScript/API/数据库/CI-CD/Docker/设计模式 |
| `skill_distill` | 从书籍/视频/播客中蒸馏方法论为可调用技能 |
| `voice_output` | 语音 AI 工具链：TTS + 声音克隆 + **ASR 转录** + **说话人分离** + **语音分析** + 创作模式、11 种语言、5 种 ASR 引擎、**Inkling 音频理解** |
| `doc_gen` | AI 文档生成：7 种类型（自述文件/API/函数/模块/变更日志/教程/架构） |
| `video_edit` | AI 视频编辑：智能剪辑/合并/特效/字幕/故事板/超分 |
| `omniroute` | **AI 网关统一层**：268+ 提供商、健康检查机制、6 种路由策略、Claude Code/Cursor/Cline 兼容、**Inkling MoE 支持** |
| `memory` | **Agent 记忆基础设施**：三层记忆（短期/中期/长期）、10 种操作、可视化、LRU 淘汰、自动清理、**Inkling 长上下文检索** |

### 鸿蒙 & 移动端节点

| 节点 | 描述 |
|------|-------------|
| `harmony_ability` | 启动鸿蒙 Ability（页面/切片/服务/数据），带类型验证 |
| `harmony_atomic_service` | 启动原子服务（基于卡片的轻量级应用，无需安装） |
| `harmony_widget` | 管理桌面卡片：添加、更新、删除、查询 |
| `harmony_device_adapt` | 检测 7 种设备类型（手机/折叠屏/平板/电视/车机/手表），生成 UI 适配方案 |
| `app_launch` | 启动移动应用（Android/iOS/鸿蒙），支持平台自动检测 |
| `ui_automate` | UI 自动化：点击、滚动、输入、滑动、截图（白名单验证的操作） |
| `cross_app_action` | 跨应用工作流：分享内容、稍后保存、比价 |
| `intent_router` | 将用户意图路由到合适的处理器，支持领域分类 |
| `device_state` | 查询设备状态：电量、网络、位置、应用、存储 |
| `agent_message` | 使用 W3C DID 身份在 Agent 之间发送跨域消息 |
| `agent_inbox` | 查询和管理 Agent 消息收件箱 |
| `system_event` | 监听移动系统事件：通知、来电、短信、位置、电量、闹钟、屏幕状态 |
| `ondevice_llm` | 设备端大模型推理（1B-7B 模型，INT4/INT8/FP16，llama.cpp/MLC-LLM/ONNX 后端） |
| `power_manager` | 自适应电源管理：节能/均衡/高性能模式，电池感知与温度感知节流 |
| `blockchain_audit` | 将工作流执行记录到区块链，实现防篡改审计追踪（以太坊/超级账本/模拟）。符合 WAIC Agent 互操作性倡议 |
| `screen_understanding` | L3 级屏幕内容理解：解析 UI 元素、识别可操作项、为 Agent 手机生成交互计划 |
| `voice_input` | 语音流水线：VAD、唤醒词检测（hey_box/hello_box/hi_box/ok_box/box_box）、支持端侧的语音转文本 |
| `robot_control` | 为具身 AI 规划和执行机器人动作序列：人形/移动底盘/机械臂/无人机/机器狗/轮椅，带安全检查 |
| `andesgpt` | OPPO AndesGPT 集成：Tiny（端侧 1B）/ Turbo（端云协同 7B）/ Titan（云端 100B+）、PersonaX 个性化、端云协同 |

### 昇腾 NPU 适配节点

| 节点 | 描述 |
|------|-------------|
| `ascend_model_search` | 在 AtomGit/昇腾模型库中搜索模型 |
| `ascend_model_verify` | 验证模型清单、依赖项、许可证、昇腾兼容性 |
| `ascend_model_adapt` | 通过 msTransplant 适配模型到昇腾 NPU，处理算子补丁 |
| `ascend_model_quantize` | 通过 msModelSlim 进行 INT8/FP8/W8A8 量化，含精度对比 |
| `ascend_model_optimize` | 通过 msProf/msprof-analyze 进行性能调优、瓶颈分析 |
| `ascend_model_deploy` | MindIE Service 部署、OpenAI API 兼容性测试 |
| `ascend_model_doc` | 自动生成基准测试报告和复现指南 |
| `ascend_model_agent` | 端到端编排器（模式：完整/快速/调优） |

### 代码智能与工具兼容节点

| 节点 | 描述 |
|------|-------------|
| `code_graph` | 提取代码结构（导入/函数/调用），支持 Go/Python/JS/TS，输出 JSON 或 Mermaid 图 |
| `file_watch` | 监听路径文件变化（创建/修改/删除），基于轮询，上下文感知 |
| `glob` | 递归文件通配匹配（`**/*.go`），深度限制，Codex 兼容 |
| `grep` | 递归内容搜索，支持正则表达式，跳过二进制文件，Codex/OpenCode 兼容 |
| `list_dir` | 列出目录内容（可选递归），Codex 兼容 |
| `apply_patch` | 原子化应用统一差异补丁（先验证再提交），Codex 兼容 |

---

## 🔒 安全

llm-box 认真对待安全问题。关键防护措施：

| 防护 | 实现 |
|------------|----------------|
| **SSRF 防护** | 自定义 `DialContext` 在连接时验证 IP（防止 DNS 重绑定） |
| **路径遍历** | 输入验证 + 符号链接解析，所有路径限制在工作目录内 |
| **命令注入** | 白名单模式阻止 Shell 元字符；安全模式完全禁用执行 |
| **秘密管理** | AES-GCM 加密，PBKDF2（60 万次迭代），文件权限 `0600` |
| **定时攻击** | 使用 `subtle.ConstantTimeCompare` 比较认证令牌 |
| **故障关闭认证** | 空令牌 = 请求被拒绝（503） |
| **审计日志** | 所有命令带脱敏秘密记录，权限 `0600` |
| **DID 身份** | W3C DID 格式验证、签名验证、跨域消息认证 |
| **内存安全** | 分层记忆 + LRU 淘汰、抗符号链接持久化、`0600` 文件权限 |
| **提示词注入** | 技能进化在注入提示词前对最佳实践/陷阱进行消毒 |
| **秘密脱敏** | 文件读取时自动检测并遮蔽 10+ 种秘密模式（AWS/GitHub/Slack/JWT/私钥）；`.env`/凭证默认完全遮蔽 |
| **出站监控** | 滑动窗口数据量监控 + 异常告警（防止 Grok-Build 式 27800 倍数据泄漏） |
| **熔断器** | 每工作者熔断器（Closed→Open→HalfOpen），故障节点自动隔离防止级联故障 |
| **原子补丁** | `apply_patch` 先验证再提交，使用临时暂存 + 原子重命名；失败时无部分写入 |
| **ANSI 注入** | TUI Markdown/Mermaid 渲染器从用户输入中剥离终端控制序列（CSI/OSC/DCS） |
| **工具可移植性** | Codex/OpenCode 兼容工具（glob/grep/list_dir/apply_patch），含完整路径/符号链接/DoS 加固 |
| **会话限制** | CLI 会话 24 小时后自动过期（最多 500 个），MCP 会话上限 1000 个并自动清理 |
| **插件限制** | 最多 100 个插件，仅 HTTPS 网址，限制 Git 主机（GitHub/GitLab/GitCode/Gitee） |
| **资源限制** | 代码知识图谱：最多 5000 个文件/深度 5；视频编辑：过滤 Shell 元字符 |
| **并发安全** | 每会话随机数互斥锁、共享状态使用 RWMutex、无锁不保护的全局可变状态 |
| **自动修复** | CI 自动运行 gofmt/go vet/gosec/govulncheck，发现问题自动提交修复 |

📖 [安全指南 →](SECURITY.md) | [审计日志 →](docs/getting-started.md#audit-logs)

---

## 🌐 生态

llm-box 参与多个开源生态：

| 生态 | 状态 | 描述 |
|-----------|--------|-------------|
| **GitCode G-Star** | 已申请 | 算力支持、流量曝光、鸿蒙认证 |
| **鸿蒙 Agent 技能** | 已发布 | 8 个技能：能力启动、原子服务、卡片、设备适配、跨应用、Agent 消息、意图路由、设备状态 |
| **ohpm SDK** | 已发布 | `@llm-box/workflow-engine` —— ArkTS SDK，含 WorkflowEngine、30+ 节点类型、设备适配、意图协议 |
| **昇腾 NPU 适配** | 活跃 | 7-Agent 自动适配流水线、3 种工作流模板（端到端/快速/性能调优）、CANN/MindIE 集成 |
| **SenseNova** | 活跃 | API 集成（6 个模型）、端侧 U1-Lite 支持（8B/A3B MoE）、8 个 SenseNova 生态技能 |
| **Ant Ling (百灵)** | 活跃 | API 集成（4 个模型：ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0）、OpenAI 兼容端点 |
| **GitHub** | 活跃 | CI/CD、CodeQL 安全扫描、自动发布 |

### 鸿蒙设备支持

| 设备类型 | 关键能力 |
|-------------|-----------------|
| 手机（标准） | 触摸、相机、GPS、NFC、生物识别 |
| 手机（双折） | 折叠屏、多窗口、拖拽分屏 |
| 手机（三折） | 折叠屏、多窗口、拖拽分屏 |
| 平板 | 手写笔、多窗口、分屏 |
| 智慧屏 | 语音、手势、遥控器 |
| 车机 | 方向盘控制、HUD、语音 |
| 穿戴 | 心率、加速度计、陀螺仪 |

### 昇腾 NPU 硬件支持

| 硬件 | 定位 | 模型规模 |
|----------|----------|-------------|
| 昇腾 910B | 训练/推理 | 7B-70B |
| 昇腾 910C | 训练/推理 | 7B-170B |
| Atlas 800I A2 | 推理服务器 | 7B-70B |
| Atlas 300I Duo | 边缘推理 | <13B |
| 310P | 边缘推理 | <7B |

📖 [G-Star 申请 →](ecosystem/GSTAR_APPLICATION.md) | [鸿蒙技能 →](ecosystem/harmonyos-skills/) | [ohpm SDK →](ecosystem/ohpm/) | [昇腾适配 →](ecosystem/ascend-adaptation/ASCEND_ADAPTATION.md)

---

## 📚 文档

### 入门

- [入门指南](docs/getting-started.md) —— 安装、第一个工作流、下一步
- [示例](examples/) —— 10 个即用型工作流模板

### 核心概念

- [数据流与变量](docs/dataflow.md) —— 步骤间数据传递、`{{input}}`、`{{step.N}}`、`{{var.NAME}}`、`{{secret.GROUP.KEY}}`
- [分布式执行](docs/distributed.md) —— 协调器/工作者设置、配置、扩展
- [调度](docs/scheduling.md) —— Cron 工作流、调度管理
- [MCP 集成](docs/mcp.md) —— 通过模型上下文协议连接外部工具
- [插件](docs/plugins.md) —— 安装和管理社区插件

### 进阶

- [Web UI 编辑器](docs/webui.md) —— 可视化工作流构建器
- [可视化器](docs/visualizer.md) —— Mermaid/JSON/DOT/ASCII 图表
- [租户隔离](docs/tenants.md) —— 多租户工作空间隔离
- [自定义节点](docs/custom-nodes.md) —— 用任意语言构建节点
- [故障排查](docs/troubleshooting.md) —— 错误码、常见问题、FAQ

### 参考

- [工作流 YAML 语法 →](docs/getting-started.md#workflow-configuration)
- [节点参考 →](docs/custom-nodes.md#built-in-nodes)
- [CLI 参考 →](docs/getting-started.md#cli-command-reference)
- [错误码 →](docs/troubleshooting.md#error-codes)

---

## 🛠️ CLI 命令

```bash
llm-box create [描述]      从描述生成工作流
llm-box run <文件>          运行工作流
llm-box secrets add         存储加密秘密
llm-box secrets list        列出分组中的秘密
llm-box schedule create     创建定时工作流
llm-box schedule list       列出定时工作流
llm-box coordinator         启动分布式协调器
llm-box worker              启动分布式工作者
llm-box ui                  启动 Web UI 编辑器
llm-box visualize <文件>    可视化工作流
llm-box validate <文件>     验证工作流文件
llm-box node install        安装外部节点
llm-box plugin install      安装插件
llm-box version             显示版本
llm-box help                显示完整帮助
```

📖 [完整 CLI 参考 →](docs/getting-started.md#cli-command-reference)

---

## 🏗️ 架构

```
┌─────────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐     ┌────────┐
│  提示词  │────▶│  规划器  │────▶│  工作流 YAML  │────▶│  执行器  │────▶│  结果  │
└─────────┘     └─────────┘     └──────────────┘     └──────────┘     └────────┘
                                                          │
                                              ┌───────────┴───────────┐
                                              ▼                       ▼
                                      ┌──────────────┐         ┌──────────────┐
                                      │  Agent 节点   │         │  实用节点     │
                                      │  ReAct 循环   │         │ 获取、执行等  │
                                      └──────────────┘         └──────────────┘
```

**关键组件：**
- **生成器** —— 基于关键词的自然语言工作流生成，100+ 即用型模板
- **解析器** —— YAML 工作流验证和解析
- **执行器** —— 带依赖追踪的确定性步骤执行
- **表达式引擎** —— 变量替换、秘密注入、文件读取
- **注册表** —— 50+ 内置节点 + 外部节点发现和加载
- **边缘路由器** —— ReAct 推理循环、三层持久化记忆、本地/云端模型路由
- **技能进化** —— 自我改进的 Agent 技能，追踪成功率并优化提示词
- **意图协议** —— `intent://` 和 `ohos://` URI 协议、W3C DID 身份、跨域消息
- **昇腾适配** —— 昇腾 NPU 模型适配 7-Agent 流水线（搜索/验证/适配/量化/优化/部署/文档）
- **代码智能** —— 代码图谱提取、Codex/OpenCode 兼容工具节点（glob/grep/list_dir/apply_patch）
- **子代理提示词** —— 17 个专家提示词模板、主/子代理层级（Grok Build 模式）
- **熔断器** —— 每工作者 Closed/Open/HalfOpen 状态机，保障分布式弹性
- **隐私层** —— 文件读取时秘密脱敏、出站数据量异常监控
- **协调器/工作者** —— 带熔断器保护的分布式任务调度和执行

---

## 🗺️ 路线图

| 版本 | 状态 | 特性 |
|---------|--------|----------|
| **v0.1** | ✅ 已发布 | 核心工作流引擎、10 个实用节点 |
| **v0.2** | ✅ 已发布 | 大模型节点、MCP 集成、外部节点 |
| **v0.3** | ✅ 已发布 | Agent 节点、分布式执行、Web UI、调度 |
| **v0.4** | ✅ 已发布 | 代码解释器、RAG、知识图谱、智能路由器、多模态、节点市场、100+ 模板、16 个专家、思维链 |
| **v0.5** | ✅ 已发布 | ReAct 引擎、分层记忆、技能自进化、鸿蒙适配（7 种设备）、跨平台协议（intent:// + ohos://）、W3C DID 身份、跨域 Agent 消息、GitCode G-Star + ohpm 生态 |
| **v0.5.1** | ✅ 已发布 | 昇腾 NPU 适配（7-Agent 流水线、3 种工作流模板、CANN/MindIE 集成） |
| **v0.5.2** | ✅ 已发布 | Grok Build 启发能力：代码图谱、子代理提示词层级、熔断器、秘密脱敏、文件监听、Codex/OpenCode 工具兼容、TUI Markdown/Mermaid 渲染（审计 15 个漏洞） |
| **v0.6.0** | **当前** | **百灵生态集成、AI 网关（OmniRoute）、Agent 记忆基础设施、语音 AI 工具链（ASR/分离/分析）、Agent 团队化（200+ 角色 + Agency 工作流）** |
| **v1.0** | 📅 2026 Q3 | 稳定 API、完整文档、LTS |

📖 [完整路线图 →](ROADMAP.md)

---

## 🤝 贡献指南

欢迎社区贡献！

### 新手任务

刚接触项目？从这里开始：

- 🐛 [标有 "good first issue" 的 Bug 修复](https://github.com/alib8b8/llm-box/labels/good%20first%20issue)
- 📝 文档改进
- ✅ 为低覆盖率包添加测试
- 🔧 新实用节点（参见[自定义节点指南](docs/custom-nodes.md)）
- 🌐 添加或改进翻译（国际化）

### 如何贡献

1. Fork 仓库
2. 创建分支：`git checkout -b feature/your-feature`
3. 修改并添加测试
4. 运行测试：`go test ./...`
5. 提交 Pull Request

### 评审流程

- CI 必须通过（构建、测试、Lint、安全扫描）
- 至少一位 CODEOWNER 批准
- 代码遵循 Go 规范（`gofmt`、`go vet`）
- 新功能包含测试和文档

📖 [完整贡献指南 →](CONTRIBUTING.md)

---

## 📄 许可证

MIT 许可证 —— 详见 [LICENSE](LICENSE)。

---

<div align="center">
  <p>为热爱终端的开发者精心打造 ❤️</p>
  <p>
    <a href="https://github.com/alib8b8/llm-box">GitHub</a>
    ·
    <a href="https://github.com/alib8b8/llm-box/issues">Issues</a>
    ·
    <a href="https://github.com/alib8b8/llm-box/discussions">Discussions</a>
  </p>
</div>