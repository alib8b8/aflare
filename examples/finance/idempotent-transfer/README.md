# 幂等转账工作流示例

> 真实金融写入场景的可运行 Demo：展示 llm-box 在**受控写入**类金融业务中的四大核心能力——幂等性、HTTP 限流、HTTP 重试、审计日志。

## 展示的金融能力

| 能力 | 实现机制 | 在本示例中的体现 |
|------|----------|------------------|
| **幂等性**（防重复扣款） | `Idempotency-Key` + 原子占位 + 跨进程锁 | 工作流通过 SDK `WithIdempotencyKey` 调用，同一 key 重复执行只产生一次真实副作用 |
| **HTTP 限流** | per-host 令牌桶 | `rate_limit_rps=5` 每秒最多 5 请求，保护下游银行 API |
| **HTTP 重试** | 指数退避 + 可配重试状态码 | `max_retries=3`，对 `429,500,502,503,504` 自动重试 |
| **审计日志** | executor HMAC 哈希链自动落盘 | 每个步骤的输入/输出/耗时/重试自动记录，防篡改 |

## 前置条件

```bash
# 1. 设置审计 HMAC 密钥（防篡改审计链）
export LLM_BOX_AUDIT_HMAC_KEY="your-secret-hmac-key"

# 2. （可选）开启 LLM 决策缓存，保证决策可复现
export LLM_BOX_LLM_CACHE=1

# 3. （可选）开启 trace 持久化，记录每步 LLM I/O（已自动脱敏）
export LLM_BOX_TRACE=1
```

## 运行步骤

### 1. 启动 mock 银行 API

```bash
cd examples/finance/idempotent-transfer
python3 mock-server.py
# 输出: [mock-bank] listening on http://0.0.0.0:17800
```

mock 银行 API 行为：
- 支持 `Idempotency-Key` 服务端去重（同一 key 返回首次结果）
- `amount > 10000` 时返回失败（演示失败审计路径）
- 约 10% 概率返回 503（演示 HTTP 重试）

### 2. 用 llm-box CLI 运行工作流

```bash
# 从仓库根目录运行（先启动 mock 银行 API，见上一步）
llm-box run examples/finance/idempotent-transfer/workflow.yaml
```

> 参数（`bank_api_url`、`from_account`、`amount` 等）在 `workflow.yaml` 的 `vars`
> 段配置。CLI 暂不支持 `--var` 覆盖，需要改参数时直接编辑 `vars` 即可。

运行后查看审计日志：
```bash
cat ./transfer-log.jsonl    # 成功记录
cat ./transfer-failed.jsonl # 失败记录
```

### 3. 用 SDK 调用（展示幂等性）

CLI 的 `run` 命令本身不传 Idempotency-Key，要演示工作流级幂等（同一 key 第二次
直接返回首次结果、不重跑任何步骤、不重复调用银行 API），需用 SDK 调用：

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
)

func main() {
	wf, err := workflow.ParseWorkflow("examples/finance/idempotent-transfer/workflow.yaml")
	if err != nil {
		log.Fatalf("parse workflow: %v", err)
	}
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	// 同一个 Idempotency-Key 无论调用多少次，工作流只真实执行一次。
	// 银行 API 侧也应基于此 key 做服务端去重——客户端幂等 + 服务端去重
	// 双重保险，才能在金融场景下真正防重复扣款。
	idempotencyKey := "transfer-ACC0001-ACC0002-20260805-001"
	exec := workflow.NewExecutor().
		WithAuditLog(true, "").           // 开启 HMAC 哈希链审计
		WithIdempotencyKey(idempotencyKey) // 开启工作流级幂等

	// 第一次调用：真实执行
	out1, _, err := exec.Execute(context.Background(), wf, reg)
	if err != nil {
		log.Fatalf("first run failed: %v", err)
	}
	fmt.Println("first run output:", out1)

	// 第二次调用（相同 key）：命中幂等缓存，直接返回首次结果，
	// 不会重复调用银行 API，不会重复写步骤审计日志（但会写一条
	// workflow_idempotent_hit 审计记录）。Execute 返回 ErrIdempotencyHit，
	// 第一个返回值仍是首次的 FinalOutput。
	out2, _, err := exec.Execute(context.Background(), wf, reg)
	if err != nil && !errors.Is(err, workflow.ErrIdempotencyHit) {
		log.Fatalf("second run failed: %v", err)
	}
	fmt.Println("second run output (idempotency hit):", out2)
}
```

## 工作流结构

```
call-bank-api (http_request + 限流 + 重试)
        │
        ▼
check-status (condition: contains:"status":"success")
        │
        ▼
   ┌────┴────┐
   │ if/else │ 分支
   └────┬────┘
   ├── success → build-success-log (template) → log-success (file_write append)
   └── failed  → build-failure-log (template) → log-failure (file_write append)
```

## 安全注意事项

> ⚠️ 本示例仅展示能力，**不能直接用于生产转账**。

| 风险 | 说明 | 生产建议 |
|------|------|----------|
| **服务端必须去重** | 客户端 Idempotency-Key 只是第一道防线，恶意调用方可绕过 | 银行 API 必须基于 Idempotency-Key 做数据库唯一索引去重 |
| **跨步骤事务** | 本工作流是单步转账；多步 saga（扣款→记账→通知）暂未实现 | 需工作流层自行实现补偿/TCC，或用数据库事务 |
| **金额精度** | 本示例用 float 演示，生产环境必须用 decimal/整数分 | 避免浮点精度丢失导致对账差异 |
| **审计密钥保管** | `LLM_BOX_AUDIT_HMAC_KEY` 泄露将使审计链可被伪造 | 通过 KMS/Secret Manager 注入，不落盘 |
| **重试副作用** | HTTP 重试仅对幂等操作安全；非幂等操作需配合 Idempotency-Key | 银行 API 必须支持基于 key 的去重 |

## 与 aml-review 示例的对比

| 维度 | [aml-review](../aml-review/) | [idempotent-transfer](./)（本示例） |
|------|------------------------------|--------------------------------------|
| **场景类型** | 只读分析（AML 可疑交易审查） | 受控写入（转账） |
| **数据流向** | 读取交易 → 分析 → 生成报告 | 调用银行 API → 解析 → 写审计日志 |
| **幂等性需求** | 低（只读，重复执行无副作用） | 高（重复扣款是金融事故） |
| **限流/重试** | 数据源查询（轻量） | 银行 API 调用（必须保护下游） |
| **审计重点** | 决策依据复现（为何上报/放行） | 交易留痕（每笔转账可追溯） |
| **LLM 使用** | LLM 综合风险评分 | 无 LLM（纯规则转账，但审计能力同样适用） |
