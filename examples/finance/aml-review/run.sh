#!/usr/bin/env bash
# AML 可疑交易审查 Demo 一键运行脚本
#
# 流程：启动 mock 服务 → 场景匹配器路由业务事件 → 运行 AML 工作流 → 输出报告
#
# 用法（从 hkaic 仓库根目录执行）：
#   bash examples/finance/aml-review/run.sh
#
# 也可自定义业务事件：
#   bash examples/finance/aml-review/run.sh "收到可疑交易告警，需查关联方黑名单"
set -euo pipefail

# ── 定位仓库根目录（含 go.mod 的目录）──
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/../../.."
cd "$REPO_ROOT"

DEMO_DIR="examples/finance/aml-review"
OUTPUT_DIR="$DEMO_DIR/output"
REPORT_FILE="$OUTPUT_DIR/report.md"
SCENES_FILE="$DEMO_DIR/scenes.yaml"
WORKFLOW_FILE="$DEMO_DIR/workflow.yaml"

# 业务事件：取第一个参数，默认演示一个可疑交易告警
EVENT="${1:-收到 3 笔可疑交易告警，金额异常，需查关联方是否命中黑名单与制裁名单}"

mkdir -p "$OUTPUT_DIR"

# 清理上次残留的 mock 服务（按端口杀进程，避免 "address already in use"）
kill_by_port() {
  local port="$1"
  local pids
  pids=$(lsof -ti tcp:"$port" 2>/dev/null || true)
  if [ -n "$pids" ]; then
    echo "▶ 清理占用端口 $port 的进程：$pids"
    kill $pids 2>/dev/null || true
    sleep 1
  fi
}
kill_by_port 17790
kill_by_port 17791

# ── 启动 mock 服务（后台），退出时自动清理 ──
echo "▶ 启动 mock 服务（LLM :17790 / 数据源 :17791）..."
go run "./$DEMO_DIR/demo/mockserver" &
MOCK_PID=$!
cleanup() {
  echo ""
  echo "▶ 停止 mock 服务..."
  # `go run` 编译出的子二进制在父进程被 kill 后仍会存活（reparent 到 init），
  # 仅 kill $MOCK_PID 会留下占用端口的孤儿 mockserver，导致脚本末尾的管道
  # (| tail) 无法收到 EOF 而挂起。按端口杀进程可确保子二进制一并退出。
  kill "$MOCK_PID" 2>/dev/null || true
  wait "$MOCK_PID" 2>/dev/null || true
  kill_by_port 17790
  kill_by_port 17791
}
trap cleanup EXIT

# 等待 mock 服务就绪（最多 15 秒）
echo "▶ 等待 mock 服务就绪..."
READY=0
for i in $(seq 1 30); do
  if curl -sf "http://localhost:17791/blacklist?name=ping" >/dev/null 2>&1 \
     && curl -sf -X POST "http://localhost:17790/chat/completions" \
        -H 'Content-Type: application/json' \
        -d '{"messages":[{"role":"user","content":"ping"}]}' >/dev/null 2>&1; then
    echo "✅ mock 服务已就绪"
    READY=1
    break
  fi
  sleep 0.5
done
if [ "$READY" != "1" ]; then
  echo "❌ mock 服务未就绪，退出"
  exit 1
fi

# ── 场景匹配器：业务事件 → 工作流路由 ──
echo ""
echo "══════════════════════════════════════════════════════════"
echo " 场景匹配：业务事件 → 工作流自动路由"
echo "══════════════════════════════════════════════════════════"
echo "业务事件：$EVENT"
MATCH_JSON=$(go run "./$DEMO_DIR/demo/matcher" \
  -scenes "$SCENES_FILE" \
  -event "$EVENT")
echo "匹配结果：$MATCH_JSON"

# 从匹配结果提取工作流路径；匹配失败则回退到 AML 工作流
MATCHED_WF=$(echo "$MATCH_JSON" | sed -n 's/.*"workflow":"\([^"]*\)".*/\1/p')
if [ -z "$MATCHED_WF" ] || [ "$MATCHED_WF" = "$MATCH_JSON" ]; then
  echo "⚠ 未匹配到场景，回退到默认 AML 工作流"
  MATCHED_WF="$WORKFLOW_FILE"
fi

# 占位工作流（kyc/card-fraud/str）尚未实现，回退到 AML
if [ ! -f "$MATCHED_WF" ]; then
  echo "⚠ 匹配的工作流 $MATCHED_WF 不存在（占位场景），回退到 AML 工作流"
  MATCHED_WF="$WORKFLOW_FILE"
fi

echo "运行工作流：$MATCHED_WF"
echo ""

# ── 运行 AML 审查工作流 ──
# http_request 默认禁止访问 localhost（SSRF 防护）。Demo 的 mock 服务跑在
# localhost，故显式开启 loopback 放行（仅限本地 demo，生产环境切勿设置）。
echo "══════════════════════════════════════════════════════════"
echo " 运行 AML 可疑交易审查工作流"
echo "══════════════════════════════════════════════════════════"
LLMBOX_ALLOW_LOOPBACK=1 go run ./cmd/aflare run "$MATCHED_WF"

# ── 展示报告 ──
echo ""
echo "══════════════════════════════════════════════════════════"
echo " 审查报告（$REPORT_FILE）"
echo "══════════════════════════════════════════════════════════"
if [ -f "$REPORT_FILE" ]; then
  cat "$REPORT_FILE"
else
  echo "⚠ 报告文件未生成：$REPORT_FILE"
  exit 1
fi
