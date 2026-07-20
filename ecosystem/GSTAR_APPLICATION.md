# GitCode G-Star 项目申请

## 项目信息

| 项目名称 | llm-box (AI 工作流引擎) |
|---|---|
| GitCode 仓库 | https://gitcode.com/llm-box/llm-box |
| GitHub 仓库 | https://github.com/alib8b8/llm-box |
| 开源协议 | MIT |
| 主要语言 | Go |
| 项目阶段 | 活跃开发中，已有生产可用功能 |
| 提交次数 | 252+ commits |
| 贡献者 | 7 位活跃贡献者 |
| 文件规模 | 492 个文件 |

## 项目简介

llm-box 是一个开源的 AI 工作流编排引擎，让开发者通过 YAML 定义工作流，串联 55+ 节点（LLM、Agent、代码执行、RAG、多模态等）构建复杂的 AI 应用。项目深度融合 ReAct 推理循环、分层持久化记忆、Skill 自演进机制，率先完成鸿蒙 HarmonyOS 与昇腾 NPU 双生态适配，是国内首个覆盖"端侧 Agent + 国产算力 + 鸿蒙生态"的开源 AI 工作流引擎。

**核心亮点：**

- **55+ 节点类型**：支持 OpenAI/DeepSeek/Qwen/GLM/Kimi/AndesGPT 等 LLM 节点、Agent、Supervisor（5种策略+17位专家）、Code Interpreter、RAG、知识图谱、多模态、Smart Router、代码图谱、文件监控、熔断器、区块链审计、屏幕理解、语音输入、机器人控制、端侧 LLM、功耗管理
- **100+ 工作流模板**：覆盖开发工具、DevOps、内容营销、研究分析、AI/ML、安全、教育、金融等 20+ 领域
- **跨平台 AI 任务协议**：基于 `intent://` 和 `ohos://` 的统一意图协议，支持 W3C DID 身份验证与跨域 Agent 消息路由
- **端侧 AI Agent 引擎**：ReAct 推理循环、三层持久化记忆（会话/任务/长期）、本地/云端模型路由、隐私保护
- **鸿蒙 HarmonyOS 深度适配**：Ability 启动、原子化服务、桌面卡片、7种设备类型适配（直板/双折叠/三折叠/平板/智慧屏/车机/穿戴）、ohpm SDK
- **昇腾 NPU 模型适配**：7-Agent 自动化适配流水线（搜索→验证→适配→量化→优化→部署→文档），单模型端到端适配 ≤ 60 分钟
- **OPPO AndesGPT 集成**：支持 Tiny/Turbo/Titan 三规格模型、PersonaX 千人千面、端云协同
- **安全防护**：SSRF 防护、命令注入防护、路径验证、认证时序攻击防护、AES-GCM 密钥管理、密钥自动脱敏、出站数据量监控、区块链审计；已修复 84+ 安全漏洞

**AtomGit/GitCode 链接：** https://gitcode.com/llm-box/llm-box

## 技术架构

```
┌──────────────────────────────────────────────────┐
│                  用户接口层                       │
│  CLI · VS Code 插件 · WebUI · TUI · MCP Server   │
├──────────────────────────────────────────────────┤
│                  工作流引擎层                      │
│  Parser · Executor · Scheduler · Generator       │
│  Expression Engine · Debugger · Visualizer       │
├──────────────────────────────────────────────────┤
│                  节点层                           │
│  LLM Nodes (10+) · Agent · Supervisor            │
│  Code Interpreter · RAG · Knowledge Graph        │
│  Multimodal · Smart Router · Mobile Nodes        │
│  HarmonyOS Nodes · Security · Marketplace        │
├──────────────────────────────────────────────────┤
│                  端侧 & 跨域层                     │
│  Edge Router · ReAct Engine · Layered Memory     │
│  Agent Registry · Cross-Domain Messenger         │
│  DID Identity · Privacy Analyzer                 │
├──────────────────────────────────────────────────┤
│                  协议层                           │
│  Intent Protocol · ohos:// scheme                │
│  W3C DID · Task Message · Workflow Types         │
└──────────────────────────────────────────────────┘
```

## G-Star 申请理由

### 1. 技术创新性
- 国内首个深度适配鸿蒙的 AI 工作流引擎，支持 Ability/原子化服务/桌面卡片/7种设备类型适配
- 融合 ReAct 推理循环 + 三层持久化记忆（会话/任务/长期），端侧 Agent 能力对标 JiuwenClaw-on-OpenHarmony
- Skill 自演进机制，Agent 技能越用越强，借鉴 jiuwenswarm 的自演进理念
- 跨平台 AI 任务协议支持 W3C DID 身份验证，借鉴 awiki.ai 的 Agent 原生身份体系
- 借鉴 SpaceX Grok Build 开源 7 项核心能力：代码图谱（语义级代码理解）、熔断器（Closed/Open/HalfOpen 状态机）、子智能体提示词架构（17 个 specialist 模板）、工具可移植性（兼容 Codex/OpenCode 工具接口）、文件监控、隐私增强（密钥脱敏 + 出站数据监控）、TUI 渲染增强（Markdown + Mermaid 转 ASCII）
- 昇腾 NPU 7-Agent 自动化适配流水线，单模型端到端适配 ≤ 60 分钟，效率提升 8-24 倍

### 2. 生态价值
- 100+ 开箱即用的工作流模板，降低 AI 应用开发门槛
- 支持国内主流大模型：通义千问、智谱 GLM、百川、零一万物、DeepSeek、MiniMax、面壁MiniMo、商汤日日新、月之暗面 Kimi
- 鸿蒙生态贡献：HarmonyOS Agent Skill 包、ohpm SDK（@llm-box/workflow-engine）
- 昇腾生态贡献：计划贡献 100+ Agent Skill 包、模型族适配模板库、性能调优最佳实践库

### 3. 社区活跃度
- 252+ 次提交，7 位活跃贡献者，持续迭代
- 完善的文档体系（20+ 文档）
- CI/CD 流程完善（GitHub Actions + CodeQL 安全扫描）
- 已完成 3 轮安全审计，修复 61 个安全漏洞

### 4. 对 G-Star 的需求
- **昇腾算力支持**：用于端侧 AI Agent 的本地模型推理测试与昇腾适配流水线验证
- **流量曝光**：让更多开发者和企业用户了解和使用项目，推动鸿蒙 + 昇腾双生态落地
- **技术支持**：鸿蒙生态的技术对接和认证、昇腾 CANN 工具链深度对接
- **社区资源**：参与 GitCode AI 社区的课程和活动，共建国产 AI 基础设施

## 未来规划

1. **鸿蒙原生应用**：开发基于 ArkTS 的原生 App，在鸿蒙设备上运行端侧 Agent
2. **DevEco Code Skill 插件**：在华为 DevEco Code 中提供 AI 辅助鸿蒙开发能力
3. **国产算力适配**：适配昇腾 NPU，在端侧推理中利用国产硬件加速
4. **Agent 市场**：构建类似 GPT Store 的 Agent 分发平台
5. **多模态工作流**：支持图像、语音、视频的处理工作流

## 联系方式

- 项目维护者：alib8b8
- 邮箱：通过 GitCode/GitHub 联系
- Issues：https://gitcode.com/llm-box/llm-box/issues
