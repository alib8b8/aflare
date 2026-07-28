# 端到端模型适配工作流（End-to-End Adapt）

> 昇腾 Model-Agent 模型适配大赛 · 赛道一主力模板
> 七大 Agent 串联：**搜索 → 验证 → 适配 → 量化 → 调优 → 部署 → 文档**

## 工作流简介

`end-to-end-adapt` 是参赛「昇腾 Model-Agent 模型适配大赛」赛道一的端到端交付模板。它把模型迁移到昇腾 NPU 的完整生命周期拆解为七个阶段，每个阶段由一个 `agent` 编排节点（负责策略决策）配合一个 `ascend_*` 执行节点（负责实际操作）共同完成，最终输出符合大赛要求的测评报告。

```
输入：模型名称 + 目标硬件
  │
  ├─ 1. 模型搜索 (ascend_model_search)   检索 AtomGit / HuggingFace
  ├─ 2. 模型验证 (ascend_model_verify)   完整性 / License / 兼容性预检
  ├─ 3. 模型适配 (ascend_model_adapt)    算子替换 / 图改写 / 框架转换
  ├─ 4. 模型量化 (ascend_model_quantize) int8 / fp8 / w8a8
  ├─ 5. 性能调优 (ascend_model_optimize) 算子融合 / Graph 编译
  ├─ 6. 模型部署 (ascend_model_deploy)   MindIE 推理服务
  └─ 7. 文档生成 (ascend_model_doc)      测评报告 + Benchmark
  │
输出：Markdown / HTML 测评报告（含复现指南）
```

## 使用方法

### 基本用法（使用默认参数）

```bash
# 使用默认模型 Qwen2.5-7B-Instruct、目标硬件 800I-A2 跑全流程
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml
```

### 指定模型与硬件

```bash
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml \
  --input "Qwen2.5-14B-Instruct" \
  --set target_npu=910C \
  --set framework=pytorch
```

### 调整量化策略与部署端口

```bash
llm-box run ascend-adapt/end-to-end-adapt/workflow.yaml \
  --input "ChatGLM3-6B" \
  --set quantize_method=fp8 \
  --set deploy_port=9000
```

### 安装为本地模板

```bash
llm-box install ascend-adapt/end-to-end-adapt
llm-box run end-to-end-adapt/workflow.yaml --input "Qwen2.5-7B-Instruct"
```

## 输入说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `model` / `--input` | string | `Qwen2.5-7B-Instruct` | **必填**，待适配的模型名称，需与 HuggingFace / AtomGit 上的模型 ID 一致 |
| `target_npu` | string | `800I-A2` | 目标 NPU 型号，可选 `910B` / `910C` / `800I-A2` / `310P` |
| `framework` | string | `pytorch` | 开发框架，可选 `pytorch`（PyTorch + CANN）/ `mindspore` |
| `quantize_method` | string | `w8a8` | 量化方法，可选 `int8` / `fp8` / `w8a8` |
| `deploy_port` | int | `8080` | 部署推理服务的监听端口 |

> **说明**：`{{input}}` 在工作流中始终指代 `model`（即 `--input` 传入的模型名称）。

## 输出说明

工作流最终输出（`output: "{{step.doc}}"`）为一份完整的测评报告，包含以下章节：

1. **模型卡**：模型基本信息、来源、License
2. **适配路径**：算子替换清单、图改写记录、框架转换说明
3. **量化方案**：量化方法、校准数据集、精度损失评估
4. **Benchmark 数据**：首 token 延迟、吞吐量、显存占用（`include_benchmark=true`）
5. **部署指南**：服务 Endpoint、调用示例、健康检查
6. **复现步骤**：从零到一的环境准备与命令清单
7. **风险说明**：已知问题、兼容性风险、回滚策略

报告默认输出为 Markdown，可通过 `doc` 步骤的 `format` 参数切换为 `html`。

## 七阶段步骤详解

| 阶段 | step id（plan / exec） | 节点 | 关键产物 |
|------|------------------------|------|----------|
| 1 搜索 | `search_plan` / `search` | `agent` + `ascend_model_search` | 候选模型列表 + 下载地址 |
| 2 验证 | `verify_plan` / `verify` | `agent` + `ascend_model_verify` | 完整性 / License / 兼容性报告 |
| 3 适配 | `adapt_plan` / `adapt` | `agent` + `ascend_model_adapt` | 昇腾可运行模型权重 + 改写日志 |
| 4 量化 | `quantize_plan` / `quantize` | `agent` + `ascend_model_quantize` | 量化后模型 + 精度评估 |
| 5 调优 | `optimize_plan` / `optimize` | `agent` + `ascend_model_optimize` | 调优配置 + 性能数据 |
| 6 部署 | `deploy_plan` / `deploy` | `agent` + `ascend_model_deploy` | 推理服务 Endpoint |
| 7 文档 | `doc_plan` / `doc` | `agent` + `ascend_model_doc` | 测评报告（最终交付物） |

每个阶段均遵循「先规划（`agent` 输出 JSON 策略）→ 再执行（`ascend_*` 节点落地）」的模式，阶段间通过 `{{step.<id>}}` 传递上一阶段产物。

## 自定义指南

### 1. 切换目标硬件

编辑 `workflow.yaml` 顶部 `inputs.target_npu`，或在运行时通过 `--set` 覆盖：

```bash
--set target_npu=310P   # 边缘侧部署
--set target_npu=910C   # 最新一代训练卡
```

### 2. 调整量化方法

不同硬件推荐的量化方法：

| 硬件 | 推荐方法 | 说明 |
|------|----------|------|
| `800I-A2` / `910C` | `fp8` | 原生 FP8 支持，精度损失最小 |
| `910B` | `w8a8` | 权重 + 激活 INT8，性价比高 |
| `310P` | `int8` | 边缘场景，极致压缩 |

```bash
--set quantize_method=fp8
```

### 3. 更换推理后端

`deploy` 步骤默认使用 `mindie`，可改为 `vllm-ascend`：

```yaml
- id: deploy
  node: ascend_model_deploy
  params:
    backend: "vllm-ascend"   # 修改此处
```

### 4. 跳过某个阶段

如不需要量化，可删除 `quantize_plan` / `quantize` 两个 step，并将 `optimize_plan` 的 `depends_on` 改为 `[adapt]`，`optimize_plan` prompt 中的 `{{step.quantize}}` 改为 `{{step.adapt}}`。

### 5. 使用编排 Agent 一键启动

工作流也支持通过 `ascend_model_agent` 编排节点以 `mode=full` 启动，等效于上述七阶段串联：

```yaml
- id: full_pipeline
  node: ascend_model_agent
  params:
    model_name: "{{input}}"
    mode: "full"
    hardware: "{{target_npu}}"
```

## 环境前置要求

- llm-box CLI（`llm-box --version`）
- CANN Toolkit ≥ 7.0 + NPU 驱动（`npu-smi info` 可用）
- PyTorch / MindSpore 昇腾适配版
- MindIE（部署阶段）
- `ascend_*` 节点已注册（参考上级目录 README 的「自定义与扩展」）

## 相关模板

- 快速适配：[`../quick-adapt/`](../quick-adapt/)（仅搜索→验证→适配→文档，1 小时内）
- 性能调优：[`../performance-tune/`](../performance-tune/)（赛道二，适配→量化→调优→部署→文档）

## 分类

`ascend-adapt` / 端到端 / 赛道一
