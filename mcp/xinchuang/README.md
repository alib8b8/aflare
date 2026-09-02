# aflare 信创 MCP 连接器市场

> **状态：部分实现。** 通用版 `aflare mcp install` / `aflare mcp list` 已实现，支持安装
> 5 个通用社区 MCP server（见下方"当前支持清单"）。本文描述的**信创 35 连接器市场仍是
> 目标设计（规划中）**，`aflare mcp install xinchuang/<domain>` 形式的安装暂不可用。
> aflare 自带 MCP Server 的用法见 [docs/mcp.md](../docs/mcp.md)。

## 当前支持清单（已实现）

`aflare mcp install <name>` 写入项目目录 `.mcp.json`（幂等，重复安装提示已存在）；
`aflare mcp list` 列出以下内置 server 及安装状态。所有启动命令**锁定 registry 精确版本**
（供应链审计要求，随 release 统一 bump）：

| 名称 | 说明 | 启动命令 |
|------|------|---------|
| `fetch` | 抓取网页转 Markdown（官方 Python 实现） | `uvx mcp-server-fetch@2026.8.18` |
| `filesystem` | 受控读写本地目录 | `npx -y @modelcontextprotocol/server-filesystem@2026.8.31 .` |
| `memory` | 知识图谱持久记忆 | `npx -y @modelcontextprotocol/server-memory@2026.8.31` |
| `sequential-thinking` | 结构化分步推理 | `npx -y @modelcontextprotocol/server-sequential-thinking@2026.8.31` |
| `everything` | MCP 协议测试沙盒 | `npx -y @modelcontextprotocol/server-everything@2026.8.31` |

> 2026-09 移除：`git`、`sqlite`、`time` 三个 server 的 npm 包
> （`@modelcontextprotocol/server-git` 等）已从 registry 下架（404），
> 安装后无法启动，故从清单删除。

以下为信创连接器市场的**目标设计（规划中）**。

> 预置国产化系统 MCP 连接器，覆盖 OA、ERP、数据库、消息、安全等 15 个业务领域。
> 参考淘宝闪购 MCP 模式，按业务领域分类，一键安装即用。

## 安装方式（规划中，暂不可用）

```bash
# 安装单个领域的 MCP 连接器
aflare mcp install xinchuang/oa        # OA 审批（泛微/致远/蓝凌）
aflare mcp install xinchuang/erp       # ERP 操作（用友/金蝶/浪潮）
aflare mcp install xinchuang/database  # 数据库查询（OceanBase/DM/GaussDB）
aflare mcp install xinchuang/message   # 消息通知（企业微信/钉钉/飞书）
aflare mcp install xinchuang/security  # 安全审计（日志/扫描/合规）

# 一键安装全部信创连接器
aflare mcp install xinchuang/all
```

## 连接器目录（15 个领域，35 个 MCP Server）

### 1. 办公协同 (xinchuang/oa)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `weaver` | 泛微 e-cology / e-office | 发起审批、查询待办、审批流转、附件下载 |
| `seeyon` | 致远 A8+ / G6 | 公文管理、会议管理、表单流程、签报 |
| `landray` | 蓝凌 EKP | 知识管理、流程审批、日程协作 |

### 2. 企业资源 (xinchuang/erp)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `yonyou` | 用友 U8+ / NC Cloud / YonSuite | 财务凭证查询、采购订单、库存查询、报表 |
| `kingdee` | 金蝶云星空 / 苍穹 | 应收应付、成本核算、总账、固定资产 |
| `inspur` | 浪潮 GS Cloud / inSuite | 财务共享、资金管理、预算控制 |

### 3. 数据库 (xinchuang/database)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `oceanbase` | OceanBase 4.x | SQL 查询、表结构、执行计划、慢查询分析 |
| `dameng` | 达梦 DM8 | SQL 查询、模式管理、备份状态、性能监控 |
| `gaussdb` | openGauss / GaussDB | SQL 查询、AIOPS 诊断、WDR 报告 |
| `tdsql` | TDSQL MySQL / PostgreSQL | SQL 查询、分片状态、主备延迟 |

### 4. 消息通知 (xinchuang/message)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `wecom` | 企业微信 | 发送消息、群机器人、应用消息、文件上传 |
| `dingtalk` | 钉钉 | 发送消息、群机器人、工作通知、OA 审批 |
| `feishu` | 飞书 | 发送消息、卡片消息、群管理、文件上传 |

### 5. 安全审计 (xinchuang/security)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `sangfor` | 深信服 SIP/EDR | 告警查询、资产发现、漏洞扫描 |
| `qianxin` | 奇安信天眼/SkyEye | 威胁情报查询、日志检索、告警处置 |
| `venus` | 启明星辰 USM | 日志审计、合规检查、事件关联 |

### 6. 云平台 (xinchuang/cloud)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `huawei` | 华为云 Stack | ECS 管理、VPC 配置、OBS 存储、IAM 权限 |
| `alibaba` | 阿里云飞天企业版 | ECS 管理、VPC 配置、OSS 存储、RAM 权限 |
| `tencent` | 腾讯云 TCE | CVM 管理、VPC 配置、COS 存储、CAM 权限 |

### 7. 监控运维 (xinchuang/ops)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `zabbix` | Zabbix 6.x LTS | 告警查询、主机状态、监控项历史、趋势 |
| `prometheus` | Prometheus + AlertManager | 指标查询、告警静默、规则管理 |
| `nightingale` | 夜莺 Nightingale v6 | 告警管理、指标查询、仪表盘、团队协作 |

### 8. 身份认证 (xinchuang/iam)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `bamboocloud` | 竹云 IDaaS | 用户同步、SSO 配置、权限策略、审计日志 |
| `authing` | Authing | 用户管理、OAuth/OIDC 配置、MFA 策略 |

### 9. 电子签章 (xinchuang/sign)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `esign` | 契约锁 / e签宝 | 发起签署、查询状态、下载文件、批量签署 |

### 10. 电子发票 (xinchuang/invoice)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `baiwang` | 百望云 | 发票开具、发票查验、进项管理、销项管理 |

### 11. 智能客服 (xinchuang/cs)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `xiaoi` | 小 i 机器人 | 知识库查询、对话管理、意图识别 |
| `qiyezhushou` | 企业助手 | 工单查询、知识检索、FAQ 匹配 |

### 12. 物联网 (xinchuang/iot)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `huawei_iot` | 华为 IoT 平台 | 设备状态、数据查询、命令下发、规则引擎 |
| `tuya` | 涂鸦智能 IoT | 设备控制、场景联动、数据统计 |

### 13. 区块链 (xinchuang/blockchain)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `chainmaker` | 长安链 ChainMaker | 合约调用、交易查询、区块查询、链上存证 |

### 14. 地理信息 (xinchuang/gis)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `supermap` | 超图 SuperMap iServer | 地图服务、空间查询、路径分析、热力图 |

### 15. 机器人控制 (xinchuang/robot)
| MCP Server | 对接系统 | 能力 |
|-----------|---------|------|
| `unitree` | 宇树 Go2/B2/H1/G1 | 动作控制、状态查询、摄像头、巡逻任务 |

## 使用示例

### 政务工单审批工作流

```yaml
# workflow: 市民诉求工单自动分派与审批
steps:
  - name: classify_work_order
    node: llm
    prompt: "将以下市民诉求分类到对应部门：{{input.content}}"
    model: qwen

  - name: dispatch_to_department
    node: mcp
    server: xinchuang/oa/weaver
    tool: create_approval
    params:
      title: "市民诉求工单 #{{input.id}}"
      department: "{{classify_work_order.department}}"
      content: "{{input.content}}"

  - name: notify_applicant
    node: mcp
    server: xinchuang/message/wecom
    tool: send_message
    params:
      user: "{{input.applicant_id}}"
      content: "您的诉求已受理，请关注审批进度"

  - name: save_to_database
    node: mcp
    server: xinchuang/database/oceanbase
    tool: execute_sql
    params:
      sql: "INSERT INTO work_orders (id, content, department, status) VALUES (?, ?, ?, 'pending')"
      params: ["{{input.id}}", "{{input.content}}", "{{classify_work_order.department}}"]
```

### 金融 AML 审查工作流

```yaml
steps:
  - name: check_blacklist
    node: mcp
    server: xinchuang/security/sangfor
    tool: threat_intelligence
    params:
      entity: "{{input.entity_name}}"
      type: "blacklist"

  - name: query_company_info
    node: mcp
    server: xinchuang/database/dameng
    tool: execute_sql
    params:
      sql: "SELECT * FROM enterprise_info WHERE name = ?"
      params: ["{{input.entity_name}}"]

  - name: sign_audit_report
    node: mcp
    server: xinchuang/sign/esign
    tool: initiate_sign
    params:
      file: "{{template_render.report_path}}"
      signers: ["{{input.auditor_id}}"]
```

## 自定义连接器

```bash
# 创建自定义 MCP 连接器
aflare mcp create --name my-oa --endpoint https://oa.example.com/api --token $OA_TOKEN

# 从 YAML 定义文件安装
aflare mcp install -f custom-connector.yaml
```

## 信创适配矩阵

| 领域 | 连接器数 | 芯片 | OS | 数据库 |
|------|---------|------|-----|--------|
| 办公协同 | 3 | - | 麒麟/统信 | - |
| 企业资源 | 3 | - | 麒麟/统信 | OceanBase/DM |
| 数据库 | 4 | 海光/鲲鹏 | openEuler | - |
| 消息通知 | 3 | - | - | - |
| 安全审计 | 3 | 海光 | openEuler | - |
| 云平台 | 3 | 鲲鹏 | openEuler | - |
| 监控运维 | 3 | - | openEuler | - |
| 其他领域 | 13 | - | - | - |
| **合计** | **35** | | | |