# X-OmniClaw Skill Adapter for llm-box

将 llm-box 工作流引擎作为 X-OmniClaw 的技能层注入，使 X-OmniClaw 获得多节点工作流编排能力。

## 架构

```
X-OmniClaw (感知+记忆+行动循环)
    │
    ├── 全方位感知 (摄像头+屏幕+语音)
    │       ↓
    ├── llm-box Skill Adapter  ←── 本模块
    │       │
    │       ├── 解析用户意图 → 选择工作流模板
    │       ├── 编排节点执行 (50+ 节点)
    │       ├── 端侧推理 (ondevice_llm)
    │       ├── 系统事件触发 (system_event)
    │       └── 功耗管理 (power_manager)
    │       │
    ├── 全方位记忆 ←── 三层持久化记忆 (会话/任务/长期)
    │       ↓
    └── 全方位行动 (点击/滑动/输入)
```

## 安装

### 方式一：作为 X-OmniClaw Skill 安装

```python
# 在 X-OmniClaw 的 skills 目录下
git clone https://gitcode.com/llm-box/llm-box.git skills/llm-box
cd skills/llm-box
pip install -r ecosystem/oppo/x-omniclaw-adapter/requirements.txt
```

### 方式二：作为独立 Agent Sidecar 运行

```bash
# 启动 llm-box 作为 gRPC 服务
llm-box serve --port 50051 --mode sidecar

# X-OmniClaw 通过 gRPC 调用工作流
```

## Skill 接口

X-OmniClaw Skill 规范要求实现以下接口：

### `perceive(context) → intent`

从 X-OmniClaw 的感知上下文中提取用户意图：

```python
def perceive(context):
    """
    context: {
        "screen_text": "OCR提取的屏幕文本",
        "voice_text": "语音识别结果",
        "camera_objects": ["咖啡杯", "书"],
        "active_app": "com.tencent.mm",
        "user_location": {"lat": 39.9, "lng": 116.4}
    }
    """
    # 使用 llm-box 的 ondevice_llm 节点理解意图
    intent = llmbox.run("ondevice_llm", {
        "model": "qwen2-1.5b",
        "quantization": "int4",
        "input": f"理解用户意图：{context['voice_text'] or context['screen_text']}"
    })
    return intent
```

### `plan(intent, memory) → workflow`

将意图映射到 llm-box 工作流：

```python
def plan(intent, memory):
    """
    intent: "帮我总结今天的未读消息"
    memory: PersonaX 长期记忆
    """
    # 选择匹配的工作流模板
    workflow = llmbox.create(intent, platform="mobile", offline=True)
    return workflow
```

### `execute(workflow, context) → result`

执行工作流并返回行动指令：

```python
def execute(workflow, context):
    """
    workflow: llm-box YAML 工作流
    context: 当前设备上下文
    """
    # 功耗感知执行
    power = llmbox.run("power_manager", {"profile": "balanced"})
    
    # 执行工作流
    result = llmbox.run(workflow, context={
        "power_profile": power["effective_profile"],
        "device_context": context
    })
    
    # 转换为 X-OmniClaw 行动格式
    actions = to_omniclaw_actions(result)
    return actions
```

## 支持的 X-OmniClaw → llm-box 映射

| X-OmniClaw 能力 | llm-box 节点 | 说明 |
|-----------------|-------------|------|
| 全方位感知 (屏幕) | `screen_understanding` | UI元素解析+交互计划 |
| 全方位感知 (语音) | `voice_input` | VAD+唤醒词+ASR |
| 全方位感知 (视觉) | `multimodal` | 图像理解 |
| 全方位记忆 | 三层记忆系统 | 会话/任务/长期持久化 |
| 全方位行动 | `system_event` + `intent://` | 系统事件触发+跨应用意图 |
| 端侧推理 | `ondevice_llm` | 16个模型, 8种量化 |
| 功耗管理 | `power_manager` | eco/balanced/high 自适应 |
| 安全审计 | `blockchain_audit` | 工作流执行上链 |

## 示例：通过 X-OmniClaw 帮用户点咖啡

用户对手机说："帮我点一杯美式咖啡"

```
1. X-OmniClaw.voice_input → "帮我点一杯美式咖啡"
2. llm-box.ondevice_llm → 意图: "点咖啡", 参数: {type: "美式"}
3. llm-box 选择工作流模板: "food-ordering"
4. llm-box.screen_understanding → 找到瑞幸/星巴克App
5. llm-box.system_event → 打开App
6. llm-box.screen_understanding → 识别菜单界面
7. X-OmniClaw.action → 点击"美式咖啡"
8. X-OmniClaw.action → 点击"下单"
9. llm-box.blockchain_audit → 记录操作上链
```

## 开源地址

- llm-box: https://gitcode.com/llm-box/llm-box
- X-OmniClaw: https://github.com/OPPO-X-OmniClaw (OPPO开源)
