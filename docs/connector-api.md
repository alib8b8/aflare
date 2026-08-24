# Connector API 设计

> 状态：草案（骨架已实现，见文末 Roadmap）
> 定位：aflare 是 AI 与用户数据之间「确定且安全」的控制层。Connector API
> 是这个控制层的数据源接入标准 —— 用户自带数据源（个人 = 本机 / 个人账号，
> 企业 = 内网系统），aflare 只负责命名连接、凭据隔离与权限控制。

## 1. 解决什么问题

现状（`sql_query` 节点）把 `driver` + `dsn` 内联在 workflow YAML 里：

```yaml
# 现状：凭据直接进入工作流文件，随日志/分享/版本管理扩散
params:
  driver: postgres
  dsn: "postgres://ro:hunter2@db.internal:5432/app"
```

问题：

1. **凭据泄漏面**：workflow 文件会被分享、提交进 git、进入会话记录。
2. **无策略上限**：工作流可以 `read_only=false`、`max_rows=100000`，引擎
   层没有「这个数据源最多能做什么」的声明。
3. **个人/企业同构不同配**：个人用户（keychain 凭据）和企业用户（内网
   Vault/SSO）没有统一的抽象点。

## 2. 核心概念

```
workflow (AI 生成)          aflare 控制层                     用户数据源
┌───────────────┐   ┌──────────────────────────┐   ┌──────────────────┐
│ sql_query     │   │ Connector Registry       │   │ PostgreSQL /     │
│  connector: ──────▶ Spec(命名+端点+上限)      │   │ MySQL / SQLite / │
│  sql: ...     │   │ CredentialResolver ────────▶ (凭据从不进 YAML) │
└───────────────┘   │ BuildDSN + 权限天花板     │   │ 内网 API (后续)  │
                    └──────────────────────────┘   └──────────────────┘
```

### 2.1 ConnectorSpec —— 命名连接

存储于 `~/.aflare/config/connectors.yaml`（`AFLARE_CONNECTORS_FILE` 可覆
盖），文件权限 0600，原子写：

```yaml
version: 1
connectors:
  - name: my-pg                # ^[a-z][a-z0-9-]{0,63}$
    type: postgres             # postgres | mysql | sqlite
    host: db.internal.example.com
    port: 5432                 # 缺省用类型默认端口
    database: analytics
    username: readonly_user
    credential:                # 引用，不是值
      kind: secret             # secret（加密库）| env（环境变量）
      group: connectors
      key: my-pg               # aflare secrets set connectors my-pg
    read_only: true            # 默认 true，写需显式声明
    max_rows: 1000             # 结果行数天花板，默认 1000
    timeout: 30                # 查询超时天花板（秒），默认 30
```

### 2.2 CredentialResolver —— 部署 profile 的抽象点

`internal/connector.CredentialResolver` 接口是个人版与企业版的分界：

| Profile | kind=secret 实现 | kind=env 实现 |
|---|---|---|
| 个人（默认） | `secrets.SecretManager`（AES-256-GCM/SM4 加密 + 系统钥匙串托管主密码） | 进程环境变量 |
| 企业（Roadmap） | Vault / SSO 短期凭据（同接口实现，启动时注入） | 内网注入的环境变量 |

统一代码库、不同部署 profile：引擎不感知 profile，只感知 Resolver 接口。

### 2.3 权限模型 —— 连接器是天花板

`sql_query` 通过 `connector: <name>` 引用连接器后：

| 维度 | 合并规则 |
|---|---|
| `read_only` | `节点read_only OR 连接器read_only` —— 只读连接器无法被工作流写 |
| `max_rows` | 未设置取连接器值；设置了取 `min(节点值, 连接器值)` |
| `timeout` | 同上 |

即：**节点参数只能收紧、不能放宽连接器策略**。这与现有
`read_only`（默认 true）、参数化查询（防注入）形成纵深防御。

### 2.4 DSN 构建

`connector.BuildDSN(spec, password)` 按类型渲染 DSN：

- **postgres**：`postgres://user:pass@host:port/db`，用户名/密码经
  `url.UserPassword` 百分号编码 —— 密码中的 `:@/?` 无法破坏 DSN 结构。
- **mysql**：`user:pass@tcp(host:port)/db`（go-sql-driver 格式）。
- **sqlite**：文件路径即 DSN，凭据被忽略。

驱动本身仍由宿主程序注册（`sql_query` 不引入第三方/CGO 依赖），
`type → driver` 映射：postgres→`postgres`、mysql→`mysql`、sqlite→`sqlite3`。

## 3. 使用方式

```bash
# 1) 存凭据（值加密落盘，主密码由系统钥匙串托管）
aflare secrets set connectors my-pg

# 2) 注册命名连接器（只读默认开启）
aflare connector add my-pg --type postgres --host db.internal \
  --database analytics --username readonly_user --credential-group connectors

# 3) 查看与校验
aflare connector list
aflare connector show my-pg

# 4) 工作流引用 —— 文件里只有名字，没有任何凭据
```

```yaml
# workflow.yaml
steps:
  - node: sql_query
    params:
      connector: my-pg
      sql: "SELECT count(*) FROM orders WHERE created_at > $1"
      args: '["2026-01-01"]'
```

内联 `driver/dsn` 与 `connector` 互斥（同时出现报错）；二者都缺省时报
参数错误。旧的内联写法保持兼容，但文档推荐 connector 模式。

## 4. 安全边界（骨架已实现）

- 凭据只存 secrets store / 环境变量，spec 文件与 workflow 文件只有引用。
- 连接器默认只读；`--writable` 需管理员显式声明，且工作流仍需
  `read_only=false` 双重确认才能执行写语句。
- 名称/端点/凭据引用字段全量校验（null 字节、端口范围、名称格式）。
- 注册表加载时逐条校验，坏数据**报错拒绝**而非静默丢弃。
- 结果行数/超时天花板随连接器下发。
- 0600 原子写（tmp+rename，防 symlink 替换攻击 —— 与 secrets store 同
  标准的 TOCTOU 加固）。

## 5. Roadmap

| 阶段 | 内容 | 状态 |
|---|---|---|
| PR #95（本 PR） | Spec/Registry/Resolver/DSN + sql_query 接入 + CLI 四命令 | ✅ 骨架 |
| 下一步 | HTTP/API 连接器（http_request 节点接 `connector`，Bearer/Basic 头注入）；策略引擎网络规则与连接器 host 联动（内网 allowlist） | 计划 |
| 之后 | 企业 profile：Vault/SSO Resolver、连接器级审计事件、DB 连接池按连接器名缓存（当前按 driver+dsn，含密码的 key 内存驻留时间需要缩短） | 计划 |

## 6. 与项目定位的关系

模板/内容生态已移除（PR #94）。aflare 的核心价值收敛为：**无论云端 AI
还是用户自部署的模型，接入 aflare 后，个人/企业用户把自己的数据源通过
Connector 接入，由 aflare 让 AI 确定且安全地运行。** Connector API 就是
这条主线上的第一块基石：引擎（workflow nodes）+ 连接层（connector）+
安全模型（secrets/policy/ceiling 合并）。
