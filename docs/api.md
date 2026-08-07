# REST API Reference

aflare 提供两套 HTTP API：

- **WebUI API**（`aflare webui`）：面向可视化编辑器，默认端口 `8081`，绑定 `127.0.0.1`
- **Workflow Execution API**（`aflare serve`）：面向编程集成，默认端口 `8080`，绑定 `0.0.0.0`

---

## 认证

### WebUI API

通过 `X-Auth-Token` 请求头进行 Token 认证（可选，取决于是否设置 `AFLARE_WEBUI_AUTH_TOKEN` 环境变量）：

```bash
curl -H "X-Auth-Token: your-token" http://localhost:8081/api/workflows
```

Token 使用 `crypto/subtle.ConstantTimeCompare` 进行常量时间比较，防止时序攻击。

### Workflow Execution API

通过 `X-API-Key` 请求头或 `Authorization: Bearer <key>` 进行 API Key 认证。`/health` 和 `/api/v1/metrics` 端点无需认证。

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/workflows
```

---

## WebUI API（端口 8081）

### `GET /`

返回 WebUI 编辑器 HTML 页面。无需认证。

### `GET /api/workflows`

列出工作流目录中所有 `.yaml` / `.yml` 文件。

**请求示例：**

```bash
curl -H "X-Auth-Token: your-token" http://localhost:8081/api/workflows
```

**响应示例：**

```json
{
  "workflows": ["daily-report", "btc-monitor", "log-analyzer"],
  "directory": "/home/user/aflare"
}
```

### `GET /api/workflow?name=<name>`

获取指定工作流的 YAML 内容。

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `name` | string | 是 | 工作流名称（不含扩展名） |

**约束：** 名称长度 1-100 字符，仅允许 `[a-zA-Z0-9._-]`，禁止 `..`、`/`、`\`。

**请求示例：**

```bash
curl -H "X-Auth-Token: your-token" "http://localhost:8081/api/workflow?name=daily-report"
```

**响应示例：**（Content-Type: `text/yaml`）

```yaml
name: Daily Report
description: Generate daily summary report
steps:
  - node: fetch_url
    params:
      url: "https://api.example.com/data"
  - node: agent
    params:
      prompt: "Summarize this data: {{step.0}}"
```

### `POST /api/workflow`

保存工作流。若文件已存在则覆盖。

**请求体：**

```json
{
  "name": "my-workflow",
  "content": "name: My Workflow\ndescription: ...\nsteps:\n  - node: fetch_url\n    params:\n      url: https://example.com"
}
```

**响应：** `201 Created`

```json
{
  "status": "saved",
  "path": "/home/user/workflows/my-workflow.yaml"
}
```

### `DELETE /api/workflow?name=<name>`

删除指定工作流。

**请求示例：**

```bash
curl -X DELETE -H "X-Auth-Token: your-token" "http://localhost:8081/api/workflow?name=my-workflow"
```

**响应：** `200 OK`

```json
{
  "status": "deleted"
}
```

### `POST /api/validate`

验证工作流 YAML 内容是否正确。

**请求体：**

```json
{
  "workflow": "name: test\nsteps:\n  - node: agent\n    params:\n      prompt: hello"
}
```

**响应（有效）：** `200 OK`

```json
{
  "valid": true,
  "error": "",
  "warnings": [],
  "name": "test",
  "steps": 1
}
```

**响应（无效）：** `400 Bad Request`

```json
{
  "valid": false,
  "error": "failed to parse workflow: ...",
  "warnings": []
}
```

### `POST /api/visualize`

将工作流 YAML 转换为可视化格式。

**查询参数：**

| 参数 | 值 | 默认 | 说明 |
|------|------|:----:|------|
| `format` | `mermaid` / `json` / `dot` / `ascii` | `json` | 输出格式 |

**请求体：**

```json
{
  "workflow": "name: test\nsteps:\n  - node: agent\n    params:\n      prompt: hello"
}
```

**curl 示例：**

```bash
# Mermaid 流程图
curl -X POST -H "X-Auth-Token: token" \
  -H "Content-Type: application/json" \
  -d '{"workflow":"name: test\nsteps:\n  - node: agent\n    params:\n      prompt: hello"}' \
  "http://localhost:8081/api/visualize?format=mermaid"

# DOT 格式
curl -X POST -H "X-Auth-Token: token" \
  -H "Content-Type: application/json" \
  -d '{"workflow":"...yaml..."}' \
  "http://localhost:8081/api/visualize?format=dot"
```

### `GET /metrics`（条件启用）

Prometheus 指标端点。仅在设置 `AFLARE_METRICS=1` 时启用。**不受认证中间件保护**（符合 Prometheus scrape 惯例），但受令牌桶限流（约 5 req/s）。

### `/debug/pprof/`（条件启用）

Go pprof 性能分析端点。仅在设置 `AFLARE_PPROF=1` 时启用，受 `X-Auth-Token` 认证保护。

---

## Workflow Execution API（端口 8080）

### `GET /health`

健康检查端点，无需认证。

**请求：**

```bash
curl http://localhost:8080/health
```

**响应：** `200 OK`

```json
{
  "status": "ok",
  "version": "1.0.0",
  "uptime": "2026-08-07T10:00:00Z"
}
```

### `GET /api/v1/metrics`

Prometheus 指标端点，无需认证。

### `POST /api/v1/workflows/run`

执行工作流。这是核心工作流执行端点。

**请求体：**

```json
{
  "workflow": "name: My Workflow\nsteps:\n  - node: fetch_url\n    params:\n      url: https://example.com\n  - node: agent\n    params:\n      prompt: \"Summarize: {{step.0}}\"",
  "timeout": "5m"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `workflow` | string | 是 | YAML 格式的工作流定义 |
| `timeout` | string | 否 | 超时时间，支持 `30s`、`5m`、`1h` 等格式 |

**响应（成功）：** `200 OK`

```json
{
  "success": true,
  "output": "Summary of the fetched content...",
  "step_results": [
    {
      "step_index": 0,
      "node_name": "fetch_url",
      "input": "https://example.com",
      "output": "<html>...</html>",
      "error": "",
      "duration": "1.2s"
    },
    {
      "step_index": 1,
      "node_name": "agent",
      "input": "Summarize: <html>...</html>",
      "output": "Summary of the fetched content...",
      "error": "",
      "duration": "3.5s"
    }
  ],
  "error": "",
  "duration": "4.7s"
}
```

**响应（失败）：** `500 Internal Server Error`

```json
{
  "success": false,
  "output": "",
  "step_results": [
    {
      "step_index": 0,
      "node_name": "fetch_url",
      "input": "https://invalid.example.com",
      "output": "",
      "error": "connection refused",
      "duration": "30s"
    }
  ],
  "error": "step 0 failed: connection refused",
  "duration": "30s"
}
```

**curl 示例：**

```bash
curl -X POST http://localhost:8080/api/v1/workflows/run \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "workflow": "name: test\nsteps:\n  - node: agent\n    params:\n      prompt: Say hello",
    "timeout": "30s"
  }'
```

### `GET /api/v1/workflows`

列出工作流目录中所有可用工作流（含元信息）。

**响应：** `200 OK`

```json
{
  "workflows": [
    {
      "name": "Daily Report",
      "description": "Generate daily summary report",
      "steps": 3,
      "file": "daily-report.yaml"
    },
    {
      "name": "BTC Monitor",
      "description": "Monitor BTC price and alert",
      "steps": 4,
      "file": "btc-monitor.yaml"
    }
  ]
}
```

### `GET /api/v1/workflows/{name}`

获取指定工作流的详细信息（步骤列表）。

**请求：**

```bash
curl -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/workflows/daily-report"
```

**响应：** `200 OK`

```json
{
  "name": "Daily Report",
  "description": "Generate daily summary report",
  "file": "daily-report.yaml",
  "steps": [
    {
      "node": "fetch_url",
      "name": "fetch_data",
      "params": {
        "url": "https://api.example.com/data"
      }
    },
    {
      "node": "agent",
      "name": "summarize",
      "params": {
        "prompt": "Summarize: {{step.0}}"
      }
    }
  ],
  "step_count": 2
}
```

---

## 错误码参考

| HTTP 状态码 | 说明 |
|-------------|------|
| `200` | 成功 |
| `201` | 创建成功（工作流保存） |
| `400` | 请求格式错误、参数缺失、工作流解析失败 |
| `401` | 认证失败（API Key 或 Token 无效） |
| `404` | 工作流不存在 |
| `405` | HTTP 方法不支持 |
| `413` | 请求体过大（超过 5MB） |
| `429` | 请求频率过高（`/metrics` 端点） |
| `500` | 服务器内部错误、工作流执行失败 |

---

## 请求限制

- 工作流 YAML 内容最大 **5MB**（`POST /api/visualize`、`POST /api/validate`、`POST /api/workflow`）
- `/metrics` 端点限流 **5 req/s**（令牌桶算法）
- 服务器读写超时：WebUI 30s，API 读 30s / 写 60s

---

## 相关文档

- [WebUI 编辑器](webui.md) — 可视化工作流编辑器
- [入门指南](getting-started.md) — 工作流 YAML 语法
- [部署指南](deployment.md) — 生产环境部署
- [README](../README.md) — 项目概览