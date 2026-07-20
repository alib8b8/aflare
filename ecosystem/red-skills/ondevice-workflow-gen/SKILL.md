# 端侧AI工作流生成器

## 描述

用自然语言描述你想做的事，自动生成可在手机/平板/车机上本地运行的 AI 工作流，无需联网、无需云端API Key，数据完全留在设备端。

基于 llm-box 的 `ondevice_llm` + `system_event` + `power_manager` 三大节点，一键生成隐私优先的端侧 AI 方案。

## 作者

llm-box 开源社区

## 标签

端侧AI, 本地推理, 隐私保护, 工作流, 离线AI, 手机AI

## 使用方式

### 方式一：一句话生成工作流

```bash
# 安装 llm-box
brew install alib8b8/tap/llm-box

# 用自然语言生成工作流
llm-box create "每天早上8点，自动总结昨晚的未读消息，并用语音播报" \
  --platform mobile \
  --offline \
  --output morning-brief.yaml

# 运行
llm-box run morning-brief.yaml
```

### 方式二：交互式创建

```bash
llm-box create --interactive --template ondevice-assistant

# 按提示输入：
# 1. 触发方式（定时/通知/语音/位置）
# 2. 执行动作（总结/翻译/回复/提醒）
# 3. 目标设备（手机/平板/车机/手表）
# 4. 功耗偏好（省电/均衡/性能）
```

## 支持的端侧模型

| 模型 | 大小 | 适用场景 | 量化推荐 |
|------|------|---------|---------|
| Qwen2-1.5B | 1.5B | 通用对话、文本生成 | INT4 |
| MiniCPM-2B | 2B | 中文理解、摘要 | INT4 |
| Phi-3 Mini | 3.8B | 代码生成、推理 | INT4 |
| Gemma-2B | 2B | 轻量级任务 | INT4 |
| Llama 3.2-3B | 3B | 多语言、工具调用 | INT8 |
| 通义千问1.5-6B | 6B | 复杂推理（需8GB+内存）| INT8 |

## 支持的触发方式

| 触发类型 | 说明 | 示例 |
|---------|------|------|
| 定时触发 | Cron表达式 | 每天早上8点 |
| 通知监听 | 监听系统通知 | 收到微信消息时 |
| 来电触发 | 接听电话时 | 自动播报来电人 |
| 位置变化 | 进入/离开某区域 | 到达公司自动静音 |
| 电量告警 | 低电量时 | 自动关闭后台应用 |
| 屏幕状态 | 亮屏/熄屏 | 亮屏时显示待办 |
| 蓝牙连接 | 连接车载/耳机 | 连上车机自动导航回家 |
| WiFi连接 | 连接特定网络 | 连上家里WiFi自动备份照片 |

## 输出示例

```yaml
name: 晨间简报
version: "1.0"
offline: true
description: 每天早上自动总结未读消息并语音播报

trigger:
  type: system_event
  event_type: alarm_triggered
  filter_keyword: "晨间简报"

power_profile: eco

steps:
  - node: system_event
    id: check_messages
    params:
      event_type: notification
      filter_app: com.tencent.mm
      debounce_ms: 5000

  - node: ondevice_llm
    id: summarize
    params:
      model: qwen2-1.5b
      quantization: int4
      system_prompt: "用30字以内总结以下消息要点："
      max_tokens: 100

  - node: notify
    id: voice_broadcast
    params:
      channel: tts
      priority: high
```

## 隐私说明

- **零云端传输**：所有推理在本地完成，不调用任何云端API
- **零数据上传**：消息内容、个人数据不出设备
- **可离线运行**：无需网络连接，飞行模式可用
- **开源可审计**：代码完全开源，可自行验证隐私策略

## 硬件要求

| 设备类型 | 最低配置 | 推荐配置 |
|---------|---------|---------|
| 手机 | 4GB RAM | 8GB RAM + NPU |
| 平板 | 4GB RAM | 6GB RAM |
| 车机 | 2GB RAM | 4GB RAM |
| 手表 | 1GB RAM | 2GB RAM（仅支持0.5B模型）|

## 依赖

- llm-box >= 0.6.0（含 ondevice_llm / system_event / power_manager 节点）
- 端侧模型文件（首次使用自动下载或手动放置）

## 开源地址

https://gitcode.com/llm-box/llm-box

## 相关资源

- [端侧模型适配指南](https://gitcode.com/llm-box/llm-box/-/blob/main/docs/ondevice-llm.md)
- [功耗优化最佳实践](https://gitcode.com/llm-box/llm-box/-/blob/main/docs/power-management.md)
- [鸿蒙设备适配文档](https://gitcode.com/llm-box/llm-box/-/blob/main/docs/harmonyos-adaptation.md)
