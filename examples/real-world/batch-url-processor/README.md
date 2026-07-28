# batch-url-processor

批量 URL 处理器。这是"能解决问题的工作流"的最小范本，用来证明本项目的工作流引擎不只是"节点串联"，而是能真正表达**批量处理 + 错误降级 + 并发控制**的完整问题求解逻辑。

## 它解决了什么问题

给定一组 URL，需要：并发抓取每个 URL → 对成功的用 LLM 摘要 → 对失败的标记为降级 → 聚合成报告。

这种"对 N 个条目各跑一段多步流程"的模式，是真实运维/数据处理场景的核心需求。

## 它展示了什么

| 能力 | 在模板里的体现 |
|------|---------------|
| **map 原语** | `map.over` 对 URL 数组逐项跑**多步子工作流**（fetch→classify→summarize），而非 loop 的单节点迭代 |
| **错误即数据** | `continue_on_error: true` + `if` 分支让抓取失败成为可决策的降级路径，而不是终止整批 |
| **并发控制** | `concurrency: 3` 限制并发抓取数，配合 `retry` + 指数退避 |
| **结构化输出** | map 默认 `json_array` 输出，失败项占位保留顺序 |

## 为什么这比"把逻辑塞进一个节点"好

对比把整段逻辑塞进 `transform` 节点的 `operation` 参数：每个子步骤（fetch/summarize/degrade）都是独立可观测、可重试、可替换的。抓取慢不会阻塞已抓取项的摘要；摘要失败不影响其他 URL。这正是工作流引擎相对"一个超大节点"的价值。

## 运行

```bash
# 用默认 5 个 example.com URL 试跑（fetch 会失败，走降级路径）
llm-box run examples/real-world/batch-url-processor/workflow.yaml

# 处理自己的 URL 列表
llm-box run examples/real-world/batch-url-processor/workflow.yaml \
  -var urls='["https://news.ycombinator.com","https://go.dev"]'
```
