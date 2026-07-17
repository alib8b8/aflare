# llm-box HarmonyOS Agent Skills

llm-box AI 工作流引擎为 HarmonyOS 提供的 Agent Skills 集合。

## 概述

本 Skill 包将 llm-box 的鸿蒙节点能力封装为 HarmonyOS Agent Skills，可被 DevEco Code、AI 助手等工具调用，辅助鸿蒙应用开发。

## 包含的 Skills

| Skill | 描述 | 对应节点 |
|---|---|---|
| `harmony-ability-launch` | 启动鸿蒙 Ability（page/slice/service/data） | HarmonyAbilityNode |
| `harmony-atomic-service` | 启动原子化服务（免安装卡片应用） | HarmonyAtomicServiceNode |
| `harmony-widget-manage` | 管理桌面卡片（添加/更新/删除/查询） | HarmonyWidgetNode |
| `harmony-device-adapt` | 多设备适配检测（7种设备类型） | HarmonyDeviceAdaptNode |
| `harmony-cross-app` | 跨应用工作流 | CrossAppActionNode |
| `harmony-agent-message` | 跨域 Agent 消息通信 | AgentMessageNode |
| `harmony-intent-router` | 意图路由分发 | IntentRouterNode |
| `harmony-device-state` | 设备状态查询 | DeviceStateNode |

## 使用方式

### 1. 通过 llm-box CLI 使用

```bash
# 启动鸿蒙 Ability
llm-box run --node harmony_ability --params '{"bundle_name":"com.example.app","ability_name":"MainAbility","ability_type":"page"}'

# 多设备适配检测
llm-box run --node harmony_device_adapt --params '{"device_type":"phone_dual_fold","fold_state":"half_folded"}'
```

### 2. 通过工作流 YAML 使用

```yaml
steps:
  - node: harmony_device_adapt
    id: detect_device
    params:
      device_type: phone_dual_fold
      fold_state: half_folded

  - node: harmony_ability
    id: launch_main
    params:
      bundle_name: com.example.myapplication
      ability_name: MainAbility
      ability_type: page
```

### 3. 通过 ohos:// 意图协议使用

```
ohos://workflow/harmony_ability?bundle_name=com.example.app&ability_name=MainAbility&ability_type=page
ohos://workflow/harmony_device_adapt?device_type=phone_dual_fold&fold_state=half_folded
```

## 安全特性

- Ability 类型白名单验证（page/slice/service/data）
- 原子化服务动作白名单（launch/router/share/notify）
- Widget 操作白名单（add/update/remove/query）
- App 名称正则验证（防注入）
- URI scheme 验证（禁止 file://、data://、javascript://）
- 参数长度和数量限制
- JSON 参数递归验证

## 设备类型支持

| 设备类型 | 标识 | 特殊能力 |
|---|---|---|
| 直板机 | phone_standard | camera, gps, nfc, biometrics |
| 双折叠 | phone_dual_fold | foldable_screen, multi_window, drag_to_split |
| 三折叠 | phone_triple_fold | foldable_screen, multi_window, drag_to_split |
| 平板 | tablet | stylus, multi_window, split_screen |
| 智慧屏 | smart_screen | voice, gesture, remote_control |
| 车机 | car | steering_wheel_control, hud, voice |
| 穿戴 | wearable | heart_rate, accelerometer, gyroscope |

## 许可证

MIT
