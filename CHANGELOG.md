# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.0] - 2026-08-07

### Added
- **Saga 事务补偿**：forward + compensation 步骤，失败时按反向执行 best-effort 补偿，幂等性保证
- **LLM 成本归因**：token usage × 模型单价表，自动计算 USD 成本写入审计日志 (`cost_usd`/`total_tokens`)
- **宇树机器人集成**：`unitree_robot` 节点，支持 Go2/B2/H1/G1 等 9 种机型、14 种动作，simulate/API 双模式
- **寒武纪 MLU 适配**：Cambricon provider 节点，支持 MLU 370/590
- **海光 DCU 适配**：Hygon provider 节点，支持 K100/Z100
- **ARM64 鲲鹏 CI**：CI 矩阵新增 `ubuntu-24.04-arm` 运行器
- **昇腾 AML 风控 + Saga 转账示例**：金融场景在国产芯片上的完整落地
- **多模态节点修复**：vision-LLM 路径正确发送图像数据，OCR 支持 tesseract + LLM 回退
- **HMAC 哈希链审计**：审计日志不可篡改
- **幂等性支持**：Idempotency-Key + 跨进程锁
- **HTTP 限流/重试**：内置 rate limiting 和自动重试
- **LLM 响应缓存**：减少重复调用成本
- **配额持久化 + 多租户**：配额跨进程持久化
- **Trace 脱敏**：JWT/私钥等敏感信息自动脱敏
- **WAL 崩溃恢复**：Write-Ahead Log 保证执行状态可恢复
- **结构化日志**：基于 `log/slog` 的 JSON/文本双格式日志
- **HTTP API 服务**：REST API 支持 workflow 执行和状态查询
- **项目更名为 aflare**：从 llm-box 正式更名为 aflare

### Changed
- 项目名从 llm-box 更名为 aflare
- 环境变量前缀从 `LLM_BOX_` 改为 `AFLARE_`
- Go 模块路径改为 `github.com/alib8b8/aflare`
- PR Review workflow 仅保留 pull_request 触发，移除 push 触发

### Fixed
- sql_query.go TOCTOU 竞态条件修复
- sql_query.go 错误处理缺失修复
- gofmt 格式对齐问题（多次）
- golangci-lint unused mutex + staticcheck S1017

## [0.6.0] - 2026-08-01

### Added
- **百灵生态集成**：多 Agent 协作框架
- **AI 网关 OmniRoute**：统一 LLM 路由，智能分发
- **Agent 记忆基础设施**：向量记忆、用户画像、分层记忆
- **语音 AI 工具链**：ASR 语音识别、音频分离、语音分析
- **Agent 团队化**：200+ 角色 + Agency 工作流编排
- **WAL 持久化**：Write-Ahead Log 保证执行可靠性
- **字节码 IR 表达式引擎**：高性能 workflow 表达式求值
- **EWMA / 帕累托路由**：智能负载均衡和路由
- **TLA+ 形式化验证**：DAG 执行正确性形式化证明
- **代码解释器**：内置 Python 沙箱执行
- **RAG 检索增强**：知识库 + 向量检索
- **知识图谱**：实体关系建模和推理
- **智能路由器**：多模型自动路由和 fallback
- **多模态节点**：图像分析、OCR、音频转录
- **节点市场**：100+ 可复用工作流模板
- **16 个领域专家**：金融、医疗、法律等垂直领域
- **思维链推理**：Chain-of-Thought 复杂推理支持

## [0.5.2] - 2026-07-28

### Added
- **代码图谱**：代码结构可视化分析
- **子代理提示词层级**：分层 Agent 提示词管理
- **熔断器**：LLM 调用自动熔断和降级
- **秘密脱敏**：敏感信息自动检测和脱敏
- **文件监听**：文件变更自动触发 workflow
- **TUI Markdown/Mermaid 渲染**：终端内图表渲染
- **LLM 路由统一**：3 套路由合并为 1 套统一路由

## [0.5.1] - 2026-07-25

### Added
- **昇腾 NPU 适配**：7-Agent 流水线、3 种工作流模板
- **CANN / MindIE 集成**：国产 AI 推理服务适配

## [0.5.0] - 2026-07-21

### Added
- **ReAct 引擎**：Reasoning + Acting 自主决策循环
- **分层记忆**：短期/长期/工作记忆三层架构
- **技能自进化**：Agent 自主学习和技能积累
- **鸿蒙适配**：7 种鸿蒙设备节点支持
- **跨平台协议**：`intent://` + `ohos://` URI scheme
- **W3C DID 身份**：去中心化身份认证
- **跨域 Agent 消息**：分布式 Agent 通信
- **GitCode G-Star + ohpm 生态**：开源生态集成

## [0.4.0] - 2026-07-14

### Added
- **代码解释器**：Python 沙箱执行节点
- **RAG 检索增强生成**：知识库 + 向量检索
- **知识图谱节点**：实体关系建模
- **智能路由器节点**：多模型自动路由
- **多模态节点**：图像分析、OCR、音频转录
- **节点市场**：100+ 可复用模板
- **16 个领域专家**：垂直领域优化
- **思维链推理**：Chain-of-Thought 支持

## [0.3.0] - 2026-07-04

### Added
- Multi-language support (9 languages: zh, en, ru, fr, ja, ko, es, ar, hi)
- Condition execution support for workflow steps
- Variable substitution (vars field) in workflows
- Atomic write operations for file_write node
- Workflow chaining via call node
- Dockerfile for containerized deployment
- Makefile for build automation
- GoReleaser configuration for cross-platform releases
- Homebrew tap support
- SHA256 checksum verification in install.sh
- Thread-safe Registry with mutex locks
- .gosec.json security scan configuration

### Changed
- Tightened directory permissions from 0755 to 0750
- Tightened file permissions from 0644 to 0600
- Ollama node now prioritizes prompt parameter over input
- notify node returns error for invalid channel instead of silent fallback

### Fixed
- SSRF protection (DNS resolution, IPv4-mapped IPv6 bypass, redirect validation)
- Path traversal protection (symlink resolution, dot-file rejection)
- Command injection protection (shell metacharacter blocking)
- template_render SSTI vulnerability - removed dangerous template functions
- Integer overflow in registry lowercase function
- Context leak in workflow executor (defer to immediate stepCancel)
- Keyword matching improved with word boundary checks and Chinese support
- gofmt formatting issues across 12 files

### Security
- Complete SSRF protection layer
- Path traversal protection for all file operations
- Command injection prevention for execute node
- Resource limits (file size, response body, retry/parallel/step counts)
- Recursive call depth tracking for workflow chaining
- Sensitive data filtering in audit logs
- External node API key protection

## [0.2.10] - 2026-06-16

### Added
- External node support with registry
- Node install/uninstall commands
- LLM node deduplication (llm_base.go)

### Fixed
- Various bug fixes and stability improvements

## [0.1.0] - 2026-06-02

### Added
- Initial release
- Core workflow engine with YAML-based step definition
- Built-in nodes: llm, execute, file_read, file_write, http_request, fetch_url
- Interactive TUI with bubbletea
- Workflow generation from natural language
- Ollama integration
- History tracking

[Unreleased]: https://github.com/alib8b8/aflare/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/alib8b8/aflare/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/alib8b8/aflare/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/alib8b8/aflare/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/alib8b8/aflare/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/alib8b8/aflare/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/alib8b8/aflare/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/alib8b8/aflare/compare/v0.2.10...v0.3.0
[0.2.10]: https://github.com/alib8b8/aflare/compare/v0.1.0...v0.2.10
[0.1.0]: https://github.com/alib8b8/aflare/releases/tag/v0.1.0
