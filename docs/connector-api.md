# Connector API 设计

> 状态：骨架已实现（数据库 + 文件/笔记），见文末 Roadmap
> 定位：aflare 是 AI 与用户数据之间「确定且安全」的控制层。Connector API
> 是这个控制层的数据源接入标准 —— 用户自带数据源，aflare 只负责命名连
> 接、凭据隔离与权限控制。
> **目标用户：先做个人。** 个人用户的数据在本机：文件、笔记库、个人数据
> 库（SQLite）。文件/笔记/个人库连接器是第一优先级；企业 profile
> （Vault/SSO/内网系统）走同一套接口，排在后面。

## 1. 解决什么问题

**数据库**：现状（`sql_query` 节点）把 `driver` + `dsn` 内联在 workflow
YAML 里，凭据随日志/分享/版本管理扩散，且没有「这个数据源最多能做什
么」的策略声明。

**文件**：现状（`file_read`/`file_write`）把 AI 关进**工作目录**沙箱 ——
安全，但个人用户的数据（`~/notes`、`~/Documents`）都在沙箱外。要么放弃
访问，要么放开整个文件系统。缺一个「用户显式授权某个目录根 + 带策略上
限」的中间层。

Connector API 统一解决：

1. **凭据泄漏面**：workflow 文件只有连接器名字，凭据只存 secrets store /
   环境变量。
2. **无策略上限**：连接器声明天花板（只读、行数、字节数、超时、扩展名
   白名单），节点参数只能收紧、不能放宽。
3. **目录授权**：files/notes 连接器把 workdir 沙箱的同一套遏制规则
   （禁绝对路径、禁穿越、L2+ symlink 检查）应用到用户授权的目录根。

## 2. 核心概念

```
workflow (AI 生成)          aflare 控制层                     用户数据
┌───────────────┐   ┌──────────────────────────┐   ┌──────────────────────┐
│ file_read     │   │ Connector Registry       │   │ ~/notes (markdown)   │
│ file_write    │   │ Spec(命名+root/端点+上限) │   │ ~/Documents          │
│ files_list ──────▶ root 遏制 + include 白名单 │   │ 个人 SQLite 库        │
│ sql_query ──────▶ CredentialResolver + DSN   │   │ PostgreSQL / MySQL   │
└───────────────┘   │ 权限天花板合并            │   │ (企业内网，后续)      │
                    └──────────────────────────┘   └──────────────────────┘
```

### 2.1 ConnectorSpec —— 命名连接

存储于 `~/.aflare/config/connectors.yaml`（`AFLARE_CONNECTORS_FILE` 可覆
盖），文件权限 0600，原子写：

```yaml
version: 1
connectors:
  # —— 文件/笔记（个人优先主线）——
  - name: my-notes            # ^[a-z][a-z0-9-]{0,63}$
    type: notes               # notes = markdown 库（include 默认 *.md）
    root: /Users/me/notes     # 绝对路径；节点 path 相对它解析，逃逸即拒绝
    read_only: true           # 默认 true，file_write 直接拒绝
    max_bytes: 1048576        # 单文件读取上限，默认 10MB（节点硬上限之内）
  - name: my-docs
    type: files               # files = 任意目录，include 不设则全类型
    root: /Users/me/Documents
    read_only: false          # --writable 显式开启
    include: ["*.md", "*.txt"]  # 扩展名白名单（glob，匹配文件名）

  # —— 个人库（SQLite 文件）——
  - name: my-library
    type: sqlite
    database: /Users/me/calibre/metadata.db

  # —— 数据库（个人自建/远程，企业复用同一套）——
  - name: my-pg
    type: postgres
    host: db.internal.example.com
    port: 5432
    database: analytics
    username: readonly_user
    credential:               # 引用，不是值
      kind: secret            # secret（加密库）| env（环境变量）
      group: connectors
      key: my-pg
    read_only: true
    max_rows: 1000            # 默认 1000
    timeout: 30               # 默认 30s
```

类型与节点的路由：`files`/`notes` → file_read / file_write / files_list；
`postgres`/`mysql`/`sqlite` → sql_query。用错节点会得到指路的报错。

### 2.2 文件连接器的权限模型

files/notes 连接器把「workdir 沙箱」的规则**原样搬到授权目录根**：

| 维度 | 规则 |
|---|---|
| 路径遏制 | `path` 只能是 root 内的相对路径：拒绝绝对路径、`..` 穿越；L2+ 拒绝 symlink 逃逸（复用 `core.SafeJoinPath`） |
| `read_only` | 默认 true。只读连接器上 `file_write` 直接拒绝，节点参数无法放宽 |
| `include` | glob 白名单匹配文件名。notes 默认 `*.md`/`*.markdown`；files 不设则全类型 |
| `max_bytes` | 单文件读取上限（默认 10MB，且永不放宽节点 10MB 硬上限） |
| 写黑名单 | workdir 模式的敏感扩展名黑名单（`.env`/`.sh`/`.exe`…）与 dotfile 限制在 connector 模式**同样生效** |
| 列举 | `files_list` 跳过 dotfile/dot 目录（`.git`/`.obsidian`）与 symlink，结果上限 1000 条 |

### 2.3 数据库连接器的权限模型

`sql_query` 通过 `connector: <name>` 引用连接器后：

| 维度 | 合并规则 |
|---|---|
| `read_only` | `节点read_only OR 连接器read_only` |
| `max_rows` | 未设置取连接器值；设置了取 `min(节点值, 连接器值)` |
| `timeout` | 同上 |

**SQLite 纵深防御**：只读 sqlite 连接器（默认）的 DSN 附加
`file:...?mode=ro` —— 即使节点层只读门被绕过，驱动本身也拒绝写。

### 2.4 CredentialResolver —— 部署 profile 的抽象点

`internal/connector.CredentialResolver` 接口是个人版与企业版的分界：

| Profile | kind=secret 实现 | kind=env 实现 |
|---|---|---|
| 个人（默认） | `secrets.SecretManager`（AES-256-GCM/SM4 加密 + 系统钥匙串托管主密码） | 进程环境变量 |
| 企业（Roadmap） | Vault / SSO 短期凭据（同接口实现，启动时注入） | 内网注入的环境变量 |

统一代码库、不同部署 profile：引擎不感知 profile，只感知 Resolver 接口。
files/notes 连接器本地文件无需凭据，spec 直接拒绝 credential 字段。

### 2.5 DSN 构建

`connector.BuildDSN(spec, password)` 按类型渲染 DSN：

- **postgres**：`postgres://user:pass@host:port/db`，用户名/密码经
  `url.UserPassword` 百分号编码 —— 密码中的 `:@/?` 无法破坏 DSN 结构。
- **mysql**：`user:pass@tcp(host:port)/db`（go-sql-driver 格式）。
- **sqlite**：只读连接器 → `file:<path>?mode=ro`；可写连接器 → 原路径。

驱动本身仍由宿主程序注册（`sql_query` 不引入第三方/CGO 依赖）。

## 3. 使用方式（个人场景）

```bash
# 1) 笔记库（Obsidian/Logseq 式 markdown 目录）—— 默认只读、只许 *.md
aflare connector add my-notes --type notes --root ~/notes

# 2) 文档目录 —— 可写 + 扩展名白名单 + 单文件 1MB 上限
aflare connector add my-docs --type files --root ~/Documents \
  --writable --include '*.md' --include '*.txt' --max-bytes 1048576

# 3) 个人 SQLite 库（Calibre/浏览器历史/自建库）—— 默认只读（驱动层 mode=ro）
aflare connector add my-library --type sqlite --database ~/calibre/metadata.db

# 4) 查看
aflare connector list
aflare connector show my-notes

# 5) 需要凭据的数据库
aflare secrets set connectors my-pg
aflare connector add my-pg --type postgres --host db.internal \
  --database analytics --username readonly_user --credential-group connectors
```

```yaml
# workflow.yaml —— 文件里只有名字，没有任何凭据，也没有绝对路径
steps:
  - node: files_list
    params:
      connector: my-notes
      pattern: "**/*.md"
  - node: file_read
    params:
      connector: my-notes
      path: "journal/2026-08-24.md"
  - node: sql_query
    params:
      connector: my-library
      sql: "SELECT title FROM books WHERE author = $1"
      args: '["鲁迅"]'
```

内联 `driver/dsn` 与 `connector` 互斥（同时出现报错）。旧的内联写法保持
兼容，但文档推荐 connector 模式。file_read 的密钥脱敏（redact，默认开）
在 connector 模式同样生效 —— 个人数据的隐私优先。

## 4. 安全边界（已实现）

- 凭据只存 secrets store / 环境变量，spec 文件与 workflow 文件只有引用。
- 连接器默认只读；`--writable` 需显式声明，且（数据库）工作流仍需
  `read_only=false` 双重确认才能执行写语句。
- 文件连接器：root 遏制（绝对路径/穿越/symlink）、include 白名单、
  max_bytes 上限、敏感扩展名黑名单、dotfile 跳过全部生效。
- SQLite 只读连接器在 DSN 层强制 `mode=ro`（驱动层纵深防御）。
- 名称/端点/root/白名单字段全量校验（null 字节、端口范围、glob 语法、
  绝对路径、文件类型互斥字段）。
- 注册表加载时逐条校验，坏数据**报错拒绝**而非静默丢弃。
- 0600 原子写（tmp+rename，防 symlink 替换攻击）。

## 5. Roadmap（个人优先）

| 阶段 | 内容 | 状态 |
|---|---|---|
| PR #95 | 数据库连接器骨架：Spec/Registry/Resolver/DSN + sql_query 接入 + CLI | ✅ 已合并 |
| 本 PR | **文件/笔记/个人库**：files/notes 类型 + file_read/file_write/files_list 接入 + sqlite mode=ro | ✅ |
| 下一步（个人） | 笔记搜索：frontmatter/tags/全文检索节点（notes 专属，走 connector root）；连接器级审计事件 | 计划 |
| 之后（个人→通用） | HTTP/API 连接器（http_request 接 `connector`，Bearer/Basic 注入）——个人云盘/API 与企业内网 API 复用 | 计划 |
| 最后（企业） | 企业 profile：Vault/SSO Resolver、内网 allowlist 与策略引擎联动、按连接器名的连接池缓存 | 计划 |

## 6. 与项目定位的关系

模板/内容生态已移除（PR #94）。aflare 的核心价值收敛为：**无论云端 AI
还是用户自部署的模型，接入 aflare 后，用户把自己的数据源通过 Connector
接入，由 aflare 让 AI 确定且安全地运行。**

目标用户先个人：个人用户「开箱即用」的数据就是本机文件、笔记库、
SQLite 个人库 —— 本 PR 让这三类零凭据、零配置地接入同一个安全模型。
企业内网系统走同一套 Spec/Resolver/天花板抽象，后续按 Roadmap 落地。
