# llm-box

> 终端里的零代码 AI 工作流引擎

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg?style=flat-square)](https://golang.org)
[![License](https://img.shields.io/github/license/alib8b8/HKAIC.svg?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/alib8b8/HKAIC.svg?style=flat-square&label=Stars)](https://github.com/alib8b8/HKAIC/stargazers)
[![Community Nodes](https://img.shields.io/badge/Community%20Nodes-5+-purple.svg?style=flat-square)](nodes/)

---

## 🎬 Demo

![llm-box demo](https://raw.githubusercontent.com/alib8b8/HKAIC/main/docs/demo.gif)

---

## ✨ Why llm-box?

- **零代码**: 用简单的 YAML 定义工作流，无需编程
- **全本地**: 数据不出本地，隐私安全
- **永久免费**: MIT 开源协议，完全免费使用
- **社区生态**: 丰富的社区节点，轻松扩展功能

---

## 🚀 Quick Start

### 一键安装

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/HKAIC/main/install.sh | bash
```

### 前置要求

1. 安装 [Ollama](https://ollama.com/)
2. 启动 Ollama 服务
3. 拉取一个模型（如 `ollama pull llama3`）

### 运行示例

```bash
# 基础总结示例
llm-box examples/basic_summary.yaml

# 多步骤工作流
llm-box examples/multi_step.yaml

# 完整示例
llm-box examples/complete_workflow.yaml
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
llm-box workflow.yaml
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

## 📄 License

MIT License - 详见 [LICENSE](LICENSE)

---

⭐ 如果喜欢这个项目，请给个 Star！