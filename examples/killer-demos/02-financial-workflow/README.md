# 🏦 企业金融工作流 — 跨行转账全链路 Demo

> **Killer Demo**: 一条 YAML 同时解决 **分布式事务一致性**、**AI 风控**、**合规审计** 三大金融痛点。

## 这个 Demo 展示什么

一个真实企业级金融转账场景的端到端工作流，覆盖从风控到执行到审计的完整链路：

| 能力 | 实现机制 | 在本 Demo 中的体现 |
|------|----------|-------------------|
| 🛡️ **LLM 欺诈检测** | 转账前 AI 风险分析 | `fraud_check` 步骤使用 GPT-4o 分析交易模式，异常金额/黑名单/行为模式，输出结构化风险评分 |
| 🔄 **Saga 事务补偿** | forward 步骤 + 反向 compensate | `debit → credit → notify`，credit 失败时自动执行 `refund_debit`（反向补偿） |
| 📜 **HMAC 审计链** | executor 内置 HMAC 哈希链自动落盘 | 所有 forward/compensate 步骤的输入/输出/耗时/重试自动记录，防篡改 |
| 🔑 **幂等性** | Idempotency-Key 双重保障 | 客户端幂等（SDK `WithIdempotencyKey`）+ 服务端去重，同一笔转账只执行一次 |
| 🚦 **限流 + 重试 + 退避** | per-host 令牌桶 + 指数退避 + 抖动 | 每个 forward/compensate 步骤都配置 `rate_limit_rps=5`、`max_retries=3`、指数退避 |
| 🛑 **capture_error** | 错误即分支，不崩溃 | LLM 不可用时降级为"人工审核"，不阻塞转账流程 |
| 📬 **多通道通知** | stdout / webhook / slack / discord | 转账完成后输出结构化报告 |
| 🏷️ **数据脱敏** | 自动 PII 脱敏 | 审计日志中敏感字段自动 redact |

## 为什么需要 Saga 模式

传统金融系统的分布式事务通常依赖两阶段提交（2PC），但 2PC 在跨银行、跨系统场景下有明显缺陷：

- 🔒 **锁持有时间长**：协调者故障导致资源长期锁定
- 🌐 **跨组织不可行**：不同银行的数据库无法参与同一个 2PC 事务
- 📉 **可用性低**：协调者单点故障阻塞所有参与方

**Saga 模式**是更实用的选择：

```
┌──────────────────────────────────────────────────┐
│              Saga 执行语义                         │
│                                                    │
│  forward（顺序执行）    失败时 compensate（反向）   │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐           │
│  │  debit  │→ │ credit  │→ │ notify  │           │
│  └────┬────┘  └────┬────┘  └─────────┘           │
│       │            │ 失败                          │
│       │            ▼                              │
│       │   ┌───────────────┐                       │
│       └──→│ refund_debit  │ ← 反向补偿第一步       │
│           └───────────────┘                       │
│                                                    │
│  ✅ forward 全部成功 → saga 提交                   │
│  ❌ 某 forward 失败   → 已完成步骤反向补偿          │
│  ⚠️  补偿失败         → best-effort，告警 + 人工    │
│  ⏭️  第一步就失败     → 无补偿（无已完成步骤）      │
└──────────────────────────────────────────────────┘
```

## 工作流结构

```
                        ┌─────────────────┐
                        │   fraud_check   │  LLM 欺诈检测
                        │  (openai/GPT-4o) │  + capture_error 降级
                        └────────┬────────┘
                                 │
                                 ▼
                        ┌─────────────────┐
                        │ save_fraud_report│  落盘风控报告
                        └────────┬────────┘
                                 │
                                 ▼
              ┌──────────────────────────────────┐
              │       cross_bank_transfer         │
              │           (Saga)                  │
              │                                  │
              │  forward 1: debit (扣款)          │
              │    compensate: refund_debit (退款)│
              │                                  │
              │  forward 2: credit (入账)         │
              │    compensate: reverse_credit     │
              │                (冲正)             │
              │                                  │
              │  forward 3: notify_bank (通知)    │
              │    (无 compensate，纯通知)        │
              └──────────────┬───────────────────┘
                             │
                             ▼
              ┌──────────────────────────┐
              │  generate_audit_trail     │  生成审计摘要
              │  (template_render)        │
              └──────────────┬───────────┘
                             │
                             ▼
              ┌──────────────────────────┐
              │  save_audit_trail         │  落盘审计轨迹
              │  (file_write append)      │
              └──────────────┬───────────┘
                             │
                             ▼
              ┌──────────────────────────┐
              │  send_notification        │  发送通知
              │  (notify stdout)          │
              └──────────────────────────┘
```

## 前置条件

```bash
# 1. 设置 OpenAI API Key（欺诈检测 LLM）
export OPENAI_API_KEY="sk-..."

# 2. 设置审计 HMAC 密钥（防篡改审计链）
export AFLARE_AUDIT_HMAC_KEY="your-secret-hmac-key"

# 3. （可选）开启 LLM 决策缓存，保证决策可复现
export AFLARE_LLM_CACHE=1

# 4. （可选）开启 trace 持久化，记录每步 LLM I/O
export AFLARE_TRACE=1
```

## 安装与运行

### 1. 启动 mock 银行 API

```bash
cd examples/killer-demos/02-financial-workflow
python3 mock-server.py
```

输出：
```
============================================================
  🏦  Mock Bank API — Financial Workflow Killer Demo
============================================================
  Listening on http://0.0.0.0:17901
  Credit fail amount: 9999.0
  Flaky probability:  0.15

  Endpoints:
    POST /debit    — 扣款
    POST /credit   — 入账（amount=9999 触发失败）
    POST /refund   — 退款（debit 补偿）
    POST /reverse  — 冲正（credit 补偿）
    POST /notify   — 通知
    GET  /health   — 健康检查
    GET  /ops      — 操作日志
    GET  /balances — 账户余额
============================================================
```

Mock 银行 API 行为：
- `POST /debit` — 扣款（始终成功）
- `POST /credit` — 入账（**amount=9999 强制失败**，触发 saga 补偿）
- `POST /refund` — 退款（debit 补偿）
- `POST /reverse` — 冲正（credit 补偿）
- `POST /notify` — 通知
- 约 **15% 概率返回 503**（演示 HTTP 重试 + 指数退避）
- 所有写端点支持 `Idempotency-Key` 服务端去重

### 2. 运行工作流

```bash
# 从仓库根目录运行
# AFLARE_ALLOW_LOOPBACK=1 允许访问 localhost mock 服务（SSRF 防护默认拦截内网地址）
AFLARE_ALLOW_LOOPBACK=1 aflare run examples/killer-demos/02-financial-workflow/workflow.yaml
```

### 3. 观察不同路径

修改 `workflow.yaml` 中的 `amount` 变量：

| amount 值 | 行为 | 演示能力 |
|-----------|------|----------|
| `< 9999` | 全部成功，saga 提交 | 正常路径 + 审计 + 通知 |
| `= 9999` | credit 失败，debit 被补偿（退款） | **Saga 事务补偿** |
| 任意值 | 15% 概率遇到 503 | **HTTP 重试 + 指数退避** |

## 预期输出

### 成功路径（amount < 9999）

```
[mock-bank] debit account=CORP-ACC-001 amount=100.0 status=success
[mock-bank] credit account=CORP-ACC-002 amount=100.0 status=success
[mock-bank] notify ref=TXN-2026-0807-001 status=success

============================================
🏦 企业金融转账工作流执行完毕
============================================
转账编号: TXN-2026-0807-001
付款账户: CORP-ACC-001
收款账户: CORP-ACC-002
金额:     100.00 CNY
风控结果: {"risk_score":5,"risk_level":"low","decision":"allow",...}
转账结果: {"status":"success","endpoint":"notify",...}
审计轨迹: ./audit-trail.jsonl
============================================
```

### 补偿路径（amount = 9999）

```bash
curl http://localhost:17901/ops | python3 -m json.tool
```

```json
{
  "ops": [
    {"endpoint": "debit",   "status": "success"},  // forward 1 ✓
    {"endpoint": "credit",  "status": "failed"},   // forward 2 ✗
    {"endpoint": "refund",  "status": "success"}   // compensate 1 ✓（反向）
  ]
}
```

Saga 执行顺序：`debit 成功 → credit 失败 → 反向补偿 refund_debit`，最终账户余额不变。

## 合规特性详解

### 1. HMAC 审计链（防篡改）

aflare executor 内置 HMAC 哈希链审计，所有步骤的元数据自动写入 `audit.log.jsonl`：

```jsonl
{"type":"workflow_start","workflow":"financial-workflow","hmac":"sha256:abc123..."}
{"type":"step_start","step":"fraud_check","node":"openai","hmac":"sha256:def456..."}
{"type":"step_end","step":"fraud_check","duration_ms":1234,"hmac":"sha256:ghi789..."}
{"type":"step_start","step":"cross_bank_transfer","saga":"forward","hmac":"sha256:..."}
{"type":"saga_compensate","step":"refund_debit","reason":"credit failed","hmac":"sha256:..."}
{"type":"workflow_end","cost_usd":0.0123,"total_tokens":1500,"hmac":"sha256:..."}
```

每条记录的 HMAC 依赖前一条，形成链式防篡改——任何一条记录被修改，后续所有 HMAC 都会失效。

### 2. 幂等性（防重复执行）

双重保障：

| 层级 | 机制 | 说明 |
|------|------|------|
| **工作流级** | SDK `WithIdempotencyKey` | 同一 key 第二次执行直接返回首次结果，不重跑任何步骤 |
| **服务端级** | `Idempotency-Key` 透传 | 银行 API 基于 key 做数据库唯一索引去重 |

```go
exec := workflow.NewExecutor().
    WithAuditLog(true, "").
    WithIdempotencyKey("TXN-2026-0807-001") // 工作流级幂等
```

### 3. 数据脱敏（PII Redaction）

- `file_read` 节点默认开启 `redact=true`，自动脱敏 API Key、Token、.env 文件
- 审计日志中敏感字段自动打码
- `AFLARE_AUDIT_HMAC_KEY` 绝不落盘

### 4. capture_error（错误即分支）

LLM 欺诈检测不可用时，不崩溃整个转账流程，而是降级为"人工审核"：

```yaml
- name: fraud_check
  node: openai
  capture_error:
    - name: fraud_fallback
      node: template_render
      params:
        template: |
          {"risk_score":50,"risk_level":"medium",
           "decision":"review","reasoning":"LLM 不可用，降级为人工审核"}
```

这确保了**风控增强流程**（有 LLM）而非**风控强依赖流程**（LLM 挂了就全挂）。

## 审计日志查看

```bash
# 查看应用层审计摘要
cat ./audit-trail.jsonl | python3 -m json.tool

# 查看 executor 内置 HMAC 审计链
cat ./audit.log.jsonl | python3 -m json.tool

# 查看风控报告
cat ./fraud-report.json | python3 -m json.tool

# 验证 mock 银行操作日志
curl http://localhost:17901/ops | python3 -m json.tool
```

## 其他金融示例

| 示例 | 场景类型 | 核心能力 |
|------|----------|----------|
| [aml-review](../../finance/aml-review/) | 只读分析 | map 并发 + LLM 风险评分 |
| [idempotent-transfer](../../finance/idempotent-transfer/) | 单步写入 | 幂等性 + 限流 + 重试 |
| [saga-transfer](../../finance/saga-transfer/) | 跨步骤写入 | saga 事务补偿 |
| [reconciliation](../../finance/reconciliation/) | 只读分析 | map 匹配 + 差异标记 |
| **financial-workflow**（本 Demo） | **全链路** | **LLM 风控 + Saga + 审计 + 幂等 + 通知** |

## 生产部署注意事项

| 风险 | 说明 | 生产建议 |
|------|------|----------|
| **补偿必须幂等** | compensate 可能被多次执行 | 通过 `Idempotency-Key` 保证幂等，银行 API 做唯一索引 |
| **补偿失败需人工介入** | best-effort 不无限重试 | 告警 + 待办流程 + 人工对账 |
| **审计密钥保管** | `AFLARE_AUDIT_HMAC_KEY` 泄露可伪造审计链 | 通过 KMS/Secret Manager 注入，不落盘 |
| **金额精度** | 浮点精度问题 | 生产环境用 decimal/整数分 |
| **LLM 成本** | 每次 fraud_check 消耗 token | 开启 `AFLARE_LLM_CACHE=1` 缓存相同输入，或使用更便宜的模型 |
| **SSRF 防护** | 默认拦截内网地址 | 生产环境用真实外部 API，不需要 `AFLARE_ALLOW_LOOPBACK` |