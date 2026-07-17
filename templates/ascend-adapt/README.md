# 昇腾 NPU 模型适配 - 工作流模板

> 基于 llm-box 工作流引擎，为昇腾 NPU 模型适配场景打造的一套端到端模型适配流水线模板。
> 通过七大专用 Agent 串联，覆盖从模型搜索到部署文档的全生命周期，帮助开发者快速产出可复现、可评测的适配成果。

## 适用场景

本套模板专为以下场景设计：

- **昇腾 NPU 模型迁移**：将开源大模型（HuggingFace / AtomGit 源）迁移至昇腾 910B / 910C / 800I A2 / 310P 等 NPU 平台。
- **生产级模型交付**：在昇腾硬件上完成「搜索 → 验证 → 适配 → 量化 → 调优 → 部署 → 文档」一体化交付。
- **快速可行性验证**：1 小时内完成模型的搜索、验证、适配与文档输出，快速判断迁移可行性。
- **性能极致调优**：针对 800I A2 / 310P 等硬件做量化、算子融合、Graph 编译，达成目标时延与吞吐。

支持的昇腾硬件：`910B`、`910C`、`800I-A2`、`310P`。
支持的开发框架：`pytorch`（PyTorch + CANN）、`mindspore`（MindSpore 原生）。

## 模板列表

| 模板 | 路径 | 适用场景 | 步骤数 | 预计耗时 |
|------|------|----------|--------|----------|
| 端到端适配 | `end-to-end-adapt/` | 全流程交付，七大 Agent 串联 | 7 阶段 | 4-8 小时 |
| 快速适配 | `quick-adapt/` | 可行性验证、Demo 跑通、最小适配报告 | 4 步 | < 1 小时 |
| 性能调优 | `performance-tune/` | 极致性能，量化 + 调优 + 部署 | 5 步 | 2-4 小时 |

> 端到端适配模板 = 快速适配（前 3 步）+ 性能调优（后 4 步）的完整组合，是主力模板。

## 七大 Agent 说明

本套模板由七个专用 Agent 协同完成，每个 Agent 对应一个 `ascend_*` 执行节点，由统一的 `agent` 编排节点驱动决策：

| # | Agent 名称 | 执行节点 | 职责 | 关键参数 |
|---|-----------|----------|------|----------|
| 1 | 模型搜索 Agent | `ascend_model_search` | 在 AtomGit / HuggingFace 等模型源检索候选模型，输出模型卡与下载地址 | `query`, `source`, `limit` |
| 2 | 模型验证 Agent | `ascend_model_verify` | 校验模型文件完整性、License 合规性、昇腾兼容性预检 | `model_name`, `source` |
| 3 | 模型适配 Agent | `ascend_model_adapt` | 将模型迁移到昇腾 NPU，处理算子替换、图改写、框架转换 | `model_name`, `framework`, `strategy`, `target_npu` |
| 4 | 模型量化 Agent | `ascend_model_quantize` | 对适配后模型执行 INT8 / FP8 / W8A8 量化，降低显存与延迟 | `model_name`, `method`, `calibration_dataset` |
| 5 | 性能调优 Agent | `ascend_model_optimize` | 针对 NPU 硬件特性进行算子融合、Graph 编译、吞吐/延迟优化 | `model_name`, `hardware`, `target_latency`, `target_throughput` |
| 6 | 模型部署 Agent | `ascend_model_deploy` | 基于 MindIE 等后端启动推理服务，输出可访问的 Endpoint | `model_name`, `hardware`, `port`, `backend` |
| 7 | 文档生成 Agent | `ascend_model_doc` | 自动生成适配报告、Benchmark 数据、复现指南 | `model_name`, `format`, `include_benchmark` |

此外还提供端到端编排 Agent `ascend_model_agent`，可在 `mode=full/quick/tune` 间切换，用于一键启动完整流程。

## 使用示例

### 1. 安装并查看模板

```bash
# 列出所有可用模板
llm-box templates list | grep ascend

# 查看某模板详情
llm-box templates show ascend-adapt/end-to-end-adapt
```

### 2. 端到端适配（全流程主力）

```bash
# 使用默认模型 Qwen2.5-7B-Instruct 跑全流程
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml

# 指定模型名称与目标硬件
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml \
  --input "Qwen2.5-14B-Instruct" \
  --set target_npu=910C \
  --set framework=pytorch
```

### 3. 快速适配（1 小时内可行性验证）

```bash
llm-box run ascend-adapt/quick-adapt/workflow.yaml \
  --input "Qwen2.5-7B-Instruct"
```

### 4. 性能调优（性能场景）

```bash
llm-box run ascend-adapt/performance-tune/workflow.yaml \
  --input "Qwen2.5-7B-Instruct" \
  --set hardware=800I-A2 \
  --set target_latency=50
```

### 5. 使用编排 Agent 一键启动

```bash
# full 模式 = 端到端，quick 模式 = 快速适配，tune 模式 = 性能调优
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml \
  --input "ChatGLM3-6B" \
  --set mode=full
```

## 工作流架构

```
                         ┌─────────────────────────────────────┐
                         │           用户输入：模型名称          │
                         │   (如 Qwen2.5-7B-Instruct + 硬件)    │
                         └──────────────────┬──────────────────┘
                                            │
            ┌───────────────────────────────┼───────────────────────────────┐
            │                               │                               │
            ▼                               ▼                               ▼
   ┌─────────────────┐          ┌─────────────────────┐          ┌─────────────────┐
   │  快速适配 (4步)  │          │   端到端适配 (7阶段)  │          │  性能调优 (5步)  │
   │  quick-adapt    │          │  end-to-end-adapt   │          │ performance-tune│
   └─────────────────┘          └─────────────────────┘          └─────────────────┘
   search → verify              search → verify                  adapt → quantize
   → adapt → doc                → adapt → quantize               → optimize → deploy
                                → optimize → deploy             → doc
                                → doc

                         ┌─────────────────────────────────────┐
                         │       输出：适配报告 + Benchmark       │
                         │   （Markdown / HTML，含复现指南）      │
                         └─────────────────────────────────────┘
```

## 环境前置要求

- **llm-box** CLI 已安装（`llm-box --version`）
- **CANN Toolkit** ≥ 7.0（昇腾 NPU 驱动与运行时）
- **PyTorch / MindSpore** 对应的昇腾适配版本
- **MindIE**（用于部署阶段，可选）
- 可访问的 NPU 设备：`npu-smi info` 可正常输出

> 若 `ascend_*` 节点尚未在本地实现，可使用 `agent` 节点配合 `execute` 节点作为占位实现，待节点注册后无缝切换。

## 自定义与扩展

1. **修改默认硬件**：编辑各 `workflow.yaml` 顶部 `inputs` 段的 `target_npu` / `hardware` 默认值。
2. **调整量化策略**：在 `ascend_model_quantize` 步骤修改 `method`（`int8` / `fp8` / `w8a8`）。
3. **更换后端**：在 `ascend_model_deploy` 步骤修改 `backend`（默认 `mindie`，可换 `vllm-ascend` 等）。
4. **新增 Agent**：参考 `nodes-reference.md` 实现新的 `ascend_*` 节点并注册到 `nodes-registry.json`。

## 相关文档

- llm-box 工作流语法：[`../../.trae/skills/llm-box-workflow/SKILL.md`](../../.trae/skills/llm-box-workflow/SKILL.md)
- 节点参考：[`../../.trae/skills/llm-box-workflow/nodes-reference.md`](../../.trae/skills/llm-box-workflow/nodes-reference.md)
- 自定义节点开发：[`../../docs/custom-nodes.md`](../../docs/custom-nodes.md)
- 昇腾适配方案：[`../../ecosystem/ascend-adaptation/ASCEND_ADAPTATION.md`](../../ecosystem/ascend-adaptation/ASCEND_ADAPTATION.md)
- 示例模板：[`../../examples/`](../../examples/)

## 分类

`ascend-adapt` / AI-ML / 昇腾生态
