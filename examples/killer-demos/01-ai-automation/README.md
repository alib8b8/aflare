# AI 每日自动化 — Killer Demo

> AI-powered daily automation: email digest, BTC monitoring, and news summary.

一个展示 aflare 全部核心能力的杀手级演示：**DAG 并行执行、定时调度、LLM Agent、多源数据聚合、Telegram 通知**。

---

## 它能做什么

每天早上 8:00，aflare 自动执行以下任务：

| 任务 | 数据源 | 方式 |
|------|--------|------|
| 📧 邮件摘要 | IMAP 邮箱（未读邮件） | Python 脚本通过 IMAP 拉取，LLM 分类摘要 |
| 💰 BTC 行情 | CoinGecko API | HTTP 请求获取实时价格、24h 涨跌、市值 |
| 🤖 AI 新闻 | Hacker News / Reddit / 新闻源 | `search_aggregate` 多平台聚合 |

三项数据采集**并行执行**（DAG），完成后由 LLM Agent 生成结构化每日简报，保存为 Markdown 文件，并通过 Telegram 推送通知。

### 预期输出示例

```markdown
# 📊 Daily Digest - 2026-08-07

## 💰 Bitcoin Price
- **Current:** $87,432 USD
- **24h Change:** +2.3%
- **Market Cap:** $1.73T
- **Sentiment:** Bullish — breaking above 200-day MA

## 📧 Email Digest (3 unread)

| Priority | From | Subject | Category | Action |
|----------|------|---------|----------|--------|
| 🔴 Critical | boss@company.com | Q3 budget review due today | Work | Respond with numbers |
| 🟢 Medium | newsletter@aiweekly.com | This week in AI | Newsletter | Read later |
| ⚪ Low | noreply@github.com | PR #342 merged | Work | None |

## 🤖 AI News
1. **OpenAI releases GPT-5 with reasoning improvements** (HN, 847 points)
   → Major leap in multi-step reasoning, 40% better on MATH benchmark
2. ...

## 📋 Today's Focus
1. [ ] Submit Q3 budget review
2. [ ] Review BTC portfolio allocation
3. [ ] Read OpenAI GPT-5 technical paper
```

---

## 快速开始

### 1. 安装 aflare

```bash
# Linux / macOS
curl -sL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash

# 验证安装
aflare --help
```

### 2. 配置密钥

aflare 使用内置的 secrets 管理，敏感信息不会出现在工作流文件中：

```bash
# 邮箱配置（IMAP）
aflare secrets set email.imap_server imap.gmail.com
aflare secrets set email.user your-email@gmail.com
aflare secrets set email.pass "your-app-password"

# Telegram 通知（可选）
aflare secrets set telegram.token "123456:ABC-DEF1234ghikl"
aflare secrets set telegram.chat_id "123456789"

# LLM API Key（OpenAI / DeepSeek 等）
export OPENAI_API_KEY="sk-..."
```

> **Gmail 用户**：需要开启 IMAP 并使用[应用专用密码](https://support.google.com/accounts/answer/185833)。
> **国内用户**：可将 `provider` 改为 `deepseek`，`model` 改为 `deepseek-chat`。

### 3. 运行工作流

```bash
# 进入 demo 目录
cd examples/killer-demos/01-ai-automation

# 立即运行一次
aflare run workflow.yaml

# 设置每天 8:00 自动运行
aflare schedule --cron "0 8 * * *" workflow.yaml

# 查看定时任务
aflare schedule --list
```

### 4. 查看输出

```bash
# 报告保存在 reports/ 目录下
ls reports/
# daily-digest-2026-08-07.md

cat reports/daily-digest-2026-08-07.md
```

---

## 工作流架构

```
                           ┌──────────────────────────────────┐
                           │       schedule: "0 8 * * *"      │
                           │       每天 8:00 自动触发          │
                           └────────────────┬─────────────────┘
                                            │
              ┌─────────────────────────────┼─────────────────────────────┐
              │                             │                             │
    ┌─────────▼──────────┐     ┌───────────▼───────────┐     ┌───────────▼──────────┐
    │  Branch A: 邮件     │     │  Branch B: BTC 行情    │     │  Branch C: AI 新闻    │
    │  execute: Python    │     │  http_request:         │     │  search_aggregate:    │
    │  IMAP 拉取未读邮件  │     │  CoinGecko API         │     │  HN/Reddit/News       │
    │  continue_on_error  │     │  retry: 2              │     │  time_range: day      │
    └─────────┬──────────┘     └───────────┬───────────┘     └───────────┬──────────┘
              │                             │                             │
              └─────────────┬───────────────┴─────────────┬───────────────┘
                            │  output_strategy: join      │
                            │  (合并三个分支的输出)        │
                            └─────────────┬───────────────┘
                                          │
                                ┌─────────▼──────────┐
                                │  Stage 2: LLM 处理  │
                                │  agent(gpt-4o):     │
                                │  - 邮件分类/优先级  │
                                │  - BTC 行情分析     │
                                │  - 新闻精选摘要     │
                                │  - 今日行动建议     │
                                └─────────┬──────────┘
                                          │
                      ┌───────────────────┼───────────────────┐
                      │                   │                   │
            ┌─────────▼──────┐  ┌────────▼────────┐  ┌───────▼────────┐
            │  save_report   │  │  notify_telegram │  │    console     │
            │  file_write:   │  │  channel:        │  │  channel:      │
            │  reports/      │  │  telegram        │  │  stdout        │
            │  daily-digest  │  │  Bot Token +     │  │  (终端输出)    │
            │  -DATE.md      │  │  Chat ID         │  │                │
            └────────────────┘  └──────────────────┘  └────────────────┘
```

### 核心设计要点

| 特性 | 实现方式 |
|------|----------|
| **DAG 并行** | `parallel` 步骤，三路并发采集数据 |
| **容错降级** | `continue_on_error: true` — 单路失败不影响整体 |
| **重试策略** | `retry` + `backoff`（指数退避），BTC API 重试 2 次 |
| **定时调度** | `schedule.cron: "0 8 * * *"` — 标准 cron 表达式 |
| **密钥管理** | `{{secret.xxx}}` 模板变量，不硬编码密钥 |
| **LLM Agent** | `agent` 节点启用 thinking 模式，工具调用 `json_parse,transform` |
| **多通道输出** | `file_write` 保存 Markdown + `notify` 推送到 Telegram + stdout |

---

## 自定义配置

### 更换 LLM 提供商

编辑 `workflow.yaml` 中的 `vars` 部分：

```yaml
# 使用 DeepSeek（国内推荐）
provider: deepseek
model: deepseek-chat

# 使用本地 Ollama
provider: ollama
model: llama3

# 使用 Qwen
provider: qwen
model: qwen-max
```

### 调整邮件拉取数量

```yaml
max_emails: "50"   # 默认 20
```

### 添加更多数据源

在 `parallel` 块中新增分支即可，例如添加股票行情：

```yaml
- name: fetch_stocks
  node: http_request
  params:
    url: "https://api.example.com/stocks/AAPL,TSLA"
    method: GET
    timeout: "30s"
  continue_on_error: true
```

### 修改定时策略

```yaml
# 工作日早上 8:00
schedule:
  cron: "0 8 * * 1-5"

# 每天 8:00 和 18:00
schedule:
  cron: "0 8,18 * * *"

# 每小时一次
schedule:
  cron: "0 * * * *"
```

### 不使用 Telegram，改用其他通知

```yaml
# Slack Webhook
- name: notify_slack
  node: notify
  params:
    channel: webhook
    url: "https://hooks.slack.com/services/xxx"
    body: '{"text":"Daily digest ready!"}'

# 邮件通知
- name: notify_email
  node: notify
  params:
    channel: webhook
    url: "https://api.sendgrid.com/v3/mail/send"
    method: POST
    headers: '{"Authorization":"Bearer {{secret.sendgrid.key}}"}'
    body: '{"personalizations":[{"to":[{"email":"you@example.com"}]}],"from":{"email":"bot@example.com"},"subject":"Daily Digest","content":[{"type":"text/plain","value":"{{step.generate_digest}}"}]}'
```

---

## 依赖项

- **aflare** ≥ 最新版本
- **Python 3**（用于 IMAP 邮件拉取）
- **LLM API Key**：OpenAI / DeepSeek / Ollama（任选其一）
- **可选**：Telegram Bot Token + Chat ID（用于通知）

---

## 故障排查

### 邮件拉取失败

```bash
# 检查 IMAP 连接
python3 -c "
import imaplib
mail = imaplib.IMAP4_SSL('imap.gmail.com', 993)
mail.login('your-email@gmail.com', 'your-app-password')
print('OK')
mail.logout()
"
```

### BTC API 限流

CoinGecko 免费 API 有频率限制。如果遇到 429，工作流会自动 retry。可在 `backoff` 中调大延迟。

### Telegram 通知未送达

```bash
# 验证 Bot Token
curl "https://api.telegram.org/bot<YOUR_TOKEN>/getMe"

# 获取 Chat ID（先给 Bot 发一条消息）
curl "https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates"
```

---

## 文件结构

```
01-ai-automation/
├── workflow.yaml      # 工作流定义（核心）
├── README.md          # 本文档
└── reports/           # 生成的日报（自动创建）
    └── daily-digest-2026-08-07.md
```

---

## 相关资源

- [aflare 文档](https://github.com/alib8b8/aflare)
- [节点参考](https://github.com/alib8b8/aflare/blob/main/docs/nodes-reference.md)
- [定时调度指南](https://github.com/alib8b8/aflare/blob/main/docs/scheduling.md)
- [数据流指南](https://github.com/alib8b8/aflare/blob/main/docs/dataflow.md)