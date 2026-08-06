# 日终对账工作流示例

> 真实金融只读分析场景的可运行 Demo：比对银行对账单与内部账本，标记差异并生成报告。展示 llm-box 的 **map 并发匹配**能力在金融对账中的应用。

## 展示的金融能力

| 能力 | 实现机制 | 在本示例中的体现 |
|------|----------|------------------|
| **map 并发匹配** | 子工作流 per-item 并发执行 | 对每笔银行交易并发匹配内部账本（`concurrency: 5`） |
| **差异标记** | condition + if/else 分支 | 匹配成功标记 `reconciled`，缺失标记 `missing_in_ledger` |
| **容错降级** | `stop_on_error: false` | 单笔匹配失败不终止整批对账 |
| **审计日志** | executor HMAC 哈希链 | 对账执行过程自动记录，合规可追溯 |

## 对账语义

```
银行对账单 (bank.json)        内部账本 (workflow var: ledger)
┌──────────────┐              ┌──────────────┐
│ TXN-001 100  │ ──匹配──→    │ TXN-001 100  │  → reconciled
│ TXN-002 250  │ ──匹配──→    │ TXN-002 250  │  → reconciled
│ TXN-003 1000 │ ──匹配──→    │ TXN-003 1000 │  → reconciled
│ TXN-004 75   │ ──匹配──→    │ TXN-004 75   │  → reconciled
│ TXN-005 500  │ ──缺失──→    │      -       │  → missing_in_ledger
└──────────────┘              └──────────────┘
```

本示例的样例数据中，`TXN-005` 在内部账本缺失，会被标记为 `missing_in_ledger`。

## 设计说明

- **银行对账单**从 `bank.json` 文件读取（演示 `file_read` + `map` 组合）。
- **内部账本**以内联 `var` 提供（`workflow.yaml` 的 `vars.ledger`）。
  生产环境中内部账本通常来自账务系统 API 或配置中心。
- **为何账本用 var 而非 step**：`map` 子工作流只能访问 `{{var.*}}`，无法引用
  外层 `{{step.*}}`（子工作流有独立的步骤命名空间）。将账本作为 `var` 注入，
  子工作流内即可用 `contains` 做匹配。匹配流程：
  1. `tx_id`：从当前银行交易提取 tx_id
  2. `ledger_text`：渲染 `{{var.ledger}}` 为纯文本（作为 condition 的输入）
  3. `check-ledger`：`contains:{{step.tx_id}}` 判断账本是否含此 tx_id
  4. `if/else`：匹配标记 `reconciled`，缺失标记 `missing_in_ledger`

## 运行步骤

### 1. 查看样例数据

```bash
cd examples/finance/reconciliation
cat bank.json    # 银行对账单（5 笔交易）
# 内部账本见 workflow.yaml 的 vars.ledger（4 笔交易，TXN-005 缺失）
```

### 2. 运行对账工作流

```bash
# 从仓库根目录运行（纯本地文件操作，无需启动 mock 服务）
llm-box run examples/finance/reconciliation/workflow.yaml
```

### 3. 查看对账报告

```bash
cat examples/finance/reconciliation/reconciliation-report.md
```

报告包含每笔交易的匹配状态数组，例如：
```
["reconciled","reconciled","reconciled","reconciled","missing_in_ledger"]
```

## 生产须知

1. **大数据量分批**：map 的 `max_iterations` 默认上限 100，真实对账数据量大时应分批处理
2. **金额校验**：本示例简化为 tx_id 匹配，生产环境需额外校验金额、币种、日期一致性
3. **差异告警**：`missing_in_ledger` 和 `amount_mismatch` 应触发告警和人工复核流程
4. **报告归档**：对账报告应归档存储，审计日志自动记录执行过程作为合规证据

## 其他金融示例

| 示例 | 场景类型 | 核心能力 |
|------|----------|----------|
| [idempotent-transfer](../idempotent-transfer/) | 单步写入 | 幂等性 + 限流 + 重试 |
| [saga-transfer](../saga-transfer/) | 跨步骤写入 | saga 事务补偿 |
| [aml-review](../aml-review/) | 只读分析 | map 并发 + LLM 风险评分 |
| **reconciliation**（本示例） | 只读分析 | map 匹配 + 差异标记 |
