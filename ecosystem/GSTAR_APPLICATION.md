# GitCode G-Star 项目申请

## 项目信息

| 项目名称 | llm-box (AI 工作流引擎) |
|---|---|
| GitCode 仓库 | https://gitcode.com/llm-box/llm-box |
| GitHub 仓库 | https://github.com/alib8b8/llm-box |
| 开源协议 | MIT |
| 主要语言 | Go |
| 项目阶段 | 活跃开发中，已有生产可用功能 |

## 项目简介

llm-box 是一个开源的 AI 工作流编排引擎，让开发者通过 YAML 定义工作流，串联 40+ 节点（LLM、Agent、代码执行、RAG、多模态等）构建复杂的 AI 应用。

### 核心能力

- **40+ 节点类型**：OpenAI/DeepSeek/Qwen/GLM/Kimi 等 LLM 节点、Agent、Supervisor（5种策略+16位专家）、Code Interpreter、RAG、知识图谱、多模态、Smart Router
- **100+ 工作流模板**：覆盖开发工具、DevOps、内容营销、研究分析、AI/ML、安全、教育、金融等 20+ 领域
- **跨平台 AI 任务协议**：基于 intent:// 和 ohos:// 的统一意图协议，支持 W3C DID 身份验证
- **端侧 AI Agent 引擎**：ReAct 推理循环、分层持久化记忆、本地/云端模型路由、隐私保护
- **鸿蒙 HarmonyOS 适配**：Ability 启动、原子化服务、桌面卡片、多设备适配检测（7种设备类型）
- **安全防护**：SSRF 防护、命令注入防护、路径验证、认证时序攻击防护、AES-GCM 密钥管理
- **多平台集成**：VS Code 插件、Claude/Grok/Trae/Codex 插件、MCP 协议支持

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
- 国内首个深度适配鸿蒙的 AI 工作流引擎，支持 Ability/原子化服务/桌面卡片/多设备适配
- 融合 ReAct 推理循环 + 分层持久化记忆，端侧 Agent 能力对标 JiuwenClaw-on-OpenHarmony
- Skill 自演进机制，Agent 技能越用越强，借鉴 jiuwenswarm 的自演进理念
- 跨平台 AI 任务协议支持 W3C DID 身份验证，借鉴 awiki.ai 的 Agent 原生身份体系

### 2. 生态价值
- 100+ 开箱即用的工作流模板，降低 AI 应用开发门槛
- 支持国内主流大模型：通义千问、智谱 GLM、百川、零一万物、DeepSeek、MiniMax、面壁MiniMo、商汤日日新、月之暗面 Kimi
- 鸿蒙生态贡献：HarmonyOS Agent Skill 包、ohpm SDK

### 3. 社区活跃度
- 持续迭代，功能不断完善
- 完善的文档体系（20+ 文档）
- CI/CD 流程完善（GitHub Actions + CodeQL 安全扫描）

### 4. 对 G-Star 的需求
- **昇腾算力支持**：用于端侧 AI Agent 的本地模型推理测试
- **流量曝光**：让更多开发者和企业用户了解和使用项目
- **技术支持**：鸿蒙生态的技术对接和认证
- **社区资源**：参与 GitCode AI 社区的课程和活动

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
