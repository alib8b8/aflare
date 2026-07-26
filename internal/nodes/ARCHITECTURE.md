# Nodes Architecture

Node 文件按功能分为以下几类（均在 `nodes` 包内）：

## 1. Core Infrastructure (核心基础设施)
- `node.go` - Node 接口定义、注册表、外部节点加载
- `call.go` - 节点调用与执行框架
- `agent_node.go` - Agent 节点基础
- `llm_base.go` - LLM 节点基础类
- `pipeline.go` - Pipeline 节点
- `router.go` - 路由节点基类
- `test_node.go` - 测试节点

## 2. LLM Providers (模型供应商)
- `openai.go`, `ollama.go`, `anthropic.go`, `gemini.go`
- `deepseek.go`, `qwen.go`, `kimi.go`, `glm.go`, `yi.go`, `mistral.go`
- `baichuan.go`, `internlm.go`, `minimax.go`, `xverse.go`, `coze.go`
- `fastgpt.go`, `ima.go`, `sensenova_node.go`, `andesgpt_node.go`
- `ondevice_llm.go`, `inference_backend.go`

## 3. Agent Nodes (智能体节点)
- `agent.go` - ReAct Agent
- `agent_helper.go` - Agent 工具函数
- `agent_browser.go` - Agent 浏览器
- `planner.go` - 规划器
- `reflector.go` - 反思器
- `evaluator.go` - 评估器
- `critic.go` - 批评者
- `supervisor.go` - 监督者/技能进化
- `mimo.go` - 多输入多输出
- `meta_orchestrator.go` - 元编排器
- `subagent_prompts.go` - 子 Agent 提示词

## 4. Router Nodes (路由节点)
- `llm_router.go` - LLM 路由核心
- `llm_router_node.go` - LLM 路由节点
- `smart_router.go` - 智能路由
- `omniroute_node.go` - OmniRoute 兼容节点

## 5. Tool Nodes (工具节点)
- `fetch_url.go` - URL 抓取
- `http_request.go` - HTTP 请求
- `file_read.go`, `file_write.go`, `file_watch.go` - 文件操作
- `execute.go` - 命令执行
- `json_parse.go` - JSON 解析
- `search_aggregate.go` - 搜索聚合
- `code_interpreter.go` - 代码解释器
- `template_render.go` - 模板渲染
- `compress.go` - 压缩
- `condition.go` - 条件判断
- `clarify.go` - 澄清询问
- `human_in_loop.go` - 人在回路
- `transform.go` - 数据转换
- `combine.go` - 合并节点
- `verify.go` - 验证节点
- `preference.go` - 偏好收集
- `quality_guard.go` - 质量守卫
- `output_quality.go` - 输出质量检测
- `mcp_bridge.go`, `mcp_server.go` - MCP 集成
- `plugin_system.go` - 插件系统
- `marketplace.go` - 市场节点
- `tools_compat.go` - 工具兼容层

## 6. Code Intelligence (代码智能)
- `code_review.go` - 代码审查
- `code_graph.go` - 代码图
- `code_knowledge_graph.go` - 代码知识图谱
- `doc_gen.go` - 文档生成
- `engineer_skills.go` - 工程师技能
- `self_heal.go` - 自修复
- `antling_node.go` - 百灵代码助手
- `skill_distill.go` - 技能蒸馏
- `skill_explorer.go` - 技能探索

## 7. Data & Memory (数据与记忆)
- `memory_node.go` - 记忆节点
- `knowledge_graph.go` - 知识图谱
- `rag.go` - RAG 检索增强
- `history.go` (in workflow) - 历史管理

## 8. Specialized Nodes (专业领域节点)
- `mobile_nodes.go` - 鸿蒙/移动端节点
- `blockchain_audit.go` - 区块链审计
- `voice_input.go`, `voice_output.go` - 语音输入输出
- `power_manager.go` - 电源管理
- `screen_understanding.go` - 屏幕理解
- `robot_control.go` - 机器人控制
- `video_edit.go` - 视频编辑
- `system_event.go` - 系统事件
- `cli_session.go` - CLI 会话模拟

## 9. Security (安全)
- `security.go` - 安全检查
- `secretdetect.go` - 密钥检测

## 10. Communication (通信)
- `notify.go` - 通知
- `swarm_comm.go` - 蜂群通信
- `webhook.go` (in webhook) - Webhook

## 11. Multimodal (多模态)
- `multimodal.go` - 多模态处理
- `moe_streaming.go` - MoE 流式输出
