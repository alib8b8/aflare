# aflare 教程体系

## 目录

1. [快速入门](#快速入门) - 60秒开始使用
2. [基础教程](#基础教程) - 核心概念与基本操作
3. [进阶教程](#进阶教程) - 自定义节点与复杂工作流
4. [最佳实践](#最佳实践) - 性能优化与生产部署
5. [常见问题](#常见问题) - FAQ 与故障排除

---

## 快速入门

### 安装

**Linux/macOS:**
```bash
curl -sL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

**Windows:**
```powershell
Invoke-WebRequest -Uri "https://github.com/alib8b8/aflare/releases/latest/download/aflare-windows-amd64.exe" -OutFile aflare.exe
```

**从源码构建:**
```bash
git clone https://github.com/alib8b8/aflare.git
cd aflare
go build -o aflare ./cmd/aflare
```

### 你的第一个工作流

**创建工作流:**
```bash
# 使用自然语言创建工作流
aflare create "fetch Hacker News top 5 stories and save to hn.txt"
```

这会生成一个 YAML 文件 `hn_workflow.yaml`:
```yaml
name: hn_workflow
steps:
  - node: fetch_url
    params:
      url: https://hacker-news.firebaseio.com/v0/topstories.json
  - node: transform
    params:
      operation: slice
      count: 5
  - node: file_write
    params:
      path: hn.txt
```

**运行工作流:**
```bash
aflare run hn_workflow.yaml
```

**查看结果:**
```bash
cat hn.txt
```

### 验证安装

```bash
# 检查版本
aflare version

# 查看帮助
aflare help

# 列出可用节点
aflare nodes
```

---

## 基础教程

### 核心概念

**工作流 (Workflow):**
- 一个工作流是一系列步骤的集合
- 每个步骤是一个节点的执行
- 步骤之间可以传递数据

**节点 (Node):**
- 节点是工作流的基本执行单元
- 每个节点有特定的功能（获取数据、转换、执行命令等）
- 节点可以链式组合

**数据流:**
- 数据在工作流中自动流转
- 上一步的输出自动成为下一步的输入
- 使用 `${steps.step_name.output}` 引用之前步骤的结果

### 内置节点

#### 1. fetch_url - 获取网络数据
```yaml
- node: fetch_url
  params:
    url: https://api.example.com/data
    method: GET  # 可选: GET, POST, PUT, DELETE
    headers:     # 可选: 自定义请求头
      Authorization: "Bearer ${env.API_TOKEN}"
```

#### 2. transform - 数据转换
```yaml
- node: transform
  params:
    operation: extract  # extract, filter, map, slice, combine
    path: "data.items"  # JSON 路径
    filter: "status == 'active'"  # 过滤条件
```

#### 3. execute - 执行命令
```yaml
- node: execute
  params:
    command: "git log --oneline -10"
    cwd: "."  # 工作目录
    env:      # 环境变量
      GIT_DIR: "/path/to/repo"
```

#### 4. file_write - 写入文件
```yaml
- node: file_write
  params:
    path: output.txt
    content: "${steps.transform.output}"
    mode: overwrite  # overwrite, append
```

#### 5. notify - 发送通知
```yaml
- node: notify
  params:
    channel: stdout  # stdout, slack, email
    message: "任务完成！"
```

#### 6. combine - 合并数据
```yaml
- node: combine
  params:
    format: json  # json, yaml, markdown
    sources:
      - "${steps.fetch1.output}"
      - "${steps.fetch2.output}"
```

#### 7. ollama - 本地 LLM 推理
```yaml
- node: ollama
  params:
    model: llama2
    prompt: "总结以下内容: ${steps.fetch.output}"
    base_url: http://localhost:11434
```

### 工作流示例

#### 每日 GitHub 摘要
```yaml
name: github-daily
env:
  GH_TOKEN: "${env.GITHUB_TOKEN}"

steps:
  - name: fetch_activity
    node: execute
    params:
      command: gh activity --user ${env.GITHUB_USER}

  - name: summarize
    node: ollama
    params:
      model: llama2
      prompt: |
        总结以下 GitHub 活动:
        ${steps.fetch_activity.output}

  - name: save
    node: file_write
    params:
      path: github-digest.md
      content: "${steps.summarize.output}"
```

#### API 数据收集器
```yaml
name: api-collector

steps:
  - name: fetch_weather
    node: fetch_url
    params:
      url: https://api.weather.gov/forecast

  - name: fetch_stocks
    node: fetch_url
    params:
      url: https://api.stock.example.com/quote/AAPL

  - name: combine_data
    node: combine
    params:
      format: markdown
      sources:
        - "${steps.fetch_weather.output}"
        - "${steps.fetch_stocks.output}"

  - name: save_report
    node: file_write
    params:
      path: daily-report.md
      content: "${steps.combine_data.output}"
```

---

## 进阶教程

### 自定义节点

#### 创建自定义节点

1. **创建节点目录:**
```bash
mkdir -p nodes/my-custom-node
cd nodes/my-custom-node
```

2. **创建节点配置 `node.yaml`:**
```yaml
name: my-custom-node
version: "1.0.0"
description: "我的自定义节点"
author: "your-name"

inputs:
  - name: text
    type: string
    required: true
    description: "输入文本"
  - name: option
    type: string
    default: "default"
    description: "可选参数"

outputs:
  - name: result
    type: string
    description: "处理结果"
```

3. **实现节点逻辑 `run.sh`:**
```bash
#!/bin/bash
# 从 stdin 读取 JSON 输入
INPUT=$(cat)

# 解析参数
TEXT=$(echo "$INPUT" | jq -r '.text')
OPTION=$(echo "$INPUT" | jq -r '.option')

# 处理逻辑
RESULT="Processed: $TEXT (option: $OPTION)"

# 返回 JSON 输出
echo "{\"result\": \"$RESULT\"}"
```

4. **使用自定义节点:**
```yaml
steps:
  - node: my-custom-node
    params:
      text: "Hello, World!"
      option: "custom"
```

### 复杂工作流模式

#### 并行执行
```yaml
name: parallel-fetch

steps:
  # 并行获取多个数据源
  - name: fetch_api1
    node: fetch_url
    params:
      url: https://api1.example.com/data
    parallel: true

  - name: fetch_api2
    node: fetch_url
    params:
      url: https://api2.example.com/data
    parallel: true

  - name: fetch_api3
    node: fetch_url
    params:
      url: https://api3.example.com/data
    parallel: true

  # 等待所有并行任务完成
  - name: wait_all
    node: combine
    params:
      format: json
      wait_for:
        - fetch_api1
        - fetch_api2
        - fetch_api3
```

#### 条件执行
```yaml
name: conditional-workflow

steps:
  - name: check_status
    node: fetch_url
    params:
      url: https://api.example.com/health

  - name: handle_success
    node: notify
    params:
      message: "服务正常"
    when: "${steps.check_status.output.status == 'ok'}"

  - name: handle_failure
    node: notify
    params:
      message: "服务异常！"
      channel: slack
    when: "${steps.check_status.output.status != 'ok'}"
```

#### 循环处理
```yaml
name: process-items

steps:
  - name: fetch_items
    node: fetch_url
    params:
      url: https://api.example.com/items

  - name: process_each
    node: transform
    params:
      operation: map
      items: "${steps.fetch_items.output.items}"
      workflow: |
        name: process-single
        steps:
          - node: ollama
            params:
              model: llama2
              prompt: "分析: ${item}"
```

### 与外部系统集成

#### Slack 集成
```yaml
steps:
  - name: notify_slack
    node: notify
    params:
      channel: slack
      webhook_url: "${env.SLACK_WEBHOOK}"
      message: |
        📊 每日报告
        完成任务数: ${steps.stats.output.completed}
        失败任务数: ${steps.stats.output.failed}
```

#### GitHub API
```yaml
steps:
  - name: create_issue
    node: execute
    params:
      command: |
        gh issue create \
          --title "自动报告 - $(date +%Y-%m-%d)" \
          --body "${steps.report.output}" \
          --label automated
```

---

## 最佳实践

### 性能优化

#### 1. 减少不必要的 API 调用
```yaml
# ❌ 不好的做法
steps:
  - name: fetch_each
    node: fetch_url
    params:
      url: "https://api.example.com/item/${id}"
    # 每个项目单独请求

# ✅ 好的做法
steps:
  - name: fetch_batch
    node: fetch_url
    params:
      url: "https://api.example.com/items?ids=1,2,3"
    # 批量获取
```

#### 2. 使用缓存
```yaml
steps:
  - name: cached_fetch
    node: fetch_url
    params:
      url: https://api.example.com/data
      cache:
        enabled: true
        ttl: 300  # 秒
```

#### 3. 限制并发
```yaml
# 全局并发限制
config:
  max_concurrent: 5
  timeout: 300

steps:
  # ...
```

### 错误处理

#### 重试策略
```yaml
steps:
  - name: unreliable_api
    node: fetch_url
    params:
      url: https://unreliable-api.example.com
    retry:
      max_attempts: 3
      backoff: exponential
      delay: 1s
```

#### 错误恢复
```yaml
steps:
  - name: primary_action
    node: fetch_url
    params:
      url: https://primary.example.com
    on_error:
      - node: notify
        params:
          message: "主服务失败，尝试备用"

  - name: fallback_action
    node: fetch_url
    params:
      url: https://backup.example.com
    when: "${steps.primary_action.failed}"
```

### 安全最佳实践

#### 1. 使用环境变量
```yaml
# ❌ 不好的做法
steps:
  - node: fetch_url
    params:
      url: https://api.example.com
      headers:
        Authorization: "Bearer sk-xxxxx"  # 硬编码密钥

# ✅ 好的做法
steps:
  - node: fetch_url
    params:
      url: https://api.example.com
      headers:
        Authorization: "Bearer ${env.API_KEY}"  # 环境变量
```

#### 2. 输入验证
```yaml
config:
  validate_inputs: true
  allowed_hosts:
    - api.example.com
    - cdn.example.com
```

#### 3. 最小权限原则
```yaml
# 只授予必要的权限
steps:
  - node: file_write
    params:
      path: ./output/
      content: "${data}"
      # 不能写入其他目录
```

### 生产部署

#### Docker 部署
```dockerfile
FROM golang:1.25-alpine

WORKDIR /app
COPY . .

RUN go build -o aflare ./cmd/aflare

ENV LLMBOX_CONFIG=/app/config.yaml
ENV LLMBOX_LOG_LEVEL=info

ENTRYPOINT ["./aflare"]
```

```bash
docker build -t aflare .
docker run -v $(pwd)/workflows:/app/workflows aflare run my-workflow.yaml
```

#### Kubernetes 部署
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
spec:
  schedule: "0 9 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: aflare
            image: aflare:latest
            command: ["./aflare", "run", "daily-report.yaml"]
            env:
            - name: API_KEY
              valueFrom:
                secretKeyRef:
                  name: llmbox-secrets
                  key: api-key
          restartPolicy: OnFailure
```

---

## 常见问题

### FAQ

**Q: 如何调试工作流？**
```bash
# 使用 verbose 模式查看详细日志
aflare run workflow.yaml --verbose

# 检查单个节点输出
aflare run workflow.yaml --step fetch_data --dry-run
```

**Q: 如何处理大型数据？**
```yaml
config:
  stream_mode: true  # 流式处理
  chunk_size: 1024   # 分块大小

steps:
  - node: transform
    params:
      operation: stream  # 流式转换
```

**Q: 如何分享工作流？**
```bash
# 打包工作流及其依赖
aflare package my-workflow.yaml -o my-workflow.tar.gz

# 导入工作流
aflare import my-workflow.tar.gz
```

**Q: 如何监控运行状态？**
```bash
# 实时监控
aflare monitor

# 查看 Web UI
aflare webui --port 8080
```

### 故障排除

#### 问题：节点执行超时
```yaml
# 解决方案：增加超时时间
config:
  timeout: 600  # 10 分钟

steps:
  - node: slow_api
    params:
      timeout: 300  # 单独设置
```

#### 问题：内存不足
```yaml
# 解决方案：启用流式处理
config:
  stream_mode: true
  max_memory: 512MB

steps:
  - node: large_file
    params:
      stream: true
```

#### 问题：API 速率限制
```yaml
# 解决方案：添加速率限制和重试
config:
  rate_limit:
    requests_per_second: 10
    burst: 20

steps:
  - node: api_call
    params:
      retry:
        max_attempts: 5
        backoff: exponential
```

---

## 下一步

- 📖 阅读 [API 参考](./api-reference.md)
- 🔧 学习 [自定义节点开发](./custom-nodes.md)
- 🌐 了解 [分布式执行](./distributed.md)
- 🤝 加入 [社区讨论](https://github.com/alib8b8/aflare/discussions)

---

## 贡献教程

发现教程有问题或有改进建议？欢迎贡献！

1. Fork 仓库
2. 编辑 `docs/tutorial.md`
3. 提交 PR

我们欢迎任何形式的贡献，包括：
- 修正错别字
- 添加新示例
- 改进解释
- 翻译文档