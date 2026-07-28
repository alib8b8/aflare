# 性能调优工作流（Performance Tune）

> 昇腾 Model-Agent 模型适配大赛 · 赛道二主力模板
> 五步冲刺性能榜单：**适配 → 量化 → 调优 → 部署 → 文档**

## 工作流简介

`performance-tune` 是参赛「昇腾 Model-Agent 模型适配大赛」赛道二的性能冲刺模板。它假设模型已通过基础可行性验证，聚焦在昇腾 NPU 上的极致性能优化，通过量化、算子融合、Graph 编译、KV-Cache 优化等手段，输出可压测的高性能推理服务与含完整 Benchmark 的调优报告。

```
输入：模型名称 + 目标硬件（800I A2 或 310P）
  │
  ├─ 1. 模型适配 (ascend_model_adapt)     定向硬件基础适配，标注性能关键算子
  ├─ 2. 模型量化 (ascend_model_quantize)  fp8 / w8a8 / int8 按硬件自动选择
  ├─ 3. 性能调优 (ascend_model_optimize)  算子融合 + Graph 编译 + KV-Cache（核心）
  ├─ 4. 模型部署 (ascend_model_deploy)    MindIE 高性能推理服务
  └─ 5. 文档生成 (ascend_model_doc)       调优报告 + 完整 Benchmark
  │
输出：性能调优报告（含 TTFT / 吞吐 / 显存 / 加速比）
```

> **赛道二 vs 赛道一**：赛道二跳过搜索与验证（假设模型已确定），把全部预算投入性能优化，是榜单冲刺专用模板。

## 使用方法

### 基本用法（800I A2 默认硬件）

```bash
llm-box run ascend-adapt/performance-tune/workflow.yaml \
  --input "Qwen2.5-7B-Instruct" \
  --set hardware=800I-A2
```

### 边缘侧 310P 场景

```bash
llm-box run ascend-adapt/performance-tune/workflow.yaml \
  --input "Qwen2.5-7B-Instruct" \
  --set hardware=310P \
  --set quantize_method=int8 \
  --set target_latency=200
```

### 设定性能目标

```bash
llm-box run ascend-adapt/performance-tune/workflow.yaml \
  --input "Qwen2.5-7B-Instruct" \
  --set hardware=800I-A2 \
  --set target_latency=30 \
  --set target_throughput=200
```

### 安装为本地模板

```bash
llm-box install ascend-adapt/performance-tune
llm-box run performance-tune/workflow.yaml --input "Qwen2.5-7B-Instruct" --set hardware=800I-A2
```

## 输入说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `model` / `--input` | string | `Qwen2.5-7B-Instruct` | **必填**，待调优的模型名称 |
| `hardware` | string | `800I-A2` | **必填**，目标硬件，大赛赛道二主要支持 `800I-A2` 与 `310P`，也兼容 `910B` / `910C` |
| `framework` | string | `pytorch` | 开发框架，`pytorch` / `mindspore` |
| `quantize_method` | string | `fp8` | 量化方法，`int8` / `fp8` / `w8a8`，按硬件自动推荐 |
| `target_latency` | int | `50` | 目标首 token 延迟（ms），调优 Agent 的优化目标 |
| `target_throughput` | int | `100` | 目标吞吐量（tokens/s） |
| `deploy_port` | int | `8080` | 部署推理服务的监听端口 |

### 硬件与量化方法推荐搭配

| 硬件 | 推荐量化 | 推荐目标延迟 | 推荐目标吞吐 | 说明 |
|------|----------|--------------|--------------|------|
| `800I-A2` | `fp8` | 30-50 ms | 100-200 tokens/s | 原生 FP8，性能榜单主力 |
| `310P` | `int8` | 100-300 ms | 20-50 tokens/s | 边缘侧，极致压缩 |
| `910C` | `fp8` | 20-40 ms | 150-300 tokens/s | 最新训练卡，吞吐最高 |
| `910B` | `w8a8` | 50-80 ms | 80-150 tokens/s | 性价比之选 |

## 输出说明

工作流最终输出（`output: "{{step.doc}}"`）为一份性能调优报告，必须包含完整 Benchmark（`include_benchmark=true`）：

1. **模型与硬件**：模型卡、目标硬件规格
2. **适配基线**：FP16 未优化状态的性能基线
3. **量化方案**：量化方法、校准集、精度损失
4. **调优技术清单**：
   - FlashAttention / FlashComm 算子融合
   - Ascend Graph 整图编译（GE）
   - batch sizing 与多流流水线
   - KV-Cache 内存复用 / PagedAttention
   - 算子精度回退清单
5. **完整 Benchmark**：
   - 首 token 延迟（TTFT）
   - 端到端延迟（E2E Latency）
   - 吞吐量（tokens/s）
   - 显存峰值（GB）
   - **与基线的加速比**
6. **部署指南**：服务 Endpoint、压测命令、健康检查
7. **回退策略**：高风险算子回退路径

## 五步步骤详解

| 步骤 | step id（plan / exec） | 节点 | 关键产物 |
|------|------------------------|------|----------|
| 1 适配 | `adapt_plan` / `adapt` | `agent` + `ascend_model_adapt` | 基础适配 + 性能关键算子清单 |
| 2 量化 | `quantize_plan` / `quantize` | `agent` + `ascend_model_quantize` | 量化模型 + 延迟收益预估 |
| 3 调优 | `optimize_plan` / `optimize` | `agent` + `ascend_model_optimize` | 调优配置 + 性能数据（**核心**） |
| 4 部署 | `deploy_plan` / `deploy` | `agent` + `ascend_model_deploy` | 高性能推理服务 + 压测入口 |
| 5 文档 | `doc_plan` / `doc` | `agent` + `ascend_model_doc` | 调优报告 + 完整 Benchmark |

> 步骤 3（`optimize`）是赛道二的核心，决定了榜单排名。`ascend_model_optimize` 节点会综合应用算子融合、Graph 编译、KV-Cache 优化等技术，向 `target_latency` 与 `target_throughput` 收敛。

## 自定义指南

### 1. 切换硬件与量化方法

```bash
# 800I A2 + FP8（榜单冲刺）
--set hardware=800I-A2 --set quantize_method=fp8

# 310P + INT8（边缘部署）
--set hardware=310P --set quantize_method=int8 --set target_latency=200
```

### 2. 调整性能目标

`target_latency` 与 `target_throughput` 会作为 `ascend_model_optimize` 的优化目标，调优 Agent 会据此选择技术组合。目标越激进，越可能触发算子精度回退。

```bash
--set target_latency=30 --set target_throughput=200   # 激进
--set target_latency=80 --set target_throughput=50    # 保守，精度优先
```

### 3. 更换推理后端

`deploy` 步骤默认使用 `mindie`，可改为 `vllm-ascend`（支持 PagedAttention 与连续批处理）：

```yaml
- id: deploy
  node: ascend_model_deploy
  params:
    backend: "vllm-ascend"   # 修改此处
```

### 4. 补充搜索与验证阶段

如需从零开始（未做基础适配），可在 `adapt_plan` 之前插入 `search` 与 `verify` 两个阶段，参考 [`../end-to-end-adapt/workflow.yaml`](../end-to-end-adapt/workflow.yaml) 的前两个阶段。

### 5. 使用编排 Agent 一键启动

通过 `ascend_model_agent` 编排节点以 `mode=tune` 启动，等效于本工作流：

```yaml
- id: tune_pipeline
  node: ascend_model_agent
  params:
    model_name: "{{input}}"
    mode: "tune"
    hardware: "{{hardware}}"
```

## 环境前置要求

- llm-box CLI（`llm-box --version`）
- CANN Toolkit ≥ 7.0 + NPU 驱动（`npu-smi info` 可用）
- PyTorch / MindSpore 昇腾适配版
- MindIE ≥ 1.0（部署阶段，支持张量并行与连续批处理）
- 推荐多卡环境（800I A2 × 2+）以发挥张量并行优势
- `ascend_*` 节点已注册

## 压测建议

部署完成后，可使用以下命令快速验证 Benchmark：

```bash
# 健康检查
curl http://localhost:8080/health

# 单请求延迟测试
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen2.5-7B-Instruct","prompt":"Hello","max_tokens":32}'

# 吞吐压测（推荐使用 vllm 的 benchmark_serving.py 或 mindie 自带压测工具）
python benchmark_serving.py \
  --backend mindie \
  --base-url http://localhost:8080 \
  --model Qwen2.5-7B-Instruct \
  --num-prompts 100
```

## 相关模板

- 端到端适配：[`../end-to-end-adapt/`](../end-to-end-adapt/)（赛道一，含搜索 + 验证的完整流程）
- 快速适配：[`../quick-adapt/`](../quick-adapt/)（1 小时可行性验证）
- 上级目录：[`../README.md`](../README.md)

## 分类

`ascend-adapt` / 性能调优 / 赛道二
