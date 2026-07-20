# 鸿蒙多设备适配助手

## 描述

输入你的设备类型（如直板手机、双折叠、三折叠、平板、智慧屏、车机、穿戴），自动生成鸿蒙 HarmonyOS 的完整 UI 适配方案，包括布局策略、断点设计、组件推荐和交互提示。

基于 llm-box AI 工作流引擎的 `harmony_device_adapt` 节点能力封装，覆盖 7 种鸿蒙官方设备类型。

## 作者

llm-box 开源社区

## 标签

鸿蒙, HarmonyOS, 多设备适配, UI设计, 前端开发, 移动端

## 使用方式

### 方式一：在线体验（推荐）

访问 llm-box 在线工作流编辑器，输入设备类型即可生成适配方案：

```bash
# 安装 llm-box
brew install alib8b8/tap/llm-box

# 运行适配工作流
llm-box run https://gitcode.com/llm-box/llm-box/-/raw/main/templates/harmony-device-adapt.yaml \
  --device "三折叠手机" \
  --app-type "社交应用"
```

### 方式二：本地运行

```bash
# 克隆仓库
git clone https://gitcode.com/llm-box/llm-box.git

# 运行设备适配节点
cd llm-box
llm-box node harmony_device_adapt \
  --device "双折叠" \
  --app_type "阅读器" \
  --output "adaptation-plan.json"
```

## 支持的设备类型

| 设备类型 | 特点 | 适配重点 |
|---------|------|---------|
| 直板手机 | 最常用形态 | 单屏竖向布局，底部导航 |
| 双折叠 | 内折/外折两种 | 展开/折叠态切换，双窗口 |
| 三折叠 | 华为Mate XT | 三态布局，多窗口并行 |
| 平板 | 大屏触控 | 分栏布局，悬浮面板 |
| 智慧屏 | 电视大屏 | 焦点导航，遥控器适配 |
| 车机 | 驾驶场景 | 大按钮，语音优先，安全模式 |
| 穿戴 | 手表小屏 | 圆形/方形适配，极简交互 |

## 输出示例

```json
{
  "device_type": "三折叠手机",
  "adaptation_strategy": {
    "single_screen": "竖向单屏布局，底部Tab导航",
    "dual_screen": "左右分栏，左侧导航右侧内容",
    "triple_screen": "三栏并行，主内容+侧边栏+详情"
  },
  "breakpoints": {
    "compact": "0-600dp",
    "medium": "600-840dp",
    "expanded": "840dp+"
  },
  "recommended_components": [
    "NavigationSplitView",
    "SideBarContainer",
    "GridLayout"
  ],
  "interaction_tips": [
    "展开态支持三指滑动切换分栏",
    "折叠态自动保存分栏状态",
    "悬停态显示快捷操作浮层"
  ]
}
```

## 依赖

- llm-box >= 0.5.0
- 可选：HarmonyOS DevEco Studio（用于验证生成的方案）

## 开源地址

https://gitcode.com/llm-box/llm-box
