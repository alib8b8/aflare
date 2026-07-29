<div align="center">
  <h1>llm-box</h1>
  <p>🌍
    <strong>中文</strong> ·
    <a href="README.en.md">English</a> ·
    <a href="README.ru.md">Русский</a>
  </p>
  <p><strong>GitHub Actions for your laptop.</strong></p>
  <p>用自然语言自动化你的终端。厌倦了写 Bash 脚本？让 AI 帮你把想法变成可执行的 YAML 工作流。</p>
  <p><em>What if Bash understood English?</em></p>

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
      <img src="https://img.shields.io/badge/License-AGPL%20v3.0-blue.svg?style=flat-square" alt="许可证" />
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
- [💡 为什么是 llm-box？](#-为什么是-llm-box)
- [📖 故事](#-故事)
- [✨ 核心特性](#-核心特性)
- [🤖 Agent 节点](#-agent-节点)
- [📱 鸿蒙 & 移动端节点](#-鸿蒙--移动端节点)
- [🔒 安全](#-安全)
- [🌐 生态](#-生态)
- [📚 文档](#-文档)
- [🛠️ CLI 命令](#️-cli-命令)
- [🏗️ 架构](#️-架构)
- [🗺️ 路线图](#️-路线图)
- [🌟 优秀集成案例](#-优秀集成案例)
- [🛒 精选技能市场](#-精选技能市场)
- [📖❓ 疑问解答](#-疑问解答)
- [🤝 贡献指南](#-贡献指南)
- [📦 代码托管](#-代码托管)
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

### 一句话生成工作流

用自然语言告诉 llm-box 你想做什么，它会生成 YAML 工作流：

```bash
# 监控 BTC 价格，每 10 分钟检查一次
llm-box create "monitor BTC price every 10 minutes and send telegram alert when > 70000"

# 监控 GitHub 仓库 Star 数，超过阈值提醒
llm-box create "watch my github repo alib8b8/llm-box and notify me when star > 100"

# 每天下载 arXiv AI 论文并总结
llm-box create "download arxiv AI papers every day and summarize top 5"

# 监控特斯拉股价并发送 Telegram 告警
llm-box create "watch TSLA stock and send telegram alert when price drops 5%"

# 监控上海天气并发邮件
llm-box create "monitor Shanghai weather and email me if it will rain tomorrow"
```

然后运行生成的工作流：

```bash
llm-box run btc-monitor.yaml
```

📖 [完整入门指南 →](docs/getting-started.md)

---

## 💡 为什么是 llm-box？

> llm-box 不是 AI 助手 —— 它是一个**确定性执行引擎**。
>
> AI 负责理解你的意图，YAML 负责保证执行。

| 工具 | 问题 | llm-box 的方式 |
|------|------|----------------|
| **Bash** | 太难写，难维护，容易出错 | 自然语言生成，YAML 可读可改 |
| **n8n** | 太重了，一个自动化任务还要跑 Docker | 单二进制，终端里就能用 |
| **Zapier** | SaaS，数据不在自己手里，收费贵 | 本地执行，完全可控 |
| **Claude Code** | 偏代码，不擅长自动化工作流 | 通用工作流引擎，不只是代码 |
| **AI Agent** | 太不可控，输出全是幻觉 | AI 只负责翻译意图，执行由 YAML 保证 |

---

## 📖 故事

> I built llm-box because I was tired of writing Bash scripts.
>
> 每次想自动化一个简单的事情——监控价格、抓取新闻、定时发消息——都要写几十行 Bash，还要处理错误、重试、依赖。
>
> 我想要的是：**告诉电脑我想做什么，它就去做。**
>
> 就像 GitHub Actions 一样简单，但在我的笔记本上跑。

---

## ✨ 核心特性

| 类别 | 特性 |
|------|------|
| **🎯 自然语言 → YAML** | 用一句话描述需求，自动生成可执行工作流。AI 只负责翻译，不负责决策 |
| **📦 Homebrew 式体验** | 一行命令安装，一条指令创建，模板一键复用。像装包一样装工作流 |
| **⚙️ 确定性执行** | YAML 工作流 = 确定性输出。没有幻觉，没有随机，每次结果都一样 |
| **🤖 个人 AI 超级智能** | 你的私有 AI Agent 在你的笔记本上跑。不是 SaaS，不是云端，就是你的 |
| **🔌 完全离线运行** | 配合 Ollama，所有东西都可以在本地完成。断网也能工作，数据从不离开你的设备 |
| **🔄 定时调度** | 内置 Cron 调度，每 10 分钟、每天、每周——想怎么跑就怎么跑 |
| **🧩 100+ 内置模板** | BTC 监控、GitHub Star 告警、Arxiv 论文总结、天气提醒……拿来就用 |
| **🔌 插件系统** | 像 Homebrew tap 一样扩展。`llm-box install btc-monitor` |
| **🌐 多模型支持** | 支持 Ollama / OpenAI / DeepSeek / Qwen / Kimi / GLM / Mistral，本地云端都能跑 |
| **🔒 隐私优先** | 默认本地执行，秘密自动脱敏，审计日志完整，98+ 漏洞已审计 |
| **🛡️ 企业级安全** | SSRF 防护、路径遍历防御、命令注入白名单、AES-GCM 加密 |

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
| `human_in_loop` | 暂停等待人工审批 |
| `code_knowledge_graph` | 语义代码知识图谱：158 种语言、向量检索、实体/关系/概念提取、**MCP 工具暴露**、**Token 高效审查**、**PR 分析**、**Inkling 代码审查** |
| `moe_streaming` | MoE 专家流式加载：消费级硬件运行 744B 模型，按需加载 |
| `cli_session` | 交互式终端会话，支持上下文持久化、流式输出、自动补全 |
| `plugin_system` | 插件扩展：从本地/git/网址/市场安装/卸载/更新，沙箱隔离 |
| `engineer_skills` | 16 个预置技能：React/TypeScript/API/数据库/CI-CD/Docker/设计模式 |
| `skill_distill` | 从书籍/视频/播客中蒸馏方法论为可调用技能 |
| `voice_output` | 语音 AI 工具链：TTS + 声音克隆 + **ASR 转录** + **说话人分离** + **语音分析** + 创作模式、11 种语言、5 种 ASR 引擎、**Inkling 音频理解** |
| `doc_gen` | AI 文档生成：7 种类型（自述文件/API/函数/模块/变更日志/教程/架构） |
| `video_edit` | AI 视频编辑：智能剪辑/合并/特效/字幕/故事板/超分 |
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

### 代码智能节点

| 节点 | 描述 |
|------|-------------|
| `file_watch` | 监听路径文件变化（创建/修改/删除），基于轮询，上下文感知 |

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
| **熔断器** | 每节点熔断器（Closed→Open→HalfOpen），故障节点自动隔离防止级联故障 |
| **ANSI 注入** | TUI Markdown/Mermaid 渲染器从用户输入中剥离终端控制序列（CSI/OSC/DCS） |
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
| **鸿蒙 & 移动端节点** | 内置 | 19 个节点：能力启动、原子服务、卡片、设备适配、跨应用、Agent 消息、意图路由、设备状态、UI 自动化、系统事件等 |
| **SenseNova** | 活跃 | API 集成（6 个模型）、端侧 U1-Lite 支持（8B/A3B MoE） |
| **Ant Ling (百灵)** | 活跃 | API 集成（4 个模型：ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0）、OpenAI 兼容端点 |
| **OPPO AndesGPT** | 活跃 | API 集成（Tiny/Turbo/Titan 三档）、PersonaX 个性化、端云协同 |
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

---

## 📚 文档

### 入门

- [入门指南](docs/getting-started.md) —— 安装、第一个工作流、下一步
- [示例](examples/) —— 10 个即用型工作流模板

### 核心概念

- [数据流与变量](docs/dataflow.md) —— 步骤间数据传递、`{{input}}`、`{{step.N}}`、`{{var.NAME}}`、`{{secret.GROUP.KEY}}`
- [分布式执行](docs/distributed.md) —— 协调器/工作者架构（设计文档，尚未实现）
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
llm-box run --resume <文件> 从上次中断处恢复工作流
llm-box secrets add         存储加密秘密
llm-box secrets list        列出分组中的秘密
llm-box schedule add        添加定时任务
llm-box schedule list       列出定时任务
llm-box schedule remove     移除定时任务
llm-box schedule start      启动调度器
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
- **子代理提示词** —— 17 个专家提示词模板、主/子代理层级（Grok Build 模式）
- **熔断器** —— 每节点 Closed/Open/HalfOpen 状态机，故障自动隔离防止级联失败
- **隐私层** —— 文件读取时秘密脱敏、出站数据量异常监控
- **Checkpoint/Resume** —— 工作流每步自动保存状态，`--resume` 从中断处恢复

---

## 🗺️ 路线图

| 版本 | 状态 | 特性 |
|---------|--------|----------|
| **v0.1** | ✅ 已发布 | 核心工作流引擎、10 个实用节点 |
| **v0.2** | ✅ 已发布 | 大模型节点、MCP 集成、外部节点 |
| **v0.3** | ✅ 已发布 | Agent 节点、Web UI、调度 |
| **v0.4** | ✅ 已发布 | 代码解释器、RAG、知识图谱、智能路由器、多模态、节点市场、100+ 模板、16 个专家、思维链 |
| **v0.5** | ✅ 已发布 | ReAct 引擎、分层记忆、技能自进化、鸿蒙适配（7 种设备）、跨平台协议（intent:// + ohos://）、W3C DID 身份、跨域 Agent 消息、GitCode G-Star + ohpm 生态 |
| **v0.5.1** | ✅ 已发布 | 昇腾 NPU 适配（7-Agent 流水线、3 种工作流模板、CANN/MindIE 集成） |
| **v0.5.2** | ✅ 已发布 | Grok Build 启发能力：代码图谱、子代理提示词层级、熔断器、秘密脱敏、文件监听、Codex/OpenCode 工具兼容、TUI Markdown/Mermaid 渲染（审计 15 个漏洞） |
| **v0.6.0** | **当前** | **百灵生态集成、AI 网关（OmniRoute）、Agent 记忆基础设施、语音 AI 工具链（ASR/分离/分析）、Agent 团队化（200+ 角色 + Agency 工作流）** |
| **v1.0** | 📅 2026 Q3 | 稳定 API、完整文档、LTS |

📖 [完整路线图 →](ROADMAP.md)

---

## 🌟 优秀集成案例

使用 llm-box 构建的优秀开源项目：

| 项目 | 描述 |
|------|------|
| [AI 新闻助手]() | 基于 llm-box 工作流的 AI 新闻聚合与摘要系统 |
| [代码审查 Agent]() | 利用代码知识图谱节点的自动化代码审查工具 |
| [研究助手]() | 结合 researcher + knowledge_graph 节点的学术研究工作流 |

> 如果您的项目使用了 llm-box，欢迎提交 PR 添加到此处！

---

## 🛒 精选技能市场

llm-box 内置 100+ 开箱即用的工作流模板，覆盖开发、运维、营销、研究等场景。一键安装，即刻使用。

### 安装方式

```bash
# 从技能市场一键安装
llm-box create from templates/developer-tools/unit-test-generator

# 或直接从 GitHub 安装
llm-box create from https://github.com/alib8b8/llm-box/tree/main/templates/ai-ml/prompt-engineering
```

### 热门技能

| 分类 | 技能 | 描述 | 一键安装 |
|------|------|------|---------|
| 🤖 AI/ML | Prompt Engineering | LLM 提示词工程模板 | `llm-box create from templates/ai-ml/prompt-engineering` |
| 🤖 AI/ML | LLM Fine-tune | 大模型微调流水线 | `llm-box create from templates/ai-ml/llm-finetune` |
| 🤖 AI/ML | Model Evaluation | 模型评估与基准测试 | `llm-box create from templates/ai-ml/model-evaluation` |
| 💻 开发工具 | Unit Test Generator | 自动生成单元测试 | `llm-box create from templates/developer-tools/unit-test-generator` |
| 💻 开发工具 | API Docs Generator | API 文档自动生成 | `llm-box create from templates/developer-tools/api-docs-generator` |
| 💻 开发工具 | Code Duplicate Finder | 代码重复检测 | `llm-box create from templates/developer-tools/code-duplicate-finder` |
| 💻 开发工具 | Dependency Checker | 依赖安全检查 | `llm-box create from templates/developer-tools/dependency-checker` |
| 🔧 DevOps | Log Analyzer | 日志智能分析 | `llm-box create from templates/devops-monitoring/log-analyzer` |
| 🔧 DevOps | Docker Cleaner | Docker 资源清理 | `llm-box create from templates/devops-monitoring/docker-cleaner` |
| 🔧 DevOps | SSL Cert Checker | SSL 证书到期检查 | `llm-box create from templates/devops-monitoring/ssl-cert-checker` |
| 📊 数据分析 | CSV Analyzer | CSV 数据分析 | `llm-box create from templates/data-analytics/csv-analyzer` |
| 📊 数据分析 | A/B Test Analyzer | A/B 测试分析 | `llm-box create from templates/data-analytics/ab-test-analyzer` |
| 📊 数据分析 | Financial Analyzer | 财务数据分析 | `llm-box create from templates/data-analytics/financial-analyzer` |
| 📝 内容创作 | Blog Outline Generator | 博客大纲生成 | `llm-box create from templates/content-marketing/blog-outline-generator` |
| 📝 内容创作 | SEO Keyword Research | SEO 关键词研究 | `llm-box create from templates/content-marketing/seo-keyword-research` |
| 🔬 研究 | Literature Review | 文献综述 | `llm-box create from templates/research/literature-review` |
| 🔬 研究 | Paper Summarizer | 论文摘要 | `llm-box create from templates/research-analysis/paper-summarizer` |
| 🔬 研究 | Competitor Analysis | 竞品分析 | `llm-box create from templates/research-analysis/competitor-analysis` |
| 📈 商业 | Business Plan | 商业计划书 | `llm-box create from templates/business-sales/business-plan` |
| 📈 商业 | SaaS Pricing | SaaS 定价策略 | `llm-box create from templates/business-sales/saas-pricing` |
| 🔒 安全 | Security Audit | 安全审计 | `llm-box create from templates/security/security-audit` |
| 🔒 安全 | Incident Response | 事件响应 | `llm-box create from templates/security/incident-response` |
| 📚 文档 | README Generator | README 自动生成 | `llm-box create from templates/documentation/readme-generator` |
| 📚 文档 | API Docs Builder | API 文档构建 | `llm-box create from templates/documentation/api-docs-builder` |
| 🏗️ 架构 | Microservices Design | 微服务设计 | `llm-box create from templates/backend/microservices-design` |
| 🏗️ 架构 | Cloud Architecture | 云架构设计 | `llm-box create from templates/cloud-infra/cloud-architecture` |

> 完整技能列表见 [templates/](templates/) 目录，80+ 技能持续更新中。

---

## 📖❓ 疑问解答

### 1. llm-box 与其他 Agent 框架有什么区别？

llm-box 专注于**确定性工作流与 AI Agent 的结合**：工作流确保执行可靠和可复现，Agent 节点提供智能推理能力。我们不依赖单一模型提供商，支持 22+ 模型、5 种路由策略，核心采用 Go 编写，零依赖、启动快、内存占用低。

### 2. 支持哪些大模型？

目前支持 **22+ 模型**，涵盖国内外主流提供商：
- **国内**：SenseNova（商汤）、Ant Ling（蚂蚁百灵）、AndesGPT（OPPO）、DeepSeek、Qwen（阿里通义）
- **国外**：OpenAI、Anthropic Claude、Google Gemini、Inkling（Thinking Machines）
- **端侧**：llama.cpp、ONNX Runtime、SenseNova U1（INT4/INT8）

### 3. 可以在企业环境中使用吗？

完全可以。llm-box 采用 GNU Affero 通用公共许可证 v3.0，提供：
- **分级防护配置**（L0-L3），支持从开发环境到生产环境的安全梯度
- **秘密脱敏** + **出站数据监控**，防止数据泄漏
- **审计日志**，所有操作可追溯
- **节点级熔断器**，保障系统稳定性

### 4. 如何扩展自定义节点？

llm-box 支持三种扩展方式：
- **插件系统**：从本地/git/网址/市场安装插件，沙箱隔离
- **MCP 协议**：通过 Model Context Protocol 连接外部工具
- **自定义节点**：用任意语言实现标准接口，参考[自定义节点指南](docs/custom-nodes.md)

### 5. 项目的长期规划是什么？

短期（v0.6-v0.9）：完善 Agent 团队协作、多模态能力、性能优化
长期（v1.0+）：稳定 API、LTS 版本、企业级支持、更多硬件适配（昇腾/寒武纪/海光）
详见[路线图 →](#️-路线图)

---

## 📦 代码托管

llm-box 在多个平台同步，欢迎在您常用的平台关注和贡献：

| 平台 | 链接 |
|------|------|
| **GitHub** | https://github.com/alib8b8/llm-box |
| **GitCode / AtomGit** | https://gitcode.com/llm-box/llm-box |

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

GNU Affero 通用公共许可证 v3.0 —— 详见 [LICENSE](LICENSE)。

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