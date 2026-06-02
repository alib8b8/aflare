# llm-box

> **零代码，终端里搭 AI 流水线，完全离线。**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square)](https://golang.org)
[![License](https://img.shields.io/github/license/alib8b8/llm-box.svg?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/alib8b8/llm-box.svg?style=flat-square&label=Stars)](https://github.com/alib8b8/llm-box/stargazers)
[![Community Nodes](https://img.shields.io/badge/Community%20Nodes-5+-purple.svg?style=flat-square)](nodes/)
[![Platforms](https://img.shields.io/badge/Platforms-Linux%20%7C%20macOS%20%7C%20Windows-blue.svg?style=flat-square)](#-quick-start)

---

## 🎬 Demo

![llm-box demo](https://raw.githubusercontent.com/alib8b8/llm-box/main/docs/demo.svg)

> 📹 录制约 20 秒的终端操作展示：运行命令 → TUI 出现 → 步骤依次成功 → 输出保存。
> 暗色背景 + 高亮字体，录制脚本见 [docs/demo.tape](docs/demo.tape)，可使用 [vhs](https://github.com/charmbracelet/vhs) 本地重新生成 `demo.gif`：
> ```bash
> vhs docs/demo.tape
> ```

---

## ✨ Why llm-box?

- **零代码**: 用简单的 YAML 定义工作流，无需编程
- **全本地**: 数据不出本地，隐私安全
- **永久免费**: MIT 开源协议，完全免费使用
- **社区生态**: 丰富的社区节点，轻松扩展功能

---

## 🚀 Quick Start

### 一键体验（60 秒跑通）

```bash
# 1. 安装（Linux/macOS）
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash

# 2. 前置：安装并启动 Ollama（必须步骤！）
#    下载地址：https://ollama.com/
#    安装后启动服务（不同系统启动方式可能不同）：
#    - macOS/Linux: 运行 ollama serve（或自动后台运行）
#    - Windows: 安装后自动启动服务
ollama pull llama3

# 3. 运行示例（确保 Ollama 服务已启动）
llm-box run examples/basic_summary.yaml
```

### 其它平台

```powershell
# Windows（PowerShell）
# 方案1：手动下载（推荐）
# 访问 https://github.com/alib8b8/llm-box/releases/latest
# 下载 llm-box-windows-amd64.exe，重命名为 llm-box.exe
# 放入系统 PATH 目录或当前目录运行

# 方案2：PowerShell 命令下载
$url = "https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-windows-amd64.exe"
Invoke-WebRequest -Uri $url -OutFile llm-box.exe
.\llm-box.exe run examples/basic_summary.yaml
```

### 本地编译

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go build -o llm-box ./cmd/llm-box
./llm-box run examples/basic_summary.yaml
```

### 运行示例

```bash
# 基础总结：抓网页 → Ollama 总结 → 保存
llm-box run examples/basic_summary.yaml

# 多步骤：抓网页 → 总结 → 翻译 → 保存
llm-box run examples/multi_step.yaml

# 完整示例
llm-box run examples/complete_workflow.yaml

# 调试用
llm-box run examples/test_workflow.yaml
```

---

## 📋 Minimal Example

创建一个极简工作流 `workflow.yaml`：

```yaml
name: "网页总结工作流"
description: "抓取网页 → AI 总结 → 保存结果"

steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  
  - node: ollama
    params:
      model: "llama3"
      prompt: "请用中文总结以下内容：{{input}}"
  
  - node: file_write
    params:
      path: "summary.txt"
```

运行：

```bash
llm-box run workflow.yaml
```

---

## 🧩 Built-in Nodes

### `ollama`
调用本地 Ollama 模型进行 AI 推理

**参数:**
- `model`: 模型名称（默认: `llama3`）
- `endpoint`: Ollama API 地址（默认: `http://localhost:11434`）
- `prompt`: 提示词模板，可用 `{{input}}` 引用输入

### `fetch_url`
抓取网页内容

**参数:**
- `url`: 目标 URL（或通过 input 传入）

**注意事项:**
- 仅抓取服务端渲染的 HTML 内容，不支持 JavaScript 动态渲染（SPA）
- 部分网站有反爬机制或需要登录，可能返回空内容或 4xx/5xx 错误
- 如果抓取结果不符合预期，先用 `curl <url>` 或浏览器"查看网页源代码"确认内容

### `file_write`
将内容写入文件

**参数:**
- `path`: 输出文件路径

### `echo`
输出输入内容（调试用）

---

## 📁 Project Structure

```
llm-box/
├── cmd/llm-box/main.go      # 入口程序
├── internal/
│   ├── workflow/            # 工作流解析与执行
│   ├── nodes/               # 内置节点实现
│   └── tui/                 # 终端界面
├── nodes/                   # 社区贡献节点
├── examples/                # 示例工作流
├── docs/                    # 演示素材
├── go.mod
└── LICENSE
```

---

## 🤝 Contributing

欢迎贡献代码和节点！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

### 贡献节点

1. 在 `nodes/` 创建新目录（如 `nodes/my_node/`）
2. 添加 `metadata.yaml`：
   ```yaml
   name: "my_node"
   description: "我的自定义节点"
   entry: "main.py"
   ```
3. 编写入口脚本（支持任何语言）
4. 提交 PR！

---

## ⚠️ 已知限制与注意事项

### 项目状态
- **尚处于早期开发阶段**：当前 Star 数较少，提交记录集中在最近几天
- **社区测试有限**：可能存在未预料的 bug
- **建议**：先快速体验 `basic_summary.yaml`，生产或关键用途请谨慎使用

### Ollama 依赖
- 所有使用 `ollama` 节点的工作流需要本地安装并运行 Ollama
- 首次使用需执行 `ollama pull llama3`（或其他模型）
- 如果报连接错误，请检查 Ollama 服务是否正在运行

### 网页抓取限制
- `fetch_url` 节点仅支持服务端渲染的网页
- 不支持 JavaScript 动态渲染（SPA）、需要登录或有反爬机制的网站
- 抓取前建议先用 `curl <url>` 确认内容

---

## 📄 License

MIT License - 详见 [LICENSE](LICENSE)

---

⭐ 如果喜欢这个项目，请给个 Star！
