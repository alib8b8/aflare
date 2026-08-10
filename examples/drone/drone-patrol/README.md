# Drone Patrol - 无人机巡检工作流

使用 aflare `drone` 节点控制 PX4/ArduPilot 无人机执行自动巡检。

## 快速开始

### 模拟模式（无需真实无人机）

```bash
# 直接运行，所有 drone 节点默认 mode=simulate
aflare run examples/drone/drone-patrol/workflow.yaml
```

### 真实无人机模式

1. 安装 Python MAVSDK bridge：

```bash
pip install pymavlink
```

2. 启动 drone bridge 连接无人机：

```bash
# 连接 SITL 模拟器
python scripts/drone_bridge.py --port 8080 --connection udp:127.0.0.1:14550

# 连接真实无人机（USB 串口）
python scripts/drone_bridge.py --port 8080 --connection /dev/ttyACM0:115200

# 连接真实无人机（网络）
python scripts/drone_bridge.py --port 8080 --connection tcp:192.168.1.100:5760
```

3. 修改 workflow.yaml 中所有 drone 节点的 `mode: simulate` 为 `mode: http`，并设置 `bridge_host` 和 `bridge_port`：

```yaml
params:
  action: takeoff
  mode: http                # 改为 http
  bridge_host: "127.0.0.1"  # bridge 地址
  bridge_port: "8080"       # bridge 端口
```

4. 运行巡检：

```bash
aflare run examples/drone/drone-patrol/workflow.yaml
```

## 工作流说明

| 阶段 | 步骤 | 说明 |
|------|------|------|
| 起飞前检查 | GPS/电池/状态 | 并行检查，确保飞行安全 |
| 解锁 & 起飞 | arm → takeoff | 按序执行，解锁后起飞到 20m |
| 巡检航线 | 上传航点 → 开始任务 | 上传 5 个航点组成的巡检航线 |
| 监控 | 轮询遥测 | 每 3 秒检查一次，电池 < 25% 自动中断 |
| 降落 | land → disarm | 降落并锁定电机 |
| 报告 | 生成摘要 | 汇总任务执行结果 |

## 支持的 drone 节点动作

| 动作 | 说明 |
|------|------|
| arm | 解锁电机 |
| disarm | 锁定电机 |
| takeoff | 起飞到指定高度 |
| land | 降落 |
| rtl | 返航到起飞点 |
| hold | 悬停 |
| goto | 飞往指定坐标 |
| mission_upload | 上传航点任务 |
| mission_start | 开始执行任务 |
| mission_pause | 暂停任务 |
| mission_resume | 继续任务 |
| mission_clear | 清除任务 |
| patrol | 巡逻模式 |
| survey | 测绘模式 |
| orbit | 环绕飞行 |
| follow | 跟随目标 |
| camera | 拍照 |
| deliver | 投递 |
| get_telemetry | 获取遥测 |
| get_gps | 获取 GPS 位置 |
| get_battery | 获取电池电量 |
| get_status | 获取系统状态 |

## 架构

```
aflare (drone node)
    │
    │ HTTP POST /api/v1/drone/<action>
    ▼
drone_bridge.py (Python HTTP server)
    │
    │ MAVLink over serial/UDP/TCP
    ▼
PX4 / ArduPilot (飞控)
```