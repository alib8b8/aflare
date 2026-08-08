# Physical AI – 宇树机器人智能巡逻与 AI 视觉异常响应

> 🏆 aflare Killer Demo — 展示 Physical AI 完整闭环：AI 大脑 + 机器人身体 + 工作流编排

---

## 一、Demo 概述

这个 Demo 展示了一个**真实的物理 AI 应用场景**：宇树（Unitree）四足机器人自主执行园区巡逻任务，通过 AI 视觉实时分析环境图像，当检测到异常（入侵、火灾、设备故障等）时自动触发多步骤事件响应，包括现场警报、安保通知、事件记录，甚至派出无人机进行航拍确认。

整个流程由 **aflare 工作流引擎** 驱动，无需编写一行代码，全部通过 YAML 声明式配置完成。

### 核心能力展示

| 能力 | 技术实现 | 业务价值 |
|------|----------|----------|
| **机器人自主巡逻** | `unitree_robot` 节点，支持站立/行走/导航/拍照/蹲下 | 7×24 无间断园区安保 |
| **AI 视觉异常检测** | `multimodal` 节点，调用 GPT-4o 等视觉大模型分析机器人拍摄的图像 | 实时发现安全隐患，远超人工巡检效率 |
| **Saga 事务事件响应** | Saga 编排：警报 → 通知 → 记录，任一步骤失败自动补偿回滚 | 确保事件响应操作的原子性和可靠性 |
| **无人机协同** | `execute` 节点触发无人机升空，`multimodal` 分析航拍图像 | 地面+空中立体监控，无死角覆盖 |
| **定时调度** | `schedule` 配置，每 2 小时自动执行 | 全自动化运营，零人工干预 |
| **安全机制** | `safety_zone_m` 安全区域、`safety_checks` 自动校验 | 人机协作环境下的安全保障 |
| **审计日志** | `file_write` 追加模式 + HMAC 哈希链 | 完整的事件追溯和合规审计 |

---

## 二、Physical AI 概念

### 什么是 Physical AI？

**Physical AI = AI 模型 + 物理实体 + 工作流编排**

传统的 AI 停留在"数字世界"——写代码、生成文本、分析数据。Physical AI 将 AI 的能力延伸到**物理世界**，让 AI 直接与真实环境交互：

```
         ┌──────────────────────────────────────┐
         │            Physical AI 架构           │
         │                                       │
         │   ┌─────────┐    ┌──────────────┐    │
         │   │  感知层  │───→│   推理决策层   │    │
         │   │ (摄像头) │    │ (LLM Vision) │    │
         │   └─────────┘    └──────┬───────┘    │
         │                         │             │
         │   ┌─────────┐    ┌──────▼───────┐    │
         │   │  执行层  │◀───│   编排调度层   │    │
         │   │ (机器人) │    │  (aflare)    │    │
         │   └─────────┘    └──────────────┘    │
         │                                       │
         │   感知 → 推理 → 决策 → 执行 → 反馈    │
         └──────────────────────────────────────┘
```

### 本 Demo 的 Physical AI 闭环

1. **感知**：机器人摄像头拍摄环境图像
2. **推理**：LLM 视觉模型分析图像，识别异常
3. **决策**：条件判断是否触发事件响应
4. **执行**：机器人移动、警报触发、无人机升空
5. **反馈**：审计日志、报告生成、状态通知

---

## 三、工作流结构

### 流程图

```
                    ┌──────────────────┐
                    │  Schedule (cron) │
                    │  每 2 小时执行     │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  ① dock_stand   │  机器人从 dock 站立
                    │  ② preflight     │  起飞前自检（电池/温度/IMU/GPS）
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┬──────────────┬──────────────┐
              │              │              │              │              │
     ┌────────▼────────┐ ┌──▼──────────┐ ┌──▼──────────┐ ┌──▼──────────┐ ┌──▼──────────┐
     │ 检查点 1: 主入口  │ │ 检查点 2: 停车场│ │检查点 3: 仓库A│ │检查点 4: 机房│ │检查点 5: 围墙│
     │  navigate →      │ │ navigate →   │ │ navigate →   │ │ navigate →   │ │ navigate →   │
     │  snapshot →      │ │ snapshot →   │ │ snapshot →   │ │ snapshot →   │ │ snapshot →   │
     │  vision_analysis │ │ vision_analysis│ │vision_analysis│ │vision_analysis│ │vision_analysis│
     └────────┬────────┘ └──┬──────────┘ └──┬──────────┘ └──┬──────────┘ └──┬──────────┘
              │              │              │              │              │
              └──────────────┴──────────────┼──────────────┴──────────────┘
                                            │
                                   ┌────────▼─────────┐
                                   │ ③ aggregate      │  汇总所有分析结果
                                   │ ④ detect_anomaly │  LLM 综合判断是否有异常
                                   └────────┬─────────┘
                                            │
                              ┌─────────────┴─────────────┐
                              │                           │
                     ┌────────▼────────┐          ┌──────▼──────┐
                     │  ⚠️ 发现异常     │          │  ✅ 正常     │
                     └────────┬────────┘          └──────┬──────┘
                              │                          │
              ┌───────────────┼───────────────┐          │
              │               │               │          │
     ┌────────▼────────┐ ┌───▼──────┐ ┌──────▼──────┐   │
     │ ⑤ Saga: 事件响应 │ │⑥ 无人机   │ │⑦ 异常报告   │   │
     │  trigger_alarm  │ │ 航拍检查  │ │ 写入文件    │   │
     │  → notify_sec   │ │ → AI分析 │ │             │   │
     │  → log_incident │ │          │ │             │   │
     └────────┬────────┘ └────┬─────┘ └──────┬──────┘   │
              │               │               │          │
              └───────────────┴───────────────┘          │
                              │                          │
                              └──────────┬───────────────┘
                                         │
                                ┌────────▼─────────┐
                                │ ⑧ return_to_dock │  返回 dock
                                │ ⑨ dock_sit       │  蹲下休息
                                │ ⑩ final_status   │  最终状态
                                └────────┬─────────┘
                                         │
                                ┌────────▼─────────┐
                                │ ⑪ patrol_complete│  输出完成通知
                                └──────────────────┘
```

### 节点说明

| 步骤 | 节点类型 | 说明 |
|------|----------|------|
| `dock_stand` | `unitree_robot` | 机器人从 dock 站立，准备巡逻 |
| `preflight_check` | `unitree_robot` | 起飞前检查电池、温度、IMU、GPS |
| `patrol_checkpoint_N` | `unitree_robot` | 导航到指定检查点 |
| `snapshot_N` | `unitree_robot` | 拍摄检查点照片 |
| `vision_analysis_N` | `multimodal` | 调用 GPT-4o 视觉分析照片 |
| `aggregate_results` | `template_render` | 汇总 5 个检查点的分析结果 |
| `detect_anomaly` | `agent` | LLM 综合判断是否存在异常 |
| `incident_response` | `saga` | 事件响应事务（警报→通知→日志） |
| `drone_aerial_inspection` | `execute` | 触发无人机航拍 |
| `drone_image_analysis` | `multimodal` | 分析无人机航拍图像 |
| `save_anomaly_report` | `file_write` | 异常报告存档 |
| `return_to_dock` | `unitree_robot` | 返回充电 dock |
| `dock_sit` | `unitree_robot` | 蹲下待机 |
| `final_status` | `unitree_robot` | 最终状态检查 |

---

## 四、安装与运行

### 前置条件

```bash
# 确保已安装 aflare CLI
aflare version

# 设置 OpenAI API Key（用于视觉分析）
export OPENAI_API_KEY="sk-..."

# 可选：设置审计 HMAC 密钥（防篡改审计链）
export AFLARE_AUDIT_HMAC_KEY="your-secret-hmac-key"
```

### 模拟模式运行（推荐首次测试）

默认使用 `simulate` 模式，无需真实机器人硬件，所有机器人动作和拍照操作均为模拟：

```bash
# 从仓库根目录运行
aflare run examples/killer-demos/03-physical-ai-robot/workflow.yaml
```

预期输出：
- 机器人模拟站立、巡逻、拍照
- multimodal 节点调用 GPT-4o 分析（模拟的）照片
- 根据分析结果进入正常/异常分支
- 最终输出完成通知

### 定时调度运行

```bash
# 每 2 小时自动执行一次巡逻
aflare schedule \
  --id robot-patrol \
  --cron "0 */2 * * *" \
  examples/killer-demos/03-physical-ai-robot/workflow.yaml

# 查看调度状态
aflare schedule --info robot-patrol

# 查看调度日志
tail -f ~/.aflare/logs/scheduler.log
```

### 真实机器人模式

连接宇树 Go2/B2 实体机器人时，切换到 API 模式：

```bash
# 设置机器人 IP
export UNITREE_ROBOT_IP="192.168.1.100"

# 修改 workflow.yaml 中的 robot_mode 为 "api"
# 或通过命令行覆盖变量：
aflare run examples/killer-demos/03-physical-ai-robot/workflow.yaml \
  --var robot_mode=api \
  --var robot_ip=192.168.1.100
```

### 完整配置（安保通知）

```bash
# 设置安保通知渠道（Webhook 或 Telegram）
export SECURITY_WEBHOOK_URL="https://hooks.slack.com/services/xxx"
# 或
export SECURITY_TELEGRAM_CHAT_ID="-1001234567890"
export SECURITY_BOT_TOKEN="123456:ABC-DEF1234"

aflare run examples/killer-demos/03-physical-ai-robot/workflow.yaml
```

---

## 五、预期输出

### 正常巡逻输出

```
[notify] 巡逻完成 - 所有检查点正常，未发现异常。
============================================
Physical AI 巡逻任务完成
============================================
机器人: Go2
模式: simulate
检查点: 5 个
异常检测: NO
日志: ./patrol-logs/run-20260807T120000Z.jsonl
============================================
```

### 异常响应输出

```
[ALARM] 异常事件触发 - 检查点发现安全隐患
时间: 2026-08-07T12:00:00Z
详情: 停车场发现未授权车辆，后门围墙有攀爬痕迹...

[notify] webhook sent to https://hooks.slack.com/... (status 200)
[DRONE] 无人机升空进行航拍检查
[notify] 巡逻完成 - 异常响应已触发，报告已生成。
============================================
Physical AI 巡逻任务完成
============================================
机器人: Go2
异常检测: YES
日志: ./patrol-logs/run-20260807T120000Z.jsonl
异常报告: ./patrol-logs/anomaly-run-20260807T120000Z.md
============================================
```

### 生成的文件

| 文件 | 路径 | 内容 |
|------|------|------|
| 巡逻日志 | `./patrol-logs/<run-id>.jsonl` | JSONL 格式的巡逻记录 |
| 异常报告 | `./patrol-logs/anomaly-<run-id>.md` | Markdown 格式的异常事件报告 |

---

## 六、自定义配置

### 调整检查点

修改 `workflow.yaml` 中的 `vars.checkpoints` 列表，添加或删除检查点：

```yaml
vars:
  checkpoints:
    - name: "数据中心"
      location: "data_center"
      duration: 15
      description: "数据中心区域"
    - name: "化学品仓库"
      location: "chemical_storage"
      duration: 10
      description: "危险品存储区域，重点关注泄漏"
```

同时需要在 `steps` 中对应添加/删除检查点的 patrol → snapshot → vision_analysis 步骤。

### 更换机器人型号

支持宇树全系列：Go2、B2、B2-W、Go1、A1、H1、H1-2、G1、G1-Humanoid

```yaml
vars:
  robot_model: "B2"        # 工业级四足机器人
  # robot_model: "H1"      # 人形机器人
  # robot_model: "G1"      # 人形机器人
```

### 调整巡逻频率

```yaml
schedule:
  cron: "0 */1 * * *"        # 每 1 小时
  # cron: "0 8,20 * * *"     # 每天 8:00 和 20:00
  # cron: "0 6 * * 1-5"      # 工作日早 6:00
```

### 更换视觉模型

```yaml
vars:
  vision_provider: "ollama"     # 使用本地模型
  vision_model: "llava"         # LLaVA 视觉模型
  # vision_provider: "openai"   # OpenAI GPT-4o
  # vision_model: "gpt-4o"
```

### 自定义异常检测 Prompt

修改 `vision_analysis_N` 步骤的 `input` 字段，针对不同场景编写检测逻辑：

```yaml
- name: vision_analysis_1
  node: multimodal
  input: |
    你是化工厂安全巡检员。重点检测：
    - 化学品泄漏（液体/气体）
    - 管道压力表异常
    - 安全阀状态
    - 人员防护装备佩戴情况
```

---

## 七、架构亮点

### 1. Saga 事务保证事件响应可靠性

事件响应涉及多个步骤（警报 → 通知 → 记录），任一步骤失败都可能造成不一致状态。Saga 模式确保：

- **Forward 全部成功**：事件响应提交
- **某 Forward 失败**：已完成的步骤按反向顺序执行 Compensate 补偿
- **Compensate 失败**：best-effort 告警，需人工介入

```yaml
- name: incident_response
  saga:
    steps:
      - forward: { name: trigger_alarm, ... }
      - forward:
          name: notify_security, ...
        compensate:
          name: notify_cancel, ...       # 通知失败时撤销警报
      - forward:
          name: log_incident, ...
        compensate:
          name: log_cancel, ...           # 日志失败时标记取消
```

### 2. 声明式工作流，零代码

整个巡逻、视觉分析、事件响应流程完全通过 YAML 配置，无需编写 Go/Python/JS 代码。修改检查点、调整检测逻辑、更换通知渠道只需编辑 YAML。

### 3. 模拟模式降低测试门槛

默认 `simulate` 模式让开发者无需真实机器人即可测试完整流程。切换到 `api` 模式即可对接真实硬件。

### 4. 安全第一

- `safety_zone_m`：每个动作都配置安全区域半径
- `safety_checks`：自动校验速度、动作类型、环境安全
- 审计日志：所有步骤自动 HMAC 哈希链存档

---

## 八、扩展方向

- **多机器人协同**：多台 Go2/B2 分区巡逻，通过 supervisor 节点协调
- **边缘部署**：将视觉模型部署在本地 Ollama，降低延迟和 API 成本
- **热成像集成**：对接红外摄像头，检测温度异常
- **人脸识别**：集成人脸识别 API，识别白名单/黑名单人员
- **自动充电**：电量低于阈值时自动返回 dock 充电

---

## 九、文件清单

```
03-physical-ai-robot/
├── workflow.yaml    # 主工作流定义（机器人巡逻 + AI 视觉 + Saga 响应）
└── README.md        # 本文档
```

---

## 十、相关资源

- [aflare 调度文档](../../docs/scheduling.md)
- [宇树机器人节点文档](../../docs/nodes-reference.md)
- [Saga 事务模式示例](../finance/saga-transfer/)
- [aflare 快速入门](../../docs/getting-started.md)