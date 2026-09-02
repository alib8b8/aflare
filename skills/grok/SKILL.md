---
name: aflare
description: 终端工作流自动化引擎，通过自然语言生成并执行 YAML 工作流。内置节点与 LLM 提供商目录可通过 aflare list 查看。
invocation: both
---

# aflare Grok Skill

## 一句话调用

当用户说类似以下内容时，直接使用 aflare：

**创建工作流：**
- "用 aflare 创建一个备份工作流"
- "用 aflare 创建 GitHub 每日总结工作流"
- "用 aflare 监控日志并在出错时通知我"
- "用 aflare 把 Markdown 文章转为 HTML 并保存"

**执行工作流：**
- "运行 aflare 工作流：每天抓取 Hacker News 前 10 条"
- "执行 workflow.yaml"
- "运行这个工作流：{yaml_content}"

**其他操作：**
- "查看 aflare 有哪些可用节点"
- "验证这个工作流文件是否正确"

## 调用流程

```
用户输入 → create_workflow(描述) → 获取 YAML → run_workflow_yaml(YAML) → 返回结果
```

1. **第一步**：调用 `create_workflow` 工具，传入用户描述
2. **第二步**：将返回的 YAML 内容传入 `run_workflow_yaml` 工具
3. **第三步**：将执行结果返回给用户

## 可用工具

| 工具 | 参数 | 说明 |
|------|------|------|
| `create_workflow` | `description: string` | 从自然语言生成 YAML 工作流 |
| `run_workflow` | `file: string` | 执行指定路径的工作流文件 |
| `run_workflow_yaml` | `yaml: string` | 直接执行 YAML 内容 |
| `list_nodes` | 无 | 列出所有可用节点 |
| `validate_workflow` | `file: string` | 验证工作流文件（不执行） |

## 核心节点

**工具类：** fetch_url, http_request, file_read, file_write, execute, json_parse, template_render, transform, combine, notify

**LLM 类：** ollama, deepseek, openai, qwen, glm, kimi, mistral, yi

**控制类：** condition, call

## 安全说明

- URL 抓取有 SSRF 防护
- 文件操作有路径遍历防护
- 命令执行有注入防护
- 支持 safe mode 禁用 execute 节点

## 安装与连接

### 安装 aflare

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh -o install.sh
bash install.sh
```

### 启动 MCP 服务

```bash
aflare --mcp-server
```

### Grok Web 连接

在 grok.com/connectors 添加自定义 MCP：
- 类型：Remote MCP (Streamable HTTP)
- URL：http://localhost:8080/mcp

## 使用示例

### 示例 1：创建并执行工作流

用户："用 aflare 创建一个抓取 Hacker News 前 5 条并保存的工作流"

执行步骤：
1. 调用 `create_workflow("fetch top 5 Hacker News stories and save to file")`
2. 获取生成的 YAML
3. 调用 `run_workflow_yaml(YAML内容)`
4. 返回执行结果

### 示例 2：验证工作流

用户："帮我检查 workflow.yaml 是否正确"

执行步骤：
1. 调用 `validate_workflow("workflow.yaml")`
2. 返回验证结果
