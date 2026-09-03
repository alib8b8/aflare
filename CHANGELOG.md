# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Claude 供应商配置对齐 2026-09 API 现状（retired 模型清理 + 牌价表补全）**：Anthropic 已将 Claude 3 / 3.5 全系标 retired（仅 Bedrock / Google Cloud 保留），9 月 1 日发布 Fable 5.1 / Mythos 5.1。仓库三处配置仍钉在 retired 模型上，用户照默认配置调用直接失败：
  - **`anthropic` 节点默认模型**：`claude-3-5-sonnet-latest` → `claude-sonnet-5`（当前主流，$2/$10 per MTok——官方 8 月 10 日宣布 introductory 价转正，原定 9 月 1 日涨到 $3/$15 的涨价已取消）；注释同步补 Fable 5.1 / Mythos 5.1 的 `tool_choice` 注意事项（`any`/`tool` 返回 400，`auto`/`none` 不受影响，应改用 strict tool use 或 structured outputs）
  - **llm_router fallback 模型**：`defaultModelFor("anthropic")` 的 `claude-3-haiku-20240307`（2024 年 3 月的模型）→ `claude-haiku-4-5`（$1/$5，haiku 档位本就是 router 降级路径的正确定位——参照 openai 条目 fallback 用 gpt-4o-mini 的既有模式）
  - **llm_pricing 牌价表**：此前只有 claude-3 代条目——用户走代理调用任何 4.5/5 代模型（sonnet-5 / opus-5 / haiku-4.5 / fable-5）成本归因全按 $0 计，而 LLM 成本归因（cost_usd 写入审计日志、预算告警）是本项目核心卖点，对 Claude 用户完全失效。现按官方定价页补全现役代：fable-5 / mythos-5（$10/$50）、opus-5 与 opus-4-5~4-8（$5/$25）、sonnet-5（$2/$10）、sonnet-4-5/4-6（$3/$15）、haiku-4-5（$1/$5）；retired 代保留并标注（Bedrock/GCP 路由仍是这个价，`claude-opus-4` 前缀条目同时覆盖 4-1 变体）。最长前缀匹配保证日期后缀变体（如 `claude-sonnet-5-20260630`）落到正确档位且不跨代串价；回归钉子 `TestComputeLLMCost_ClaudeCurrentGeneration` 钉死各档牌价与前缀行为，`TestDefaultModelFor` 断言同步。docs/nodes-reference.md 重新生成，openrouter.md 示例的 retired `anthropic/claude-sonnet-4` 改为 `claude-sonnet-5`

- **pre-commit hook 补水印覆盖检查**：scripts/pre-commit 新增第 3 步——staged 含 .go 文件时跑 `aflare watermark check-source --all`（与 CI 门禁同口径，热缓存约 1s），缺水印的提交在本地即被拦下并给出修复命令（`encode-source --all`）；纯文档提交自动跳过。起因：d520092 新增的 10 个文件缺源码水印，本地门禁五件套不含此项，直到 push 后 CI 红叉才发现（699812e 补救）——现在本地 hook 与 CI 对齐，这类红叉不会再发生

- **崩溃恢复的最后一公里（DAG checkpoint / 状态写盘原子化 / scheduler misfire 标记）**：确定性引擎的自然卖点——崩溃后重启、跳过已完成节点、从断点续跑——此前只兑现了一半。四处收口：
  - **DAG checkpoint**：`executeWorkflowDAG` 此前直接报 "checkpoint/resume is not supported in DAG mode"（executor_executor.go），现在持久化已完成节点集合（StepOutputs，节点一旦完成在单次 run 内单调递增、天然好存）；重启恢复时已完成输出直接回填引擎、标记 cached，仅运行剩余子图。`WorkflowState` 新增 `dag_mode` / `step_names` 钉住快照对应的工作流形状——对步骤列表不一致的工作流 resume 直接拒绝（报错），而非把陈旧输出静默挂到错误的步骤上。WAL 仍是顺序模式专属（线性游标语义），两路径并存时 DAG 只读 JSON checkpoint 不双读
  - **状态写盘原子化**：新增 `internal/fsutil.WriteFileAtomic`（同目录临时文件 + fsync + rename + 目录 fsync）——`SaveState` / `saveCheckpoint` / `SaveSchedules` 三处此前都是裸 `os.WriteFile`，防崩溃的文件本身会被崩溃写坏（写一半的 JSON）。调度定义同批收口：非原子写崩溃后 daemon 会以零调度重启、所有任务静默停摆
  - **损坏文件保全**：checkpoint 解析失败（如崩溃截断的尾部）不再静默丢状态——`PreserveCorrupt` 把坏文件挪到 `<path>.corrupt-<unix-nano>` 后才报错起跑，用户最后一份可手工恢复的快照不会被"starting fresh"覆盖
  - **scheduler last-run 持久化 + missed 标记**：`aflare agent` 起 daemon 时记录每个任务的最近触发时刻（`lastrun_store`，dispatch 时写而非完成时写——崩溃 mid-task 的重启不会双触发）；启动时 `RestoreLastRun` 数出停机期间错过的触发次数（封顶 1000，分钟级任务停机数周不至迭代百万槽位），`ListTasks` 暴露 `LastRun`/`MissedRuns` 并告警日志。只标记不补跑：语义诚实——不假装停机没发生过，也不用陈旧积压淹没 agent
- **运维可观测性：节点级 Prometheus 指标 + daemon opt-in ops 端点**：原 4 个指标全是产品分析类（session/template/capability/provider），daemon 用户回答不了"我的工作流为什么慢/卡"。三块补齐：
  - **新指标**：`aflare_node_failures_total`（node_name × error_class：timeout/canceled/not_found/other）、`aflare_runs_active`（在途 run 数，幂等缓存命中不计）、`aflare_queue_depth`（daemon 任务队列待取数，Enqueue/Dequeue 实时更新）；`aflare_llm_tokens_total` 已有，不重复造
  - **指标接线修复**：workflow executor 的顺序 / DAG 两条路径都是直接 dispatch `node.Execute`（不走 `Registry.ExecuteWithStats`），节点时延/失败序列对 workflow run 全盲——现在两条路径在步骤完成处调用 `RecordNodeExecution`，且用 raw pre-recovery error（被 fallback / capture_error 掩盖的节点失败对运维仍是失败；condition 跳过与 eval 失败的步骤从未派发节点、不计数）
  - **daemon ops 端点**：`aflare agent --ops-port 9090`（默认 9090）+ `AFLARE_METRICS=1` / `AFLARE_PPROF=1` 双开关 opt-in——默认一个端口都不开（local-first 口径：不主动开端口，开的人自己负责）；默认绑 127.0.0.1（`AFLARE_OPS_ADDR` 可覆盖），无认证无限流（与 WebUI 端点不同，trusted networks only）；`--debug` 之外的 pprof 首次对 daemon 可用
- **`aflare audit tail`（审计日志实时订阅 + JSONL/SIEM 输出）**：HMAC 链与 bundle 此前的输出形态只有文件，企业尽调要的实时订阅是纯 I/O 包装——引擎一行未动，完整性校验仍在 `aflare audit verify`。默认打印最近 10 条后跟随新追加记录（tail -f，500ms 轮询——append-only 本地日志上轮询比 fsnotify 便宜且零新依赖）；`--json` 原样透传磁盘上的 JSONL 字节（SIEM forwarder 摄入的与哈希链覆盖的字节一致）；`-n 0` 纯跟随不吐历史；文件变小（截断/轮转）从头重读并 stderr 告警——对哈希链日志这本身就是篡改信号；后向分块扫描定位末尾 N 条不整文件加载，写一半的尾行等完整了才发出
- **CI 防线三件套（examples 全量校验 / nightly fuzz / bench 回归门禁）**：
  - **examples 全量校验门禁**（ci.yml）：仓库 40 个 yaml 样例此前只有 1 个（content-processor.yaml）被 action-test.yml 触碰，其余 39 个是零验证的死资产。新门禁对每个含顶层 `steps:` 的 yaml 跑 `aflare validate`（数据文件如 scenes.yaml 路由表自动跳过），坏样例立红。上线即抓到 drone-patrol 的不可运行语法（见 Fixed）
  - **nightly fuzz**（soak.yml）：10 个 fuzz 目标（workflow 解析器、表达式引擎 ×2、MCP 请求处理、policy 加载与路径校验、节点输入 ×4）此前只在 PR CI 跑种子语料，种子之外的 bug 永远不炸。夜间每目标 time-boxed 60s；目标从源码枚举，新增 fuzz 函数自动纳入。FuzzExecuteCommand 以 dry_run=true 运行，无真实命令执行
  - **bench 回归门禁**（benchmark.yml）：每周 benchstat 对比从纯报告（continue-on-error）升级为 blocking——任何显著（p 值通过）的 >15% 变慢/内存增长置红；阈值取 15% 以滤除共享 runner 噪声，与该 workflow 头注释既有口径一致

- **版权来源治理四件套（商业授权前置条件）**：
  - `.claude/skills/setup/SKILL.md` 整体重写——原文件由外部贡献者 webbrain-one 撰写（c62e13f，111 行，早于贡献条款生效），重写版基于本仓库源码自行核实的事实（CLI 命令面、安装脚本、工作流布局）全新撰写，第三方表达清零；同批的 `.claude/skills/llm-box/SKILL.md`（122 行）此前已在改名提交中删除，无残留
  - `PROVENANCE.md` 版权与来源声明（草稿，未签名）——身份声明表（10 个历史 git 身份的归属与确认状态）、第三方贡献披露（webbrain-one 处置记录）、依赖许可声明、签名前必答的 HKAIC/llm-box 雇佣关系问题清单
  - `.mailmap` 身份显示统一——确定项：两个 alib8b8 邮箱拼写归一到 ID 前缀形式；Dev/HKAIC User/llm-box dev/Security Audit Bot 等匿名身份以注释形式待所有者在 PROVENANCE.md 确认后启用（不做历史重写——806 个提交、公开仓库、GitCode 镜像与 release tag 经不起 force-push）
  - `CLA-INDIVIDUAL.md` / `CLA-CORPORATE.md`（中英对照）——与 CONTRIBUTING.md 贡献条款逐条一致的许可式 CLA（非版权转让），个人/企业两版，供签名流程（如 cla-assistant）使用
- **所有者 git 身份落定**：今后提交统一使用 GitHub 已验证登记的 Gmail 身份（`sjxj19921205@gmail.com`；历史 0 提交，2026-09 起启用，避免再新增匿名身份）；`.mailmap` 规范身份改为该邮箱并映射两个历史 noreply 拼写（仅影响本地 `git log` 显示，不重写历史，GitHub 网页端显示不受影响）；PROVENANCE.md §2 身份表新增该条目（历史提交身份不变，仍待所有者逐项确认）。双邮箱分工显式化（PROVENANCE.md 新 §5）：Gmail 为开发者身份邮箱（git 提交、签署人标识），`local_first_agent@126.com` 为 aflare 商业合作邮箱（双许可询价、企业 CLA 签署件、合同往来）——两邮箱同归所有者，尽调读者可据此对应代码作者与商业主体；签署节顺延为 §6，签署人标识补全为 `alib8b8 <sjxj19921205@gmail.com>`（签署提交的作者邮箱可与身份表互验）
- **`docs/licensing.md` 用户视角双许可指南**：「什么场景用哪个协议」决策表、AGPL §13 网络条款与传染性解析、Go 静态链接为何堵死弱链接口子（import aflare = 派生作品，无灰色地带）、FAQ
- **双许可模式落地（AGPL v3.0 社区版 + 商业授权版）**：新增 `LICENSE-COMMERCIAL.md` 商业授权说明（社区版/商业版对照、适用场景判定、FAQ、中文摘要；联系入口 local_first_agent@126.com）；README（中英）License 章节改双许可声明、badge 更新为 `AGPL v3 | Commercial`。贡献条款同步收紧（堵住双许可的法律口子）：CONTRIBUTING.md License 章节重写——贡献按 AGPL v3.0 出站许可 + 授予项目所有者平行商业再许可权（版权保留，非 CLA 转让；第三方代码仅接受 MIT/Apache-2.0/BSD 等宽松许可）；PR 模板新增授权确认勾选项。依赖经核查全部为宽松许可（无 AGPL/GPL 传染），商业版二进制分发无第三方开源义务。此前 CHANGELOG 中「仓库 100% 提交来自所有者」的表述基于浅克隆误判，现更正：全量历史含 1 位外部贡献者（webbrain-one）与多个匿名身份，处置见本轮 PROVENANCE.md / SKILL.md 重写条目
- **LLM 供应商目录扩容：新增 13 家 OpenAI 兼容供应商（21 → 34 家 LLM 节点，85 registered nodes）**：全部走既有的 `core.OpenAICompatibleNode` 配置表驱动路径（openai_compatible.go 追加条目即注册，零新文件、零协议代码）。国内 5 家——火山方舟 Ark（`ark`，doubao-seed-2-1-pro，ARK_API_KEY）、硅基流动（`siliconflow`，Qwen/Qwen3-32B）、百度千帆（`qianfan`，ernie-4.5-turbo-128k，v2 端点原生 OpenAI 兼容）、腾讯混元（`hunyuan`，hunyuan-pro）、阶跃星辰（`stepfun`，step-2-16k）；国际 8 家——OpenRouter（`openrouter`，openrouter/auto，一家端点聚合全厂商，此前只能借 openai 节点 `endpoint` 参数曲线接入）、xAI Grok（`xai`，grok-4）、Groq、Cerebras、Perplexity（`sonar`）、Together、Fireworks、NVIDIA NIM。端点/环境变量名/默认模型逐一对照各厂商官方 OpenAI 兼容文档核实（Ark / 混元 / 硅基流动 / 千帆经官方文档检索确认）。配套接线：llm_router 的 `defaultModelFor` / `detectAvailableProviders` 候选名单与 `core.DefaultEndpointFor` 同步补全（新供应商设了环境变量即被路由自动检测与 fallback）；llm_pricing 增补有官方牌价的模型（grok-4 / sonar 系 / llama-3.3-70b-versatile / Llama-3.3-70B-Instruct-Turbo，无把握牌价的模型宁缺勿假——unknown 模型成本按 0 计不虚报）；README（中英）"20+" → "30+"、skills 镜像 LLM 节点数同步。回归钉子：`TestNewProviderContracts` 钉死 13 家的 name/env-var/endpoint/default-model 四元契约（环境变量名是公开 API——用户 shell 与 CI secrets 里写的就是它，静默改名即断部署；且必须符合路由器 `UPPER(name)+"_API_KEY"` 推导约定，偏离即破坏路由自动检测），`allProviderNames` 注册表名单 21 → 34，`defaultModelFor` / `DefaultEndpointFor` 表驱动测试补 13 家用例。docs/nodes-reference.md 重新生成（72 → 85 nodes）

### Fixed

- **examples/drone/drone-patrol/workflow.yaml 不可运行语法修复**：三个阶段（arm_and_takeoff / patrol_mission / land_and_disarm）使用了引擎不支持的裸嵌套 `steps:` 分组（无 `node:` 字段的子步骤既不是 parallel/loop/map 等复合类型，也不是可执行步骤）——该 example 自加入起从未通过 `aflare validate`，更不可能 `aflare run`。拍平为受支持的顶层 `depends_on` 链（arm → takeoff → upload_waypoints → start_patrol → … → land → disarm），报告模板引用同步改名（`{{arm_and_takeoff.takeoff.success}}` → `{{takeoff.success}}` 等）。由新加的 examples 全量校验门禁抓出（见 Added「CI 防线三件套」）
- **身份治理链收口：gmail 提交身份从显示层落到提交层**：#148 提交信息自称「本提交即首个使用 gmail 身份的提交」，但 GitHub API 原始数据显示网页端 squash 合并的作者邮箱全部是 noreply（邮箱隐私设置开启时，服务器端合并一律改写为 noreply）——gmail 此前只活在 .mailmap 显示层与提交信息文本里。修正三处：① PROVENANCE.md §2 身份表 gmail 行口径改为「本地推送提交使用」（网页 squash 合并仍记录 noreply 原始邮箱，经 .mailmap 归一显示）；② 签署须知补关键一条——签署 PROVENANCE 的提交必须本地完成并直推（`git config user.email sjxj19921205@gmail.com` → 本地 commit → push，别走网页合并），否则「提交即签署」验不回 gmail；③ §6 签署节补验证方法（验 raw email 而非 mailmap 显示层）。合并规范同步确立：需保留 gmail 作者身份的提交，本地合并后 push，替代网页端 Merge 按钮
- **Auto-merge Dependabot 工作流修复**：旧流程三个缺陷叠加，导致每个 dependabot PR 的 auto-merge 检查必然失败（4 个 PR 积压于此）——① `pull_request` 触发即跑，不等 CI；② `gh pr merge --auto` 依赖仓库未启用的 "Allow auto-merge" 设置，GraphQL 直接拒绝；③ `GITHUB_TOKEN` 提交的批准永不计入分支保护的 required review，等了也白等。新流程：未配置 `DEPENDABOT_MERGE_TOKEN` secret 时优雅跳过（::notice 提示手动处理，不再红叉）；配置后（fine-grained PAT，仅本仓库，contents:write + pull-requests:write）全自动——`gh pr checks --watch` 等 CI 全绿 → PAT 批准（计入 required review）→ squash 合并；semver-major 升级永不自动合并，留人工评审
- **aflare-action 文档 pin 回切**：示例 6 处 `@main` → `@v0.12.0`（README 中英 + action/README）；v0.12.0 是首个含 action/ 的 release tag，`@main` 不另行通知地移动；action/README 过期的 "Why @main?" 注记替换为正式 Pinning 说明（兑现仓库自立 TODO）
- **MCP catalog 供应链收口：版本全 pin + 清除 npm 下架死条目**：`aflare mcp install` 内置 catalog 此前全部 `npx -y <pkg>` / `uvx <pkg>` 裸引用（每次安装拉 registry 当日最新，分发链上唯一的浮动依赖点）。核实 registry 后两处修正：① 5 个存活条目全部锁精确版本——fetch `mcp-server-fetch@2026.8.18`（PyPI）、filesystem/memory/sequential-thinking/everything `@2026.8.31`（npm，官方 CalVer 统一发布）；② 发现并移除 3 个死条目——`@modelcontextprotocol/server-git`、`-sqlite`、`-time` 已从 npm 下架（404），`aflare mcp install git` 写入的启动命令根本起不来。回归钉子 `TestCatalogEntriesPinned`：catalog 新增条目必须带版本 pin（无 pin 即红），且三个下架包名不得回流（含被 pin 的变体）。mcp/xinchuang/README.md 清单同步（8 → 5）

## [0.12.0] - 2026-08-30

### Added

- **examples/real-world/digital-company-marketing / digital-company-sales（数字公司两部蓝本，对照 OpenExecutive）**：把 OpenExecutive（2.8k⭐）的三个核心纪律——角色职责分解、交接物契约化、对外单一声音——映射为部门级 agent 工作流。市场部：Intelligence Analyst（researcher，HN Algolia 开放信号采集，对应 CSO 竞争分析）→ GTM Strategist（structured_output，定位简报 JSON 契约，对应 CMO 的 GTM 职责）→ Campaign Copy Lead（agent，input 列表合并两路输入产统一声音文案包，提示词显式禁止内部角色泄漏）→ Brand Guardian（critic，brand_consistency / single_voice / no_internal_role_leakage 等六项评审）。销售部同构：Account Researcher（含 HN 评论流查询——公开抱怨是最密集的免费 B2B 痛点信号）→ Sales Analyst（ICP 评分档案 JSON）→ Account Executive（外联序列，限定"只能引用档案支持的主张"）→ Sales Ops Review（pricing_discipline / promise_compliance / spam_sensitivity 评审）。两部默认 Ollama 本地可跑、零商业依赖，validate 通过
- **examples/real-world/trading-agents（多智能体角色流水线）**：TradingAgents 范式的角色分工编排——Analyst（structured_output，行情快照 → JSON 情报，temperature=0 + schema 自纠错）/ Researcher（开放免密钥数据源调研）/ Trader（agent，列表形式 input 合并两路上游输出，产出 BUY/SELL/HOLD 一页决策）/ Risk Manager（critic，提案与评审角色分离）。默认 Ollama 本地可跑、零商业数据依赖（frankfurter / ECB），备忘录尾部内嵌免责声明
- **docs/openrouter.md（LLM 路由指南）**：多厂商降本两条纯配置路径——① OpenRouter：`OPENAI_API_BASE` 环境变量零改动切换（openai 节点）/ 任意 OpenAI 兼容节点 `endpoint` 参数逐步混用 / config 文件按项目钉住，模型命名 `vendor/model`；② 原生 llm_router 多厂商路由：cost / priority / round_robin / latency / pareto / random 策略、失败与 quota 用尽自动 fallback（max_retries）、`cost_per_1k` / `quota_daily` 逐厂商配额——该能力已实现但此前零散文（仅自动生成的 schema 表），本页为首个成文文档。两份 README 文档索引挂链
- **wait 节点（工作流内延时）**：`node: wait` + `duration`（Go duration 格式：500ms / 10s / 2m / 1h），暂停后原样透传输入——可插入任意两步之间（poll → wait → poll）不断数据流。语义要点：裸数字（"10" = 10ns）与负值直接报错并给出格式指引；单次上限 1h（更长间隔应走 `aflare schedule` 而非钉死 worker）；select 实现，工作流取消 / 步级超时立即打断等待而非睡满。8 条单测（注册、透传、0s 直通、非法/缺失/负值/超限、取消打断）；docs/nodes-reference.md 重新生成（72 nodes），skills 镜像补 wait 条目；examples/log-monitor.yaml 补 cool-down 用法（fetch → wait → filter，透传经端到端实测）
- **CI 增加 nodes-reference 新鲜度守卫（blocking）**：docs/nodes-reference.md 由 cmd/gen-nodes-doc 从 Schema() 元数据生成，但此前没有任何机制钉住——加节点后忘跑生成器，文档就静默过期（上一轮 68→70 漂移同成因）。ci.yml 水印检查之后新增一步：备份现存文档 → 现场重新生成 → diff，不一致即红，::error 直接给出修复命令
- **email_send 节点（SMTP 通知）**：465 隐式 TLS / 其余端口 STARTTLS，明文仅限回环中继（AFLARE_ALLOW_LOOPBACK 门控）；AUTH PLAIN + LOGIN 回退；凭据走 password_env，内联 password 永不落日志；拨号时 Control hook IP 校验（与 HTTP 节点同 SSRF 策略，封 DNS-rebinding TOCTOU）；from/to/cc/subject 经 net/mail 解析并拒 CR/LF 防头注入，subject Q-encoding；收件人 ≤50、正文 ≤100KB、超时 ≤120s；21 条测试（fake SMTP server）
- **HTTP/API 连接器**：http_request 可引用命名 http 连接器——base_url 源固定（绝对/协议相对 URL 拒绝）、bearer/basic/header 凭据经 CredentialResolver 运行时解析（不落 YAML）、read_only 默认仅 GET/HEAD、timeout 天花板；连接器注入的认证头与内联头冲突报错不静默覆盖
- **MCP Server HTTP 传输（token 必须）**：`aflare mcp --port 8082`，POST /mcp（JSON-RPC 2.0）+ POST /v1/call（简化直调）；X-MCP-Token 常数时间比较，无免认证模式；请求体上限 1MiB；默认绑 127.0.0.1，非回环须显式 --host
- **定时工作流失败重试（指数退避）**：`schedule add --retry N（0–10）--retry-delay 30s`，退避封顶 5m；panic 计为失败；停机中止等待中的重试；步级 retry 优先且独立
- **aflare-action（复合 GitHub Action）**：action/ 目录——sha256 校验的预编译二进制安装 + 运行工作流（秒级，非 Docker 源码构建）；支持版本 pin、--set、safe-mode、validate-only；macOS shasum 兼容
- **examples/real-world/ 工业案例**：OpenFOAM 发散监视、log-similarity-RAG 事故分诊

### Changed

- **README 重构为英文主文档（README.zh.md 中文镜像）**：定位 personal-first → local-first，tagline "AI Beyond Chat — Get Things Done"；470 行精简至约 155 行（砍特性矩阵/路线图，指向 docs/）
- **webhook 对不存在的工作流提前返回 404**（原 202 + 异步失败任务）

### Fixed

- **文档生成器按字节截断中文，产出非法 UTF-8（用户报告，#138 CI 守卫拦不住）**：cmd/gen-nodes-doc L49 的 `desc[:117]` 是字节切片——中文描述在第 117 字节被拦腰截断，docs/nodes-reference.md 两行中招（L27 doc_gen"模块"后断、L31 engineer_skills"支持"后断），GitHub 上现在就是 � 乱码。新鲜度守卫拦不住它：生成器每次都复现同样的坏字节，diff 是"干净"的。internal/output L252 是同一模式的运行时路径——ADHD 模式步骤输出 >120 字节时 `item[:117]` 截断，含中文的输出在运行时被截出非法 UTF-8 进日志与下游。修复：新增 internal/strutil.Truncate（字节预算内回退到 rune 边界），两处共用；实现上比"回退到 RuneStart"多防一步——切点恰好落在 lead byte（s[:1] = "\xe6"）时边界回退判不出来，孤立的 lead byte 同样非法，故用 DecodeLastRuneInString 逐字节剥到完整 rune（ASCII 与合法 U+FFFD 不受影响）。测试：strutil 全 max 扫描（每个切点都必须合法 UTF-8）+ 边界/中英混合/emoji/负预算用例，output 包补 ADHD 中文截断回归测试；nodes-reference.md 重新生成，全文件校验合法 UTF-8、零替换字符
- **OpenClaw 插件三处断裂（插件页随 v0.12 更新时端到端实测发现）**：contrib/openclaw 是挂在 38.8 万星生态上的分发渠道，但核心工具实际是坏的——① `aflare_run_workflow` 构造 `aflare run <file> --input <text>`，而 CLI 根本没有 `--input` flag：flag 解析走 default 分支进 filtered、只有 filtered[0] 被用作文件路径，input 被静默丢弃、从未送达工作流（插件主打卖点"带参数执行"名存实亡）；② `spawn(..., { shell: true })` + 未引用的外部输入——工作流输入是 OpenClaw 会话可控文本，shell 拼接即命令注入（`; rm -rf ~` / `$(whoami)` 原样执行），含空格输入还会被 word-split 截断；③ 插件 README 三个"Creating Workflows for OpenClaw"示例全部虚构语法（`prompt:` 参数不存在——LLM 节点用户消息走 step input；`{{output}}` 引用无效——正确语法 `{{step.<name>}}`），与此前修过的 skills 文档同款问题，照抄必炸。修复：`--set input=...`（单 argv 元素，空格/元字符原样送达，注入载荷不再被执行）、去掉 `shell: true`、三示例改为真实语法（含语法要点注释）、Features 补 OpenRouter/llm_router 多厂商路由、badge v0.12.0；版本漂移一并收敛（plugin.json 1.1.0 / index.ts 1.0.0 / package.json 1.1.0 → 统一 1.2.0），runWorkflow/describeWorkflow 补可选 aflarePath/workflowDir 参数使声明的配置项可接线；README 示例经 aflare validate 逐个验证。修复本身补了回归钉子：run_workflow.test.ts（node:test 零新依赖，stub aflare dump 原始 argv 断言精确契约——`--set input=` 单元素、无输入不带 flag、非 yaml 拒执行、注入载荷原样送达），openclaw-ci.yml 加 npm test 步骤；负向验证过杀伤力——把 shell: true 临时加回去，argv 契约测试立刻红
- **LLM 预检无视环境变量配置（docs/openrouter.md 编写过程中端到端实测发现）**：`detectLLMConfig` 只读配置文件——`export OPENAI_API_KEY=...` 这类纯环境变量配置（OpenAI 兼容生态最标准的方式，也是新文档 docs/openrouter.md 的主推路径）被预检拦死，报"尚未配置 LLM provider"，而提示语自己却写着"配置 DeepSeek/OpenAI API Key"。现预检同时扫描各 provider 的 EnvAPIKey（经 providers.OpenAICompatibleConfigs()，不重复维护名单）；新增 TestDetectLLMConfig_EnvVarOnly 钉死（含 configOnce 进程级缓存导致的单跑/合跑顺序敏感问题的处理说明）
- **技能文档版本漂移（用户报告）**：skills/aflare/SKILL.md 写 `version: 0.10.0`、两处宣传 "45+ nodes"，实际二进制 v0.11.0、注册节点 70 个（`aflare list` 实测）；skills/aflare/nodes-reference.md 也停留在 "68 registered nodes"。上架技能市场前需同步刷新元数据——现已全部对齐（0.11.0 / 70），并新增 `TestSkillMetadataVersionSync` 防复发测试：SKILL.md frontmatter 版本必须与 `internal/meta` 的 `Version` 常量一致，下次发版漏更会直接红 CI。docs/nodes-reference.md 一并重新生成（70 nodes，含 pipeline 失败语义说明）
- **pipeline 节点 DAG 依赖失败后下游仍执行（用户报告）**：`tryStart` 只检查 `completed[dep]`，而失败步骤（含 panic 路径）同样被标记 completed——a1/a2 失败后依赖它们的 b1 照常调度，拿着缺失/错误的 input 运行，语义与多数 DAG 系统"依赖失败则跳过"直觉不符且文档未说明。现实现标准级联跳过语义：步骤失败后，其全部传递下游被标记 `skipped: true`（error 注明根因 `skipped: upstream step "<name>" failed`），不再执行；独立分支不受影响照常运行；整体 `success: false` 由根因错误决定（级联跳过不重复计入 errors）。实现要点：skip 结果在同一锁区段内写入（否则同轮 map 遍历顺序下传递下游读不到直接父级的失败结果，级联断裂——首轮测试实测踩中）。Schema Input/Output 补失败语义说明并重新生成 docs；新增测试钉死：根因报错非跳过、直接下游跳过指认根因、传递下游级联跳过指认直接依赖、独立分支照跑
- **docs/dataflow.md 文档了不存在的 `input:` / `id:` 字段（用户报告）**：文档示例大量使用步骤级 `input: "..."` / `input: [...]` 传参与 `id:` 命名，但 `WorkflowStep` 结构体根本没有对应 yaml 字段——`yaml.Unmarshal` 静默忽略，用户照文档写工作流 → input 静默丢弃 → 节点拿到空输入，排查成本高。现实现两字段：① `input:`——新 `StepInput` 类型（标量模板 / 模板列表两种形式，非法形态解析期报错而非静默丢弃），列表各项单独渲染后以 `\n---\n` join，全部占位符（`{{step.*}}` / `{{var.*}}` / `{{input}}` / `{{env.*}}` / `{{secret.*}}`）可在内求值；② `id:`——作为 `name:` 的别名在解析期提升（递归穿透 if/map/reduce/saga 复合子工作流，`name:` 同时存在时以 `name:` 为准），`{{step.<id>}}` 引用由此生效。覆盖全部三条执行路径（顺序 / DAG / 复合子步骤）且语义统一：覆盖先于该步的 condition 与 params 求值（`{{input}}` 见到覆盖后的值，与 DAG 模式一致）；条件跳过的步骤其覆盖值不向下游泄漏（跳过仍为透传上游原值）。docs/dataflow.md 补权威语义说明；新增 9 条测试钉死解析形态、别名提升、三条路径的运行时行为与跳过透传
- **pipeline 节点 YAML 支持是虚假宣传（用户报告）**：Schema 声称 Input 为 "YAML or JSON pipeline configuration"、参数表声称 `format: json|yaml|auto`，但实现 `parsePipelineConfig` 只认 `{` / `[` 开头的 JSON——`format` 参数被文档记载却在 Execute 中从未读取，YAML 配置直接报 `unsupported format, use JSON`。现实现承诺的能力：`PipelineStep` / `PipelineConfig` 补 yaml tag（snake_case 的 `depends_on` / `input_from` / `timeout_seconds` 正确映射）；`format` 参数生效——`auto`（默认）按载荷形态分派（`{`/`[` 前缀走 JSON、其余走 YAML）、`json` / `yaml` 强制指定解析器（错误信息精确）、非法值（如 `xml`）解析期报错而非静默忽略。新增 4 条测试：YAML 解析（含全部 snake_case 字段与 timeout_seconds）、auto 双格式探测、显式 format 强制与非法值报错、YAML 配置端到端执行（依赖链 a→b→c + input_from 汇聚）。parsePipelineConfig 此前零测试覆盖
- **`http_request` → `json_parse` 生态组合断裂（用户报告）**：文档声称 http_request 输出 "response body"，实际输出 `HTTP <code>\n<body>`（状态行前缀拼接，单测锁定）——最自然的 `http_request → json_parse` 组合开箱即断（`HTTP` 开头不是合法 JSON，json_parse 直接失败），仓库自己的 examples/finance/idempotent-transfer/workflow.yaml 注释承认该问题并用 `contains` 字符串匹配绕过。根因修复：json_parse 解析前自动剥离行首 `HTTP <status>\n` 状态行（合法 JSON 不可能以 `H` 开头，剥离无歧义；锚定行首、容忍 CRLF，body 内部状态样式文本不受影响）；http_request 输出格式保持不变（向后兼容既有 `contains:"HTTP 200"` 类判断）。文档同步：nodes-reference.md 的 http_request Output 改为如实描述 `HTTP <status>\n<body>`（并注明 json_parse 自动剥离）、json_parse Input 注明容忍状态行；example 注释更正。回归测试覆盖前缀剥离（pretty-print / path 提取 / 任意状态码 / CRLF）、body 内嵌状态样式文本不被误剥、非 JSON body 仍报错
- **Ollama 流式输出多字节 rune 损坏（本轮全量安全与代码质量审计发现）**：流式 JSON 过滤器 `ollamaStreamFilter` 的前缀匹配把 rune 截断为 byte——中文等非 ASCII 内容的低字节可能碰巧匹配 `"thought": "` 前缀字节而误入字段模式，把无关 JSON 值当作思考内容流给用户（如 U+0174 截断为 't'，`"ŴhoughŴ": "` 在截断语义下拼出前缀）；同时过滤器内嵌只写不读的 `strings.Builder` 缓冲区随输出无界增长（死代码 + 长流内存浪费）。修复：非 ASCII rune 直接重置匹配状态（前缀为纯 ASCII，多字节 rune 不可能合法延续匹配）并删除死缓冲区；新增两条回归测试——CJK 内容原样流出、低字节碰撞序列不得误触发字段模式
- **`aflare validate` 对纯建议性警告返回退出码 1（本轮全量审计发现）**：`Consider adding a file_write step` 这类建议（工作流照常可跑）与 unknown node 这类硬错误（跑到该步必炸）此前同走 `exit 1`——CI 里 `aflare validate wf.yaml && ...` 会被纯建议卡死，仓库自带 3 个 examples（devops-deploy / file-organizer / log-monitor）即受害者。现区分严重度：unknown node、空 steps、YAML/加载错误保持退出 1；缺 name / 缺 file_write 等纯建议退出 0（警告照常打印）。新增退出码契约测试
- **SSRF 拨号路径尊重本地出站代理**；examples 全部改用开放数据源
- **用户视角硬化**：doctor/create 输出迁移 i18n（--lang 端到端生效）；secrets keyring 缓存不可用日志降为 debug（无头环境预期）；validate 空 node 名报 missing 'node' field；aflare create 保留 read-file intent

## [0.11.0] - 2026-08-25

### Added
- **Agent 互联与指挥（`internal/agentx` 包，aflare 作为上位指挥者）**：项目此前只能被其他 Agent 集成（MCP Server / DSH 插件），方向反过来才是个人自动化主线的缺口——用户装好 aflare 后由 aflare **指挥和监督其他 Agent 干活**，而非把 aflare 做成别人生态里的 skill。两条互联通道：① **CLI 通道**——本地 Agent CLI（codex / claude / gemini 内置预设 + 任意通用 CLI profile）作为受管子进程运行：二进制路径解析、参数白名单校验（model/sandbox/approval 按各 CLI 语义映射）、prompt 永远作为单个 argv 末位元素传递（不参与 flag 解析，杜绝命令注入）、超时硬边界（默认 10m / 上限 60m）、输出捕获；② **A2A 通道**——A2A 协议远程 Agent：Agent Card 发现（agent-card.json + agent.json 双路径）、`message/send` → `tasks/send` 新旧方法回退、`tasks/get` 轮询至终态、Bearer 认证（密钥走 `api_key_env` 环境变量引用，不落盘）、SSRF 防护 dial + 10MB 响应上限。指挥入口三个：supervisor 节点 `specialists: "@codex,@my-a2a"` 真实委派（LLM 规划子任务 → 并行监督执行 → LLM 综合；无 LLM 时全量扇出 + 拼接汇总，命令与监督不依赖规划器）、`cli_agent` / `a2a_agent` 单步委派节点、`aflare agent list` 注册表查看；外部 Agent 经 `agents:` 配置段注册（内置预设可按名覆盖）。生产级语义：**失败隔离**（单 Agent 失败只记录该次结果，不拖垮整批）、**背压**（`max_parallel` 并发上限，默认 4 / 钳制 1..16，超发子任务排队而非一次性 fork 全部子进程）、**瞬时故障重试**（A2A 提交阶段仅重试 dial 错误——请求未送达、重试不可能重复委派；轮询阶段幂等读重试传输错误与 5xx——此前单次瞬时 502 杀死整个委派；2 次退避重试 + context 感知）、**fail-closed 审计**（每次委派前审计钩子失败即拒绝执行）。agentx 覆盖率 86.5%，纳入 ci.yml per-package 门禁（阈值 60%）
- **守护进程稳定性基建（soak 测试 + nightly workflow + goroutine 泄漏检测）**：生产级要求"跑得久"而非"跑得过"——新增进程级集成与 soak 测试（`AFLARE_SOAK=1` 门控，`-short` 跳过）：真实拉起 `aflare agent` 守护进程（scheduler + filewatch 全活跃），10s/30s/50s 采样 RSS 与线程数、注入文件事件搅动，断言内存增长有界（<32MB）、线程数稳定（≤32）、SIGINT 优雅退出；nightly soak workflow（每日 18:00 UTC 定时 + 手动触发）跑 soak + 全量非 short 测试套件 + daemon 相关包 race 检测；taskqueue / scheduler 接入 goleak.VerifyTestMain（Start/Stop 循环不得遗留 tick/task goroutine）。首跑已验证：RSS 24→27MB 平台期、线程恒定 7、无泄漏
- **`aflare webhook` 命令：事件驱动触发入口（WebhookServer 首次接线 CLI）**：WebhookServer 本体早已完工（异步任务 202 + task_id + 轮询、per-IP 限流、1MB body 上限、100 并发上限、无 secret 只绑 127.0.0.1）但从未接到任何 CLI 命令——项目触发面此前只有时间驱动（`aflare schedule` cron），事件驱动为空。现 `aflare webhook [--port] [--host] [--secret] [--dir]` 补上该入口：外部调用方**按名触发本地已注册的工作流文件**，body → `{{var.input}}`、query → vars，不能注入新逻辑——与 `aflare serve`（执行请求体内任意工作流定义）形成信任模型互补，这是不可信事件源（GitHub/Gitea/Forgejo webhook、告警回调、n8n/Make）的正确入口。安全：secret 可经 `AFLARE_WEBHOOK_SECRET` 环境变量传入（避免 ps 泄漏）；绑定非回环地址且无认证时拒绝启动（与 serve 同款守卫）
- **webhook 认证新增 GitHub HMAC 签名校验（X-Hub-Signature-256）**：此前仅共享 secret 头（"知道密码就能发"），现同一 secret 双凭证：① GitHub/Gitea/Forgejo 标准 `X-Hub-Signature-256: sha256=<HMAC-SHA256(body)>` 签名——同时证明来源与 body 完整性，仓库 webhook 设置配同一 secret 即可；② 通用 `X-Webhook-Secret` 头（curl/n8n/定时任务）。安全语义：携带**无效签名**的请求直接 401、不回退共享 secret 检查（坏签名是篡改证据而非缺失凭证）；签名校验为常数时间；status 端点维持仅共享 secret（GET 无 body 可供签名验证）。E2E 实测：openssl 签名触发 → 202 → 工作流执行 → 审计链 start/step/end 完整；篡改 body 用旧签名（即使另附有效共享 secret）→ 401
- **Connector API（命名数据源连接）**：新增 `internal/connector` 包与 `aflare connector add / list / show / remove` 命令——postgres / mysql / sqlite / files / notes 五类数据源注册为命名连接器（配置存 `~/.aflare/config/connectors.yaml`，`AFLARE_CONNECTORS_FILE` 可覆盖），`sql_query` / `file_read` / `file_write` / `files_list` 四节点新增 `connector` 参数，工作流引用连接器名而非内联 DSN 或路径。安全设计：凭据只存引用（加密 secrets store 或环境变量），spec 与 workflow YAML 永不落明文；只读默认（写需连接器显式开启），节点参数只能收紧不能放宽；天花板三件套 `max_rows`（默认 1000）/ `timeout`（默认 30s）/ `max_bytes`（默认 10MB，单文件读取上限）；postgres DSN 经 `url.UserPassword` 构造，密码特殊字符无法改变 DSN 结构；sqlite 拒绝 `file:` 前缀与 `?#` 参数并强制 `mode=ro`；文件连接器以 root 为围栏——`EvalSymlinks` 全链解析、最终组件拒绝符号链接（含悬空，防追加跟走与写入替换）；registry 原子保存（tmp+rename，文件 0600 / 目录 0700，防 tmp 被符号链接栽赃）（#95、#96、#97）
- **全源码隐形水印与 CI 门禁**：所有 Go 源文件版权行内嵌零宽字符水印（U+200B / U+200C 表比特、U+200D 分片标记），新增 `aflare watermark encode-source --all` / `check-source` / `decode-source` / `strip-source` 命令；CI 增加覆盖门禁——新增源文件无水印直接红叉。与部署水印（payload v2）互补：部署水印标记"哪次部署"，源码水印标记产物来自哪个源码树，用于泄露溯源

### Changed
- **主 usage 补齐 10 个可达命令**：PrintUsage 此前只列 18 个命令，`validate` / `list` / `init` / `config` / `secrets` / `schedule` / `audit` / `review` / `serve` / `webui` 均已实现且可分发，但用户从 help 不可发现——现全部补入（中/英 locale 新增 9 个 usage.* key；`usage.schedule` key 早已存在只是从未被引用）；同步删除 5 个引用不存在命令的死 key（usage.debug / usage.history / usage.plugin / usage.secret / usage.template，×2 locale）
- **CodeQL 74 条告警清零**：告警均属 taint tracking 无法建模 aflare 守卫（validateReadPath / validateURL / 命令白名单 / SQLite 标识符转义等）的误报——排除走规则级 query-filters（`.github/codeql-config.yml` 逐条注明理由与对应守卫），不采用行内 `// codeql` 注释（GitHub code scanning 不识别）与路径级过滤（对 Go 不生效，实测 excluded queries 仍会产生告警）；排除 7 条规则后其余全部保持激活，gosec 与 golangci-lint 门禁不受影响
- **gocritic 存量违例清零并启用 linter**（#57、#91）
- **workflow 上帝文件拆分**：generator 拆为 generator_llm / generator_parse / generator_validate 等 topical 模块（#84，13 文件，+3022 / -2782），拆分产生的新文件同步补嵌源码水印
- **CLI os.Exit 收敛为单点 dispatch**，internal/cli 测试覆盖率 21% → 61%

### Removed
- **NodesByCategory 死代码与 24 个幽灵条目（本轮全量自检发现）**：`Registry.NodesByCategory` 及 `NodeCategory` 八个常量、硬编码 categoryMap 全仓零生产调用（仅测试引用），且映射严重漂移——24 个条目对应的节点从未注册（cohere/together/groq/file_append/file_list/stdin/stdout/output/markdown_render/base64_encode/base64_decode/if/switch/loop/parallel/map/hash/encrypt/decrypt/sign/echo/log/env/variable），28 个实际注册节点反而漏列（files_list/codex_agent/全部国产芯片 provider 等）；这是 #98 修掉的 wait/node_marketplace 幽灵条目同款温床。整体删除（含配套测试），节点发现统一走 `List()` / `Search()`（均基于真实注册表）
- **内置模板库与内容生态（破坏性变更，#94）**：删除 17 分类 330 个内置技能模板与 `templates/` 目录（-1041 文件 / -71375 行），移除 `internal/{templates,skills,marketplace,badge,packs,agentplugins}` 包及 CLI 命令 `template` / `skills` / `marketplace` / `badge` / `install-pack`；agent 工具 `template_list` / `template_info` 与 MCP 模板工具同步移除。理由：330 个模板零真实使用信号，marketplace / badge 无人需要，维护成本无回报；项目定位收敛为"引擎 + 连接器 + 安全"，内容用户自带。保留：`run` / `create_workflow`、`template_render` 节点（渲染用户自带模板）、`examples/` 示例、用户本地模板目录扫描。迁移：引用内置模板的工作流改为本地模板路径；使用已删命令的脚本需移除
- **俄语 locale 整体移除（破坏性变更，本轮自检发现）**：项目无俄语 README/文档（仅中英双语），CLI help 一直只写 `(en, zh)`，俄语用户既无获客入口也无官方宣传；`systemPrompts` 俄语仅 2/9 个 action prompt（summarize/translate，其余 7 个 action 返回空字符串，比回退英文更糟）；同一周期已出现 key 漂移（resume.* 8 个 key 缺失，俄语界面直接显示 raw key）。零使用信号 + 持续同步成本（每新增 key ×3 locale）——与 #94 裁撤逻辑一致。删除 `locales/ru.json`（172 key）、`normalizeLang` 俄语分支、generator 俄语 prompt 与配套测试；`--lang ru` 现回退英文（带不支持警告）。顺带清理 `systemPrompts` 中 fr/ja/ko/es/ar/hi 六个不可达死条目（`normalizeLang` 只可能返回 en/zh，六种语言永不会被选中）

### Fixed
- **守护进程 SIGINT/SIGTERM 完全无法停止（生产级稳定性，soak 测试发现）**：daemon 主循环内联 `scanner.Scan()` 阻塞在 stdin 读取，stdin 空闲时（生产守护的常态——无人交互）信号永远无法被观察——`aflare agent` 此前只能 SIGKILL 停止，systemd / supervisor 等进程管理器会 hang 到 stop timeout 超时强杀，优雅清理逻辑（Goodbye! 拆卸）完全不可达。修复：stdin 读取移入独立 goroutine，主循环 select 同时监听信号、ctx.Done() 与输入通道；回归测试以 stdin 空闲状态拉起真实进程、发 SIGINT、断言 15s 内退出且完成优雅清理
- **API server / webhook 两个远程入口与 run 路径同款双缺口（安全，#103 合并后入口普查发现）**：修完 resume 后对全部 executor 构造点做普查，发现同款 bug 还有两个受害者——① `aflare serve` 的 `POST /api/v1/workflows/run` 用裸 executor（只设 timeout）执行**请求体里任意的工作流定义**：零审计记录、零策略校验；② webhook server 的 `runTask` 用包级 `ExecuteWorkflow` 执行工作流且把请求 body/query 注入 vars：同样零审计、零策略。两处均比 resume 更严重——resume 好歹要本地敲命令，这两个是远程可达入口。现两处均对齐 run 路径：审计启用（DefaultPolicy、无审批函数时 approval 类动作 fail-safe 拒绝——远程路径上没有人类可审批）+ PolicyExecutor 前置校验（API 违规返回 400，webhook 任务标记 failed）。注：WebhookServer 目前未接线到任何 CLI 命令（仅测试引用），属库级公开 API 修复。防复发护栏：每个入口补"执行后审计日志必须有 start/step/end 记录"+"策略违规零执行零审计"回归测试——下次有人自建 executor 绕过审计，测试直接红
- **resume 路径与 run 路径双缺口：审计黑洞 + 策略绕过（本轮个人用户全流程自测发现）**：`ResumeWorkflow` 用裸 executor 执行恢复——① 恢复的运行**不写任何审计记录**（`aflare run` 写 start/step/end 三类记录，resume 一条没有，审计链出现黑洞）；② 完全绕过 PolicyExecutor（`--safe-mode` 暂停的运行恢复时策略全部失效；暂停后编辑 workflow 文件加 shell 步骤也照跑不误）。现 ResumeWorkflow 默认启用审计（新增 `ResumeWorkflowWith` 允许 CLI 传入持跨进程审计锁 H-6 的 executor，锁被占时降级为禁用审计并告警，绝不分叉哈希链）；`RunMeta` 新增 `safe_mode` 字段在暂停时记录策略上下文，恢复时按记录的策略类重新校验全部步骤（策略拦截时保持 paused 可修复后再恢复）；CLI `aflare resume` 走与 run 相同的审计锁路径。E2E 实测：篡改 workflow 加 shell → `resume blocked by policy: shell execution not allowed`，恢复原文件 → 正常完成，审计 3→7 条且整链校验通过；伪造 `.audit.lock` → resume 照常完成 + 明确警告 + 链未分叉
- **`aflare resume --list` 文档与实现不一致（本轮自测发现）**：pr-review-gate 示例 README 写 `aflare resume --list`，但 CLI 只接受位置参数 `list`——照文档敲命令直接报 `failed to load run "--list"`。现 `--list` / `-l` 作为别名支持；README 同步删除不存在的"按提示选择 run-id"交互描述、修正审计日志路径（`~/.config/aflare/history/audit.log.jsonl`）与示例 run-id 格式（UUID）
- **`aflare list` 漏 20 个节点（本轮自测发现）**：HandleList 自建私有 registry（仅 RegisterBuiltins），而 agent / memory / files_list / code_interpreter / drone / supervisor team 等节点经 init() 自注册进全局 registry——`aflare list` 只显示 48/68 个节点，且恰好漏掉 connector 生态关键的 files_list。改用 GetGlobalRegistry（RegisterBuiltins 幂等回填）
- **file_write `content` 参数文档承诺但未实现（本轮自测发现）**：nodes-reference.md 与 SKILL.md 均记载 `content` 参数（"Content to write; defaults to step input"），但节点实现只认 step input、静默忽略 `content`——照文档写的 `params: {path: x, content: y}` 工作流把空输入写进文件。现实现该参数（覆盖 step input，append/write 两模式均生效），补单测
- **`secrets delete` 子命令缺失（本轮自测发现）**：secrets 有 set/get/list 却没有 delete——照 help 的 `<group>/<key>` 语义管理凭据时无法删除已泄密的条目。新增 `aflare secrets delete <group> [<key>]`（key 省略整组删除）
- **审计日志泄漏 secret 明文（安全，本轮自测发现）**：步骤参数按 key 名脱敏，但工作流把 `{{secret.GROUP.KEY}}` 模板进数据后，secret 值会出现在 step 的 input/output/error 字段——值级脱敏缺失导致明文落盘审计日志。现审计写入前按已知 secret 值做值级替换（加载一次、按长度降序防部分掩码泄漏），配套单测覆盖多 secret 嵌套场景
- **审计链 key 切换静默分叉（本轮自测发现）**：同一审计链先后用不同 key 签名（如一次带 `AFLARE_SECRETS_PASSWORD` 运行、一次不带）后，VerifyAuditChain 从切换点起永久失败且无人知晓何时坏的。现追加前做链连续性检查——定位上一条记录实际使用的 key 并续用；无任何可用 key 能验证既有链时**拒绝追加**（ErrAuditKeyUnavailable，警告含修复指引），链保持可验证而不是静默分叉
- **skills 文档表达式语法全错（本轮自测发现）**：SKILL.md / examples.md / nodes-reference.md 中 `{{.steps[N].output}}` / `{{.step.name.output}}` 等写法引擎根本不支持（正确语法为 `{{step.name}}` / `{{step.N}}`），照抄示例必然报 "variable not found"；`vars` 参数的 map 语法、condition 示例同步修正。另修正 usage 帮助引用不存在的 `examples/basic_summary.yaml`（模板库 #94 删除后的幽灵示例，改为 data-collector.yaml）与 init wizard 中指向已删除模板的"不需要 LLM 的模板"提示
- **govulncheck 本地不可用 + CI 四工作流 @latest 不可复现（工具链债）**：AGENTS.md 要求提交前本地跑全套门禁，但 mise.toml 未钉 govulncheck——shim 直接报 "No version is set" 退出，俄语移除提交的本地门禁实际缺了这一环；且 4 个 workflow（ci / pr-review / supply-chain / security-auto-fix）全部 `@latest`，同一棵树在不同时间可能被不同版本工具扫描，结果不可复现。现 mise.toml 与四个 workflow 统一钉至 v1.7.0（2026-08-13 发布，当前最新），AGENTS.md 工具链节同步登记——与 go / golangci-lint / benchstat 既有钉版纪律一致
- **`aflare connector` 子命令完全不可达（断点，本轮自检发现）**：`HandleConnector` 已实现且 main.go 分发器有 `case "connector"`，但 `cli.knownCommands` 漏列 `"connector"`——`ValidateCommand` 在分发前直接以 unknown command 拒绝，`aflare connector add/list/show/remove` 一律不可用。现补入命令表，并在主 usage（PrintUsage，中/英 locale）加 connector 一行
- **skills/aflare/nodes-reference.md 谎称 Complete catalog（本轮全量自检发现）**：AI 入口技能文档实际只收录 6 个核心节点却自称"完整目录"，且无任何连接器内容——改为诚实标注"精选子集 + `aflare list` 查看 68 节点全量"，file_read / file_write 条目补 `connector` 参数说明（含只读连接器拒写、天花板生效语义）
- **mise.toml 未钉 golangci-lint 版本**：AGENTS.md 要求本地 lint 与 CI 一致（v2.13.1），但 mise.toml 只钉了 go——本地默认解析到 2.1.6（go1.24 构建），跑本仓库直接报 "can't load config" 退出；现补钉 `golangci-lint = "2.13.1"`

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
