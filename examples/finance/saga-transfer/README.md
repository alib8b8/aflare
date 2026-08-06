# Saga 跨行转账工作流示例

> 真实金融跨步骤事务场景的可运行 Demo：展示 llm-box 的 **saga 事务补偿**能力——多步骤写入在任一步骤失败时，已完成的步骤按反向顺序自动补偿（回滚）。

## 展示的金融能力

| 能力 | 实现机制 | 在本示例中的体现 |
|------|----------|------------------|
| **saga 事务补偿** | forward 步骤 + 反向 compensate | `debit → credit → notify`，credit 失败时自动执行 `refund-debit`（反向补偿 debit） |
| **best-effort 补偿** | 补偿失败不阻断其他补偿 | 某步补偿失败仅告警，其他步骤仍继续补偿，需人工对账 |
| **{{var.error}} 上下文** | 触发回滚的失败原因传入补偿步骤 | compensate 步骤可通过 `{{var.error}}` 分支处理不同失败原因 |
| **HTTP 限流/重试** | per-host 令牌桶 + 指数退避 | 每个 forward/compensate 步骤都配置 `rate_limit_rps` 和 `max_retries` |
| **审计日志** | executor HMAC 哈希链自动落盘 | 所有 forward/compensate 步骤的输入/输出/状态自动记录，防篡改 |

## Saga 执行语义

```
forward（顺序执行）        失败时 compensate（反向执行）
┌─────────┐  ┌─────────┐  ┌─────────┐
│  debit  │→ │ credit  │→ │ notify  │
└─────────┘  └─────────┘  └─────────┘
                  │ 失败
                  ▼
         ┌───────────────┐
         │ refund-debit  │  ← 补偿 debit（反向第一步）
         └───────────────┘
```

- **forward 全部成功**：saga 提交，输出最后一步结果
- **某 forward 失败**：已完成的 forward 步骤按**反向顺序**执行 compensate
- **compensate 失败**：best-effort，记录告警并继续补偿其他步骤（需人工对账）
- **无 compensate 的步骤**：跳过（视为无副作用需回滚，如纯通知）
- **第一步就失败**：无已完成步骤，不执行任何补偿

## 前置条件

```bash
# 设置审计 HMAC 密钥（防篡改审计链，记录所有 forward/compensate）
export LLM_BOX_AUDIT_HMAC_KEY="your-secret-hmac-key"
```

## 运行步骤

### 1. 启动 mock 银行 API

```bash
cd examples/finance/saga-transfer
python3 mock-server.py
# 输出: [mock-bank] listening on http://0.0.0.0:17801
```

mock 银行 API 端点：
- `POST /debit` — A 行扣款（始终成功）
- `POST /credit` — B 行入账（对 `amount=9999` 强制失败，演示补偿）
- `POST /refund` — A 行退款（debit 的补偿）
- `POST /reverse` — B 行冲正（credit 的补偿）
- `POST /notify` — 转账通知
- `GET /ops` — 查看操作日志（验证 saga 执行顺序）

### 2. 运行工作流（演示补偿路径）

默认 `amount=9999`，触发 credit 失败 → debit 被补偿：

```bash
# 从仓库根目录运行
# LLMBOX_ALLOW_LOOPBACK=1 允许访问 localhost mock 服务（SSRF 防护默认拦截内网地址）
LLMBOX_ALLOW_LOOPBACK=1 llm-box run examples/finance/saga-transfer/workflow.yaml
```

预期行为：
1. `debit` forward 成功（A 行扣款）
2. `credit` forward 失败（amount=9999 触发 500）
3. saga 启动补偿：`refund-debit` 执行（A 行退款）
4. 工作流返回 credit 的失败错误

### 3. 验证 saga 执行顺序

```bash
# 查看 mock 服务的操作日志
curl http://localhost:17801/ops | python3 -m json.tool
```

补偿路径下的操作顺序：
```json
{"ops": [
  {"endpoint": "debit",  "status": "success"},  // forward 1
  {"endpoint": "credit", "status": "failed"},   // forward 2 失败
  {"endpoint": "refund", "status": "success"}   // compensate 1（反向）
]}
```

### 4. 观察不同路径

修改 `workflow.yaml` 中的 `amount` 变量：

| amount 值 | 行为 |
|-----------|------|
| `< 9999` | 全部成功，saga 提交，输出 notify 结果 |
| `= 9999` | credit 失败，debit 被补偿（退款）|
| `> 9999` | 可调整 mock 的 `MOCK_CREDIT_FAIL_AMOUNT` 演示其他路径 |

## 生产须知

1. **补偿必须幂等**：compensate 步骤可能被多次执行（重试、重复触发），必须通过 `Idempotency-Key` 保证幂等
2. **补偿失败需人工介入**：best-effort 意味着补偿失败不会无限重试，应有告警和待办流程
3. **审计是合规依据**：所有 forward/compensate 步骤自动写入 HMAC 哈希链审计日志，是金融合规的核心证据
4. **配合服务端幂等**：saga 补偿客户端层面，真实跨行转账仍需银行 API 支持服务端幂等和冲正接口
5. **不替代分布式事务**：saga 是最终一致性模型，强一致场景仍需 2PC（本项目暂未实现）

## 与幂等转账示例的区别

| 示例 | 场景类型 | 核心能力 | 适用场景 |
|------|----------|----------|----------|
| [idempotent-transfer](../idempotent-transfer/) | 单步写入 | 幂等性 + 限流 + 重试 | 防止重复扣款 |
| **saga-transfer**（本示例） | 跨步骤写入 | saga 事务补偿 | 跨行转账、多步骤需回滚 |
| [aml-review](../aml-review/) | 只读分析 | map 并发 + LLM 风险评分 | 反洗钱审查 |
| [reconciliation](../reconciliation/) | 只读分析 | map 匹配 + 差异标记 | 日终对账 |
