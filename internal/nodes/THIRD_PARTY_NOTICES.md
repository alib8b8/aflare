# 第三方工具声明 (Third-Party Notices)

本目录下的 `tools_compat.go` 实现了"工具可移植性"兼容层，为从其他主流
AI Agent 工具链迁移到 llm-box 的工作流提供熟悉的工具名入口。

下列工具的**工具名与概念**借鉴自以下开源项目，但 llm-box 中的实现为**完全
独立的原创实现**，未复制任何源代码。

## 借鉴来源

### 1. Codex (openai/codex)
- License: Apache License 2.0
- Repository: https://github.com/openai/codex
- 借鉴内容：`glob` / `grep` / `list_dir` / `apply_patch` 的工具名与概念
- 说明：Codex 提供了一组面向代码工作流的本地工具，本兼容层沿用了其工具命名
  与语义约定，使 Codex 用户迁移到 llm-box 时无需重写工作流。

### 2. OpenCode
- License: MIT License
- 借鉴内容：`bash` / `edit` / `glob` / `grep` / `read` / `skill` / `todowrite` /
  `write` 的工具名与概念
- 说明：OpenCode 定义了一组面向 Agent 编排的工具接口，本兼容层中的 `glob` 与
  `grep` 同时借鉴了 OpenCode 的同名工具语义。其余工具名（`bash`/`edit`/`read`/
  `write`/`skill`/`todowrite`）仅作为概念参考列出，llm-box 已有等价节点
  （如 `file_read` / `file_write` / `execute`），未在此兼容层中重复实现。

### 3. Grok Build (xai-org/grok-build)
- License: Apache License 2.0
- Repository: https://github.com/xai-org/grok-build
- 借鉴内容：从 Codex / OpenCode 移植工具的实践做法，以及工具安全限制
  （路径校验、递归深度限制、结果数限制、二进制文件跳过、原子补丁等）的参考。
- 说明：Grok Build 公开演示了在自有 Agent 中移植 Codex/OpenCode 工具的做法，
  本兼容层在设计上参考了其移植思路，但所有代码均为独立实现。

## 实现范围

本兼容层 (`tools_compat.go`) 实现了以下兼容节点：

| 工具名 | 借鉴来源 | 功能 |
|--------|----------|------|
| `glob` | Codex / OpenCode | 递归匹配文件路径，返回匹配文件列表 |
| `grep` | Codex grep_files / OpenCode | 递归搜索文件内容，返回匹配行 |
| `list_dir` | Codex | 列出目录内容（可选递归） |
| `apply_patch` | Codex | 应用 unified diff 格式补丁（原子语义） |

## 独立实现声明

llm-box 是一个独立项目。本兼容层中的所有代码：

- **未复制** Codex / OpenCode / Grok Build 的任何源代码；
- **仅借鉴** 工具名、参数命名与大致语义，便于工作流迁移；
- 实现细节（路径校验、安全限制、原子补丁算法、控制字符清洗等）均依据
  llm-box 自身的安全模型独立设计。

## License 说明

- Apache License 2.0：见 https://www.apache.org/licenses/LICENSE-2.0
- MIT License：见 https://opensource.org/licenses/MIT

如上述开源项目的许可证要求，本声明文件旨在注明借鉴来源。llm-box 本身的
许可证以其仓库根目录下的 LICENSE 文件为准。
