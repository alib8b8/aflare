# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **README 中英文更新为「个人优先」定位**：头部定位改为「AI 与你的数据之间确定且安全的控制层」，新增 Connector API 优势条目、功能矩阵行、核心能力专节（五类连接器 / 凭据隔离 / 权限天花板 / 根目录遏制示例）与路线图 main 行，文档索引补 connector-api.md 链接
- **Connector API（命名数据源连接）**：新增 `internal/connector` 包与 `aflare connector add / list / show / remove` 命令——postgres / mysql / sqlite / files / notes 五类数据源注册为命名连接器（配置存 `~/.aflare/config/connectors.yaml`，`AFLARE_CONNECTORS_FILE` 可覆盖），`sql_query` / `file_read` / `file_write` / `files_list` 四节点新增 `connector` 参数，工作流引用连接器名而非内联 DSN 或路径。安全设计：凭据只存引用（加密 secrets store 或环境变量），spec 与 workflow YAML 永不落明文；只读默认（写需连接器显式开启），节点参数只能收紧不能放宽；天花板三件套 `max_rows`（默认 1000）/ `timeout`（默认 30s）/ `max_bytes`（默认 10MB，单文件读取上限）；postgres DSN 经 `url.UserPassword` 构造，密码特殊字符无法改变 DSN 结构；sqlite 拒绝 `file:` 前缀与 `?#` 参数并强制 `mode=ro`；文件连接器以 root 为围栏——`EvalSymlinks` 全链解析、最终组件拒绝符号链接（含悬空，防追加跟走与写入替换）；registry 原子保存（tmp+rename，文件 0600 / 目录 0700，防 tmp 被符号链接栽赃）（#95、#96、#97）
- **全源码隐形水印与 CI 门禁**：所有 Go 源文件版权行内嵌零宽字符水印（U+200B / U+200C 表比特、U+200D 分片标记），新增 `aflare watermark encode-source --all` / `check-source` / `decode-source` / `strip-source` 命令；CI 增加覆盖门禁——新增源文件无水印直接红叉。与部署水印（payload v2）互补：部署水印标记"哪次部署"，源码水印标记产物来自哪个源码树，用于泄露溯源

### Changed
- **CodeQL 74 条告警清零**：告警均属 taint tracking 无法建模 aflare 守卫（validateReadPath / validateURL / 命令白名单 / SQLite 标识符转义等）的误报——排除走规则级 query-filters（`.github/codeql-config.yml` 逐条注明理由与对应守卫），不采用行内 `// codeql` 注释（GitHub code scanning 不识别）与路径级过滤（对 Go 不生效，实测 excluded queries 仍会产生告警）；排除 7 条规则后其余全部保持激活，gosec 与 golangci-lint 门禁不受影响
- **gocritic 存量违例清零并启用 linter**（#57、#91）
- **workflow 上帝文件拆分**：generator 拆为 generator_llm / generator_parse / generator_validate 等 topical 模块（#84，13 文件，+3022 / -2782），拆分产生的新文件同步补嵌源码水印
- **CLI os.Exit 收敛为单点 dispatch**，internal/cli 测试覆盖率 21% → 61%

### Removed
- **内置模板库与内容生态（破坏性变更，#94）**：删除 17 分类 330 个内置技能模板与 `templates/` 目录（-1041 文件 / -71375 行），移除 `internal/{templates,skills,marketplace,badge,packs,agentplugins}` 包及 CLI 命令 `template` / `skills` / `marketplace` / `badge` / `install-pack`；agent 工具 `template_list` / `template_info` 与 MCP 模板工具同步移除。理由：330 个模板零真实使用信号，marketplace / badge 无人需要，维护成本无回报；项目定位收敛为"引擎 + 连接器 + 安全"，内容用户自带。保留：`run` / `create_workflow`、`template_render` 节点（渲染用户自带模板）、`examples/` 示例、用户本地模板目录扫描。迁移：引用内置模板的工作流改为本地模板路径；使用已删命令的脚本需移除

### Fixed
- **`aflare connector` 子命令完全不可达（断点，本轮自检发现）**：`HandleConnector` 已实现且 main.go 分发器有 `case "connector"`，但 `cli.knownCommands` 漏列 `"connector"`——`ValidateCommand` 在分发前直接以 unknown command 拒绝，`aflare connector add/list/show/remove` 一律不可用。现补入命令表，并在主 usage（PrintUsage，中/英/俄三语 locale）加 connector 一行

## [0.10.0] - 2026-08-22

### Added
- **国内合规通知渠道（飞书/钉钉/企业微信）**：`notify` 节点新增 `feishu` / `dingtalk` / `wecom` 三个 channel——对应三平台**官方群机器人 webhook**（个人微信与 QQ 无官方机器人推送接口，微信生态以企业微信群机器人为唯一合规路径），共享一个群机器人传输层：HTTPS+SSRF URL 校验、100KB 请求体上限、1MB 响应读取上限，且解析响应体中的错误码（三平台 token 无效时仍返回 HTTP 200，`errcode`/`code`/`StatusCode` 非零、响应非 JSON、HTTP ≥400 均判为失败，杜绝静默假成功）。`aflare create` 识别描述中的"飞书/钉钉/微信/企业微信"（含 feishu/lark/dingtalk/wecom 英文词）生成对应渠道步骤与 `<channel>_webhook_url` 变量；中文 README 股价监控示例与三层模型图同步由 Telegram 改为飞书，并在金融合规章节新增通知渠道表（QQ/个人微信无官方接口的诚实说明）
- **Agent Plugins 1.0.0 宿主支持（双向生态互通）**：新增 `internal/agentplugins` 包与 `aflare marketplace install <plugin-dir>`——加载任意符合 Agent Plugins 1.0 开放标准（OpenAI/Google/AWS/Microsoft/Cursor/Vercel 联合支持）的插件目录：`skills/*/SKILL.md` 解析 frontmatter 后物化为可运行的 aflare 技能（SKILL.md 指令嵌入单步 openai 包装 workflow 的 system prompt），`mcp.json` 声明的 stdio server 幂等注册进 `.mcp.json`（不覆盖用户已有配置）。安全规则对齐规范：manifest 双布局（`.plugin/plugin.json` 优先、根 `plugin.json` 回退）、技能目录名与插件名拒绝路径穿越、非 stdio transport 跳过、`cwd` 必须为插件根内 `./` 相对路径、`${PLUGIN_ROOT}` 展开。配合既有 `marketplace export` 实现 export → install 生态往返（已实测：aflare 导出的插件装回 aflare 直接可 `aflare run plugin/<plugin>-<skill>`）
- **MemHarness 记忆批判-重构模式**（arXiv:2607.28272 思想）：① memory 节点新增 `harness_search` 操作——检索候选记忆时携带完整来源状态（type/level/tags/source/confidence/created_at/score），并生成自包含的批判 prompt（逐条 keep/rewrite/discard + 无适用记忆输出 `<EMPTY>`），LLM 批判作为显式可重试的工作流步骤执行而非隐藏调用，默认跨层级检索；② agent 会话记忆注入加确定性批判：超过 30 天且从未被复用的记忆直接丢弃，幸存记忆带来源状态标注（记录日期/类别）注入并指示模型先判断适用性再使用——"记忆是重构的线索，不是当前任务的事实"
- **步骤级类型化输出契约（NOOA 思想吸收）**：WorkflowStep 新增 `output_schema`（JSON Schema draft-07 子集，复用 structured_output 校验器）——任意节点的输出在每次尝试后强制校验，违规按步骤失败处理并报出首个违规的 JSON pointer 位置，自然流入既有 retry/backoff/on_error/capture_error 机制；LLM 输出的 ```json 围栏自动容忍
- **有界预览输入（NOOA pass-by-reference 思想吸收）**：WorkflowStep 新增 `preview_input: true`——超过 16KiB 的输入替换为"类型+总长+头尾样本+省略字节数"的有界预览（UTF-8 与行边界安全），完整值保留在工作流状态、原样传给所有其他步骤；LLM 步骤看样本、确定性节点操作完整数据，长上下文工作流的 prompt 成本可控
- **MemHarness 模式示例**：`examples/real-world/memharness-critique/`——harness_search → LLM 批判（output_schema 契约 + preview_input）→ 行动 三阶段完整可运行示例
- **水印部署溯源（payload v2）**：水印内容哈希 8→6 字节，腾出 2 字节嵌入部署 ID（`AFLARE_DEPLOYMENT_ID`，1-4 位十六进制）。泄漏的内容现可直接定位到生成它的部署实例，无需再对照审计日志逐时间点排查；payload 总长保持 21 字节，分片/校验逻辑不变；v1 水印（8 字节哈希、无部署 ID）仍可解码，`aflare watermark decode/verify` 输出部署 ID
- **A 股股价监控生成器**：`aflare create` 识别 A 股 6 位代码（6xx→沪 sh、0xx/3xx→深 sz，或 sh/sz 显式前缀），经腾讯行情接口（`web.ifzq.gtimg.cn`）生成取价工作流——qt 快照现价为十进制字符串，`gt/lt` 数值比较直接可用；含加密货币提示词（BTC/比特币等）时保持 CoinGecko 路由不误判
- **港股 / 美股股价监控生成器**：港股 `hk00700`（3-5 位自动补零至 5 位，或裸 5 位前导零代码）、美股 `usAAPL` / `US:AAPL` / `US AAPL`（代码须大写 2-6 字母，API 小写返回空 qt；常见非代码词黑名单防误报）；英文调度短语 `every N minutes/hours` 同步支持；美股识别使用原始大小写描述（小写化会破坏代码）
- **A 股监控模板**：`templates/finance/stock-alert/`（workflow.yaml + skill.json + README），`aflare install finance/stock-alert` 一键安装，symbol/threshold 变量化
- **README 中英文市场分线**：中文版走合规路线——A 股示例 + 金融场景与合规说明章节（数据来源、量化/复盘/投顾定位边界表、免责声明，不出现 BTC/美股/港股）；英文版新增 Market Data & Financial Use Cases 章节（BTC/美股/港股/A 股四市场速查表 + 妙想 API 接入说明 + 定位边界 + Disclaimer）；含东方财富妙想大模型调研结论（闭源模型，官方 API 平台可选接入）
- **codex_agent 节点**：新增 `internal/nodes/codex.go`——工作流步骤委托 `codex exec` 执行（prompt 为单一 argv 元素、绝不经过 shell，model 值经白名单校验阻断 argv 级把戏，cwd 规范化后经路径校验）；配套 ReAct agent 会话上下文预算压缩与 pause-resume 状态持久化（CLI 展示 resume 提示）（#78）

### Changed
- `mcp.ServerEntry` 新增可选 `cwd` 字段（Agent Plugins 1.0 传递插件相对 cwd；主流 MCP 客户端同一 schema，向后兼容）
- **GitCode 镜像加 CI 门禁**：sync-gitcode.yml 由"push 到 main 即镜像"改为 workflow_run（CI 成功后触发）+ 推送前经 Actions API 复核 main HEAD 的 CI 运行确为 success——CI 红色或未完成的构建永远不会到达 GitCode；手动/定时触发同样受门禁约束（不绿即跳过并留 notice）。为查询运行状态新增 `actions: read` 权限
- **PR Review 的 golangci-lint 改为阻断性**：pr-review.yml 此前 `continue-on-error: true`——lint 失败照样绿灯，PR 门禁形同虚设（与 ci.yml 主干行为及 docs/code-review.md 声明的 Blocking: Yes 相悖）。现移除，lint 失败即红叉
- **govulncheck 改为阻断性（ci.yml + pr-review.yml + supply-chain.yml）**：此前三处均 `continue-on-error: true`，与 docs/code-review.md 声明的 "Vulnerability scan — Blocking: Yes" 相悖——PR gate job 名义上检查 security-scan 结果，但 job 级 continue-on-error 使其恒为 success，可达漏洞从不阻断。现移除 job 级与步骤级 continue-on-error：govulncheck 为符号级可达性分析，报出的都是依赖链中真实可达的漏洞，必须阻断；gosec 保持告警（误报多，security-auto-fix.yml 独立建 issue 跟踪）
- **AGENTS.md 修正为真实工具链与提交规则**：原文件写的是 `npm test` / `npm run lint`（本项目为纯 Go 仓库，无 package.json，命令无效）。现写明 Go 1.25 工具链、golangci-lint v2.12.2 版本对齐要求、提交前本地 CI gate 命令集、GitHub/GitCode 提交策略（PR + CI 绿 + code review checklist）

### Security
- **CI 工具链升级 go1.25.12 → go1.25.13（修复 7 个可达标准库漏洞）**：govulncheck 改为阻断性后立刻暴露 CI 长期掩盖的真实问题——CI 钉死的 go1.25.12 存在 7 个符号级可达的标准库漏洞（encoding/xml、encoding/asn1、net/http 等，全部在 go1.25.13 修复），此前 `continue-on-error` 使其从未被看见。全部 workflow（ci/pr-review/supply-chain/security-auto-fix/auto-fix/release/benchmark）与 mise.toml 统一升至 1.25.13；本地工具链经 go.mod 的 toolchain 指令本就用 1.25.13，故本地扫描为 0 漏洞
- **插件技能 frontmatter name 路径穿越（高危）**：SKILL.md frontmatter 的 `name` 字段此前未做与目录名相同的段校验——恶意插件可用合规目录名 + 穿越 frontmatter name 把 skill.json/workflow.yaml/SKILL.md 写到技能根目录外任意可写路径（含持久化 prompt 注入）。现 LoadSkills 拒绝非安全段的 frontmatter name，InstallPlugin 另加最终路径必须在技能根内的纵深防御检查（安全自检发现）
- **插件 MCP cwd 符号链接绕过（中危）**：cwd 的字符串前缀包含检查可被插件目录内的符号链接绕过（`./data` 链接到插件外）。现对存在路径追加 `EvalSymlinks` 双侧解析复核（安全自检发现）
- **记忆值注入围栏（中危）**：持久化记忆值此前原文拼回 prompt，构成跨会话 prompt 注入持久化向量。现新增 `memory.FenceValue`（换行/制表折叠 + 反引号中和 + 围栏包裹），PreProcess 与 harness_search 批判 prompt 的注入路径全部收口
- **记忆 AccessCount 数据竞争（中危）**：`PersistentMemoryStore.Retrieve` 与 `MemoryCapability.PreProcess` 此前在读锁下自增共享条目的 AccessCount，并发调用构成 data race（`go test -race` 可复现）。现分别改为写锁与读写锁分段（安全自检发现）

### Fixed
- **slack 通知步骤生成后必然运行失败（遗留 bug，本轮顺手修复）**：`aflare create` 对含 slack 的描述生成的 notify 步骤只有 `channel: slack` 而没有 `url` 参数——而 slack 渠道运行时强制要求 url，生成的工作流一跑就报 "url parameter is required"。现所有 webhook 型渠道（slack/feishu/dingtalk/wecom/webhook）统一生成 `<channel>_webhook_url` 变量引用
- **stock-alert 模板字段契约修复（代码审查发现）**：模板初版使用了 Workflow/WorkflowStep 不存在的字段——`input:` 块（默认值被 yaml.Unmarshal 静默丢弃，symbol/threshold 从未有默认值）、步骤级 `id:`/`input:`（同样被丢弃）、`timeout: 30s`（http_request 节点要求整型秒数）、以及 `price: "{{ .input }}"` 参数（向 Go 模板传递字面量字符串 `{{ .input }}`，告警消息渲染为未解析表达式）。现改用 `vars:` 默认值 + `name:` 步骤名 + `timeout: "30"` + 模板内直接 `{{ .input }}`；新增 `embed_templates_test.go` 契约测试锁定字段正确性并离线验证告警渲染链路
- **带 vars 默认值的模板被参数提示阻断（断点E）**：`aflare run` 对无 input_schema 但引用 `{{var.*}}` 的工作流会强制弹出参数提示退出——即使变量已在工作流自身 `vars:` 中声明默认值。现已在提示前过滤掉有默认值的变量，带默认值的模板（如 finance/stock-alert）开箱即跑，`--set` 仍可覆盖
- **无标的的价格查询静默生成比特币监控（安全自检发现）**：price 关键词命中但既无股票代码也无加密货币提示时（如 "check gold price" / "监控黄金价格"），生成器此前无条件落入 CoinGecko 分支生成比特币监控工作流——错误的市场、错误的标的。现 CoinGecko 路由仅在实际提及加密资产时触发，其余无标的价格查询不生成任何取价步骤（新增回归测试）
- **工作流审计在全新安装上被静默跳过**：工作流执行记录器仍按 0.8.x 逻辑要求 `AFLARE_AUDIT_HMAC_KEY` / `AFLARE_SECRETS_PASSWORD` 环境变量才写审计，而 0.9.0 的审计链已支持自动生成每安装随机密钥文件——两条路径不一致导致 `aflare run` 在未设环境变量的新安装上一条工作流审计都不写，且打印误导性警告。现移除该过时门控，密钥解析完全由 history 包负责（混沌测试实测发现）
- **MemHarness 批判对损坏时间戳失效**：记忆 Timestamp 为空或不可解析时此前按"新鲜"处理（永不进入丢弃通道）。现按"陈旧"处理——损坏/被篡改的记录不再是批判盲区；`InstallPlugin` 不再忽略 `filepath.Abs` 错误；物化技能文件权限 0644 → 0640
- **文档与数据口径修正**：skills-registry.json 的 `count` 字段在删除 3 条注册后未同步（仍为 333，实际 330，现修正为 330）；README 中英文及系统提示词/工具描述中的"16 个领域/16 domains"统一修正为实际值 17 个领域（供应链场景包加入后未同步）
- **WorkflowCapability 模板缓存恒为空（CI 阻断性覆盖率门禁暴露）**：`parseTemplateMeta` 把整个 workflow 文档 unmarshal 进 `TemplateMeta{Steps []string}`，而真实模板的 steps 是 map 列表（`node:`/`id:` 键），类型必然不匹配报错返回 nil——`w.templates` 缓存永远为空，PreProcess 提示词恒报 "0 templates available"，extractTemplateName 的最长匹配分支永不命中（无任何测试覆盖此函数，故从未被发现）。现元数据与 steps 分离解码（steps 标签优先取 id、回退 node/name），并用真实模板格式补齐测试；internal/agent 覆盖率 60.1% → 63.5%（删除 adaptive 能力及其测试后曾贴到 arm64 60% 阈值之下，导致 main CI 红叉）

### Removed
- **`internal/protocol` 死代码包**：intent 协议（DID 身份认证、跨域消息路由）共 605 行代码无任何调用方——宣称的跨域身份能力从未接线到 CLI / Agent / Runtime，属于纯宣称功能。整包删除（含测试），对二进制行为零影响
- **`adaptive` 能力（伪学习）**：`AdaptiveCapability` 仅将反馈文本追加到 learning.json，`PreProcess` 只注入一段固定提示词，不存在任何实际学习/自适应决策逻辑——文档却宣称"从反馈中学习、跨轮次改进"。删除该能力及全部接线：`CreateCapability`/`AvailableCapabilities` 注册、`smart` 预设（现为 reflection + memory）、chat/agent/serve/webui CLI 帮助文本、学习日志 Feedback 字段与 adaptive 分组统计、相关测试用例；`ParseCapabilities("all")` 由 7 项变为 6 项
- **学习日志 pattern 识别死代码**：`recognizePatterns`/`LearningPattern`/`normalizeIssue`、learningStore 的 pattern 缓存字段及 `LearningEntry` 从未被写入的 `Output` 字段均无生产调用方（仅测试自测），文件头注释却把 pattern 识别列为"关键特性"。一并删除，学习存储收敛为生产实际使用的追加/去重/压缩/读取链路（并发与竞争测试保留）
- **3 个昇腾空壳模板**：`software-engineering/end-to-end-adapt`、`performance-tune`、`quick-adapt`——模板中的 `code_interpreter` 节点返回硬编码占位 JSON（`{"status":"not_available"}`），依赖的 `ascend_*` 执行节点不存在，装出来即失败。删除模板目录与 skills-registry.json 注册条目（注册表实际 333 → 330；README 此前宣称的 332 本身即为过时口径）；serve/webui 帮助文本顺带修正——删除 `bdi` / `multi-agent` / `simulation` 等 `CreateCapability` 从不认识的伪能力名（传入会被静默丢弃），对齐真实 6 能力

## [0.9.0] - 2026-08-16

本版本主题：**国密算法支持、审计链安全硬化、升级兼容保证、MCP server 一键安装**。默认路径与 0.8.x 字节级兼容，滚动升级可共享 home 目录。

### Added
- **国密算法支持（opt-in）**：审计链 HMAC 可选 SM3（`AFLARE_AUDIT_MAC_ALGO=sm3`），secrets 静态加密可选 SM4（`AFLARE_SECRETS_CIPHER=sm4`）；默认仍为 SHA-256 / AES-GCM，两者均不改变默认 on-disk 格式
- **审计 HMAC 密钥管理**：新链首次写入自动生成每安装随机密钥文件（`audit-hmac.key`，0600）；支持 `AFLARE_AUDIT_HMAC_KEY` 环境密钥与 `AFLARE_SECRETS_PASSWORD` 密码派生密钥（PBKDF2）；验证时按多密钥候选重试，密钥轮换后历史记录仍可验证
- **MCP server 一键安装**：`aflare mcp install <name>` / `aflare mcp list`，内置 8 个社区 server（fetch、filesystem、git、memory、sqlite、sequential-thinking、everything、time），写入项目级 `.mcp.json`（Claude Code / Cursor / opencode 同一 schema），幂等不覆盖本地修改；检测到 npx/uvx 缺失时安装即警告
- **跨进程审计日志锁**：审计追加使用 `O_CREATE|O_EXCL` 锁文件串行化多进程写入，带超时与陈锁回收，杜绝并发追加导致的链断裂
- **供应链场景包**：新增 18 个 supply-chain 模板（需求预测、库存补货、路线优化、清关、冷链监控等），内置模板 323 → 332
- **doctor 加密兼容性体检**：报告审计链算法混合情况与 secrets 静态加密套件（不解密），识别 SM3/SM4 数据时给出精确升降级步骤；检测审计链使用公开默认密钥并给出迁移指引
- **loong64 构建目标**：GoReleaser 矩阵新增 Linux loong64

### Security
- **审计默认 HMAC 密钥可伪造**：旧版使用源码内公开默认密钥，任何读过源码的人都能伪造审计记录。现新链自动生成随机密钥；已有旧链为保持可验证继续用原密钥（一次性警告 + doctor 迁移指引：导出归档 → 移走旧链 → 新链自动换随机密钥）
- **未过滤 bundle 截断伪造**：导出 bundle 声明"全量"时强制首条记录 `prev_hash` 为零哈希，删除前缀记录冒充完整链的伪造直接拒绝
- **secrets 临时文件符号链接攻击**：`SaveToFile` 写临时文件前移除已存在的符号链接/目录占位，防止原子替换路径被劫持；密钥文件与密钥库权限收紧
- **InspectFile 版本校验**：doctor 诊断 secrets 格式前先校验版本字节，不再把未来格式文件按字节误读误诊

### Fixed
- **滚动升级兼容（0.9.0 最关键修复）**：国密开发曾在默认路径引入两个静默断点——secrets 每次保存写版本化 header（0.8.x 无法读取）、审计记录携带 `mac_algo` 字段。现默认 AES-GCM 仍写 legacy headerless 格式、默认 SHA-256 记录不携带 `mac_algo`，与 0.8.x 输出字节一致；切回 aes-gcm 重存即恢复 legacy 格式（回滚路径）
- **Ollama 模型不存在 404**：透传 Ollama 错误信息并给出 `ollama pull <model>` 修复命令（模板用户首跑第一断点）
- **超大审计记录读取失败**：末 8KiB 解析失败时回退全文件扫描，区分超大记录（可恢复）与截断行（报错）；末行截断（写入崩溃的半行 JSON）给出"备份后截断到最后完整记录"的修复指引

### Changed
- 版本化 secrets header 与 `mac_algo` 字段仅在显式选择非默认算法时写入，并附一次性进程警告（提示 pre-0.9.0 校验方兼容性）

## [0.8.1] - 2026-08-15

本版本为**发布审计修复**：面向首批测试用户修复安装与入口断点、清零安全漏洞。

### Fixed
- **国内安装 404**：一行安装脚本此前从陈旧的 pre-rename GitCode 镜像（`llm-box/llm-box`，停留在 v0.5.0）取"最新版本号"，再到 GitHub 下载对应 tag——必然 404。移除该镜像源，GitHub 为唯一版本源，国内通过 ghproxy 加速
- **`aflare mcp` 子命令缺失**：文档记载的 MCP 入口实际是未知命令（此前仅 `--mcp-server` flag 可用），注册 `mcp` 子命令使两者等价
- **execute 白名单报错不明**：allowlist 拦截 shell 元字符时定位违规字符及位置、列出全部禁止字符、给出 `echo hello` vs `echo "hello"` 修正示例，CLI 附中文排查建议
- **pre-rename 残留清理**：README、docs、下载页、skills 中的 `llm-box/llm-box` 链接统一替换为规范 GitHub（或 ghproxy 加速）地址

### Security
- **govulncheck 可达漏洞清零**（10 → 0）：otel SDK/exporter 升级至 v1.45.0，Go 工具链钉至 go1.25.13

### Added
- internal/errors、internal/packs（覆盖率 100%）、internal/telemetry 首次补齐测试

## [0.8.0] - 2026-08-14

本版本主题：**离线 / 内网首选项体验、隐私安全硬化、CLI 体验优化**。面向内网 / 本地优先、对数据隐私与安全敏感的企业用户与个人。

### Added
- **离线 / 内网首选项体验**：air-gapped 离线安装（`install.sh` 支持本地归档）、`aflare doctor --offline` 离线环境自检、WebUI Mermaid 离线回退、323 模板内嵌进二进制首跑自动释放到 `~/aflare/templates`
- **本地 / 离线 LLM 丝滑接入**：Ollama / vLLM / LM Studio / 本地 DeepSeek / 任何 OpenAI 兼容 endpoint，loopback 地址（127.0.0.1 / localhost）免 API Key 接入；有本地 LLM 走意图理解 + 动态生成工作流（`--ai` / `chat`），无 LLM 关键词匹配兜底
- **隐私安全硬化**：面向本地 / 气隙部署的隐私强化，遥测可关闭，aflare 不回传用户数据
- **`template run <id>` 命令**：一键运行模板，无需 clone 或记 `workflow.yaml` 路径，复用 `--set` / `--params-file` / `--resume`，支持 fuzzy 短名匹配
- **未知命令智能提示**：`aflare node list` 不再静默回退到 usage，报错 + did-you-mean（前缀匹配 + 编辑距离 ≤2）
- **secrets 友好提示**：headless / CI / 容器无系统密钥环时，给出 `export AFLARE_SECRETS_PASSWORD='你的主密码'` 中文指引
- **create LLM 提示**：已配置 LLM 时提示 `--ai` / `chat` 更强生成路径
- **内置节点注册修复**：注册 14 个此前孤儿化的内置节点
- **会话持久化**：退出时自动保存会话，`/resume` 恢复历史对话，文件锁防止并发写入
- **上下文窗口指示**：prompt 显示 `[ctx: N%]`，80% 以上加 ⚠ 警告，压缩后显示 `[ctx: compressed]`
- **`/export` 命令**：导出对话为 markdown 文件，含时间戳和消息角色标注
- **多行输入**：`\` 续行 + 空行提交，进入多行模式时显示操作提示
- **场景包安装**：`aflare install-pack <name>` 支持 12 个场景包，`--list` 列出已安装，`--force` 重装
- **产品指标**：`first_session_success`、`session_turns`、`template_usage`、`capability_inits`，接入 Prometheus `/api/v1/metrics`
- **插件系统**：Go 插件 `.so` 加载，支持 `PluginManager` 注册/启用/依赖管理
- **插件文档**：`docs/plugins.md` 含平台说明、接口定义、示例代码、Troubleshooting
- **文件监听**：daemon 模式 `--watch` 轮询监听目录变化，自动喂给 agent
- **任务队列**：FIFO + 去重，任务状态追踪（pending → running → done/failed），并发控制
- **API 限流**：基于令牌桶，10 req/min per IP，支持 `X-Forwarded-For` / `X-Real-IP`，超限返回 429
- **代码质量**：`.golangci.yml` 14 个 linter（`funlen`/`errcheck`/`bodyclose`/`staticcheck`），pre-commit hook

### Fixed
- **template list/clone 不一致**：裸二进制 `template list` 显示模板但 `template clone` 提示"未找到模板"——`SkillMeta.Path` 在从 `skills-registry.json` 反序列化时丢失，改为根据 ID 重建 Path；并将 323 模板内嵌进二进制首跑自动释放
- **模板释放污染工作目录**：`ResolveTemplatesPath` 兜底返回用户主目录下 `~/aflare/templates`，避免在当前工作目录创建模板目录
- **虚构功能清理**：移除虚构的 `unitree_robot` 节点、`SandboxNode` / `AgentOrchestratorNode` 死代码、虚构的 unitree-patrol 市场包；修正 MCP 工具列表与 `custom-nodes.md` 文档
- **GitCode 镜像 URL**：修正为 `gitcode.com/llm-box/llm-box`（原 `aflare/aflare` 错误）
- **enforce-pr 误判**：修复对 squash merge 的误判与 fake PR ref 安全绕过漏洞（后因用户要求只留 main、不走 PR 流程，该 workflow 已整体移除）
- **Ollama 真流式**：字符级 `ollamaStreamFilter` 状态机，逐字符匹配 `"thought"` / `"final_answer"` 前缀，提取纯文本流式输出，抑制 JSON 结构噪声
- **Tool call 累积**：`CallWithToolsStream` 后续 chunk 携带 `ID`/`Name` 时正确更新（兼容分块发送 name+arguments 的 provider）
- **版本同步**：`version.go` 从 `0.6.0` 更新为 `0.7.0`，本次更新为 `0.8.0`
- **多行输入提示**：从 `... ` 改为 `... (empty line to submit, \ to continue) `
- **插件平台检查**：`LoadPlugin` 在 Windows 上返回友好错误提示，引导使用 MCP 替代方案

### Changed
- **项目介绍重构**：README「和别的工具有什么区别」对比表替换为「项目优势」章节，聚焦 aflare 自身价值，不与其他工具对比（中英文同步）
- **CI 提速**：benchmark 步骤（count=5 跑两遍 `./...`）改为只在 release tag / 手动触发时跑，普通 push main 跳过，CI 从近 40 分钟降至约 7 分钟；质量门（build/race test/coverage/lint/vulncheck）未削弱
- **分支清理**：删除所有已合并的残留源分支，仓库只留 main
- **Ollama 流式过滤重构**：`ReActStreamFilter` 从 `chat.go` 移至 `agent.go` 的 `callOllama`，删除冗余结构和 `json` import
- **排序优化**：`CallWithToolsStream` 的 tool call 索引排序从 O(n²) 插入排序改为 `sort.Ints`
- **死代码清理**：删除 `chat.go` 中无调用方的 `processInput` 方法
- **文件重命名**：`capability_stubs.go` → `capability_adaptive.go`
- **指标超时判定**：`first_session_success` 的 timeout 从会话级改为单轮级（`turnStart`），避免恢复会话误判

## [0.7.0] - 2026-08-07

### Added
- **Saga 事务补偿**：forward + compensation 步骤，失败时按反向执行 best-effort 补偿，幂等性保证
- **LLM 成本归因**：token usage × 模型单价表，自动计算 USD 成本写入审计日志 (`cost_usd`/`total_tokens`)
- **宇树机器人集成**：`unitree_robot` 节点，支持 Go2/B2/H1/G1 等 9 种机型、14 种动作，simulate/API 双模式
- **寒武纪 MLU 适配**：Cambricon provider 节点，支持 MLU 370/590
- **海光 DCU 适配**：Hygon provider 节点，支持 K100/Z100
- **ARM64 鲲鹏 CI**：CI 矩阵新增 `ubuntu-24.04-arm` 运行器
- **昇腾 AML 风控 + Saga 转账示例**：金融场景在国产芯片上的完整落地
- **多模态节点修复**：vision-LLM 路径正确发送图像数据，OCR 支持 tesseract + LLM 回退
- **HMAC 哈希链审计**：审计日志不可篡改
- **幂等性支持**：Idempotency-Key + 跨进程锁
- **HTTP 限流/重试**：内置 rate limiting 和自动重试
- **LLM 响应缓存**：减少重复调用成本
- **配额持久化 + 多租户**：配额跨进程持久化
- **Trace 脱敏**：JWT/私钥等敏感信息自动脱敏
- **WAL 崩溃恢复**：Write-Ahead Log 保证执行状态可恢复
- **结构化日志**：基于 `log/slog` 的 JSON/文本双格式日志
- **HTTP API 服务**：REST API 支持 workflow 执行和状态查询
- **项目更名为 aflare**：从 llm-box 正式更名为 aflare

### Changed
- 项目名从 llm-box 更名为 aflare
- 环境变量前缀从 `LLM_BOX_` 改为 `AFLARE_`
- Go 模块路径改为 `github.com/alib8b8/aflare`
- PR Review workflow 仅保留 pull_request 触发，移除 push 触发

### Fixed
- sql_query.go TOCTOU 竞态条件修复
- sql_query.go 错误处理缺失修复
- gofmt 格式对齐问题（多次）
- golangci-lint unused mutex + staticcheck S1017

## [0.6.0] - 2026-08-01

### Added
- **百灵生态集成**：多 Agent 协作框架
- **AI 网关 OmniRoute**：统一 LLM 路由，智能分发
- **Agent 记忆基础设施**：向量记忆、用户画像、分层记忆
- **语音 AI 工具链**：ASR 语音识别、音频分离、语音分析
- **Agent 团队化**：200+ 角色 + Agency 工作流编排
- **WAL 持久化**：Write-Ahead Log 保证执行可靠性
- **字节码 IR 表达式引擎**：高性能 workflow 表达式求值
- **EWMA / 帕累托路由**：智能负载均衡和路由
- **TLA+ 形式化验证**：DAG 执行正确性形式化证明
- **代码解释器**：内置 Python 沙箱执行
- **RAG 检索增强**：知识库 + 向量检索
- **知识图谱**：实体关系建模和推理
- **智能路由器**：多模型自动路由和 fallback
- **多模态节点**：图像分析、OCR、音频转录
- **节点市场**：100+ 可复用工作流模板
- **16 个领域专家**：金融、医疗、法律等垂直领域
- **思维链推理**：Chain-of-Thought 复杂推理支持

## [0.5.2] - 2026-07-28

### Added
- **代码图谱**：代码结构可视化分析
- **子代理提示词层级**：分层 Agent 提示词管理
- **熔断器**：LLM 调用自动熔断和降级
- **秘密脱敏**：敏感信息自动检测和脱敏
- **文件监听**：文件变更自动触发 workflow
- **TUI Markdown/Mermaid 渲染**：终端内图表渲染
- **LLM 路由统一**：3 套路由合并为 1 套统一路由

## [0.5.1] - 2026-07-25

### Added
- **昇腾 NPU 适配**：7-Agent 流水线、3 种工作流模板
- **CANN / MindIE 集成**：国产 AI 推理服务适配

## [0.5.0] - 2026-07-21

### Added
- **ReAct 引擎**：Reasoning + Acting 自主决策循环
- **分层记忆**：短期/长期/工作记忆三层架构
- **技能自进化**：Agent 自主学习和技能积累
- **鸿蒙适配**：7 种鸿蒙设备节点支持
- **跨平台协议**：`intent://` + `ohos://` URI scheme
- **W3C DID 身份**：去中心化身份认证
- **跨域 Agent 消息**：分布式 Agent 通信
- **GitCode G-Star + ohpm 生态**：开源生态集成

## [0.4.0] - 2026-07-14

### Added
- **代码解释器**：Python 沙箱执行节点
- **RAG 检索增强生成**：知识库 + 向量检索
- **知识图谱节点**：实体关系建模
- **智能路由器节点**：多模型自动路由
- **多模态节点**：图像分析、OCR、音频转录
- **节点市场**：100+ 可复用模板
- **16 个领域专家**：垂直领域优化
- **思维链推理**：Chain-of-Thought 支持

## [0.3.0] - 2026-07-04

### Added
- Multi-language support (9 languages: zh, en, ru, fr, ja, ko, es, ar, hi)
- Condition execution support for workflow steps
- Variable substitution (vars field) in workflows
- Atomic write operations for file_write node
- Workflow chaining via call node
- Dockerfile for containerized deployment
- Makefile for build automation
- GoReleaser configuration for cross-platform releases
- Homebrew tap support
- SHA256 checksum verification in install.sh
- Thread-safe Registry with mutex locks
- .gosec.json security scan configuration

### Changed
- Tightened directory permissions from 0755 to 0750
- Tightened file permissions from 0644 to 0600
- Ollama node now prioritizes prompt parameter over input
- notify node returns error for invalid channel instead of silent fallback

### Fixed
- SSRF protection (DNS resolution, IPv4-mapped IPv6 bypass, redirect validation)
- Path traversal protection (symlink resolution, dot-file rejection)
- Command injection protection (shell metacharacter blocking)
- template_render SSTI vulnerability - removed dangerous template functions
- Integer overflow in registry lowercase function
- Context leak in workflow executor (defer to immediate stepCancel)
- Keyword matching improved with word boundary checks and Chinese support
- gofmt formatting issues across 12 files

### Security
- Complete SSRF protection layer
- Path traversal protection for all file operations
- Command injection prevention for execute node
- Resource limits (file size, response body, retry/parallel/step counts)
- Recursive call depth tracking for workflow chaining
- Sensitive data filtering in audit logs
- External node API key protection

## [0.2.10] - 2026-06-16

### Added
- External node support with registry
- Node install/uninstall commands
- LLM node deduplication (llm_base.go)

### Fixed
- Various bug fixes and stability improvements

## [0.1.0] - 2026-06-02

### Added
- Initial release
- Core workflow engine with YAML-based step definition
- Built-in nodes: llm, execute, file_read, file_write, http_request, fetch_url
- Interactive TUI with bubbletea
- Workflow generation from natural language
- Ollama integration
- History tracking

[Unreleased]: https://github.com/alib8b8/aflare/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/alib8b8/aflare/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/alib8b8/aflare/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/alib8b8/aflare/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/alib8b8/aflare/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/alib8b8/aflare/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/alib8b8/aflare/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/alib8b8/aflare/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/alib8b8/aflare/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/alib8b8/aflare/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/alib8b8/aflare/compare/v0.2.10...v0.3.0
[0.2.10]: https://github.com/alib8b8/aflare/compare/v0.1.0...v0.2.10
[0.1.0]: https://github.com/alib8b8/aflare/releases/tag/v0.1.0
