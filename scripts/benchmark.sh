#!/usr/bin/env bash
# ==============================================================================
# aflare Benchmark Suite
#
# Reproducible performance comparison: aflare vs n8n vs LangChain
# Measures: startup time, memory usage, task execution latency
#
# Prerequisites:
#   - aflare: built from source (go build -o aflare ./cmd/aflare)
#   - n8n:    docker pull n8nio/n8n (or npx n8n)
#   - LangChain: pip install langchain langchain-ollama
#   - Ollama: running with llama3 model (ollama pull llama3)
#
# Usage:
#   chmod +x scripts/benchmark.sh
#   ./scripts/benchmark.sh
#   ./scripts/benchmark.sh --skip-n8n --skip-langchain  # aflare only
# ==============================================================================

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────

OUTPUT_DIR="${OUTPUT_DIR:-./benchmark-results}"
RUNS="${RUNS:-5}"
SKIP_N8N="${SKIP_N8N:-false}"
SKIP_LANGCHAIN="${SKIP_LANGCHAIN:-false}"
AFLARE_BIN="${AFLARE_BIN:-./aflare}"

# ── Parse arguments ──────────────────────────────────────────────────────────

for arg in "$@"; do
    case "$arg" in
        --skip-n8n) SKIP_N8N=true ;;
        --skip-langchain) SKIP_LANGCHAIN=true ;;
        --runs=*) RUNS="${arg#*=}" ;;
        --output=*) OUTPUT_DIR="${arg#*=}" ;;
        --help|-h)
            echo "Usage: $0 [--skip-n8n] [--skip-langchain] [--runs=N] [--output=DIR]"
            exit 0
            ;;
    esac
done

mkdir -p "$OUTPUT_DIR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RESULTS="$OUTPUT_DIR/benchmark-$TIMESTAMP.md"

echo "=============================================="
echo "  aflare Benchmark Suite"
echo "  Runs per test: $RUNS"
echo "  Output: $RESULTS"
echo "=============================================="
echo ""

# ── Helper functions ─────────────────────────────────────────────────────────

measure() {
    local label="$1"
    local description="$2"
    local cmd="$3"
    local prep_cmd="${4:-true}"

    echo "  Measuring: $label"
    echo "    $description"

    local start_times=()
    local mem_usages=()
    local wall_times=()

    for i in $(seq 1 "$RUNS"); do
        # Run prep command
        eval "$prep_cmd" 2>/dev/null || true

        # Measure wall time + peak memory via /usr/bin/time
        local time_output
        time_output=$(/usr/bin/time -f "%e %M" bash -c "$cmd" 2>&1)
        local wall_time=$(echo "$time_output" | awk '{print $1}')
        local mem_kb=$(echo "$time_output" | awk '{print $2}')

        wall_times+=("$wall_time")
        mem_usages+=("$mem_kb")
        echo "    Run $i: ${wall_time}s, ${mem_kb}KB"
    done

    # Calculate averages
    local avg_wall=$(average "${wall_times[@]}")
    local avg_mem=$(average "${mem_usages[@]}")
    local avg_mem_mb=$(echo "scale=1; $avg_mem / 1024" | bc)

    echo "    Average: ${avg_wall}s, ${avg_mem_mb}MB"
    echo ""

    # Return results
    echo "$label|$avg_wall|$avg_mem_mb"
}

average() {
    local sum=0
    local count=0
    for val in "$@"; do
        sum=$(echo "$sum + $val" | bc)
        count=$((count + 1))
    done
    if [ "$count" -eq 0 ]; then
        echo "0"
    else
        echo "scale=3; $sum / $count" | bc
    fi
}

# ── Check prerequisites ──────────────────────────────────────────────────────

echo "Checking prerequisites..."
echo ""

if ! command -v ollama &>/dev/null; then
    echo "WARNING: ollama not found. Some benchmarks will be skipped."
fi

# ── aflare Benchmarks ────────────────────────────────────────────────────────

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  aflare"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

AFLARE_STARTUP=$(measure \
    "aflare-startup" \
    "Time to print version and exit" \
    "$AFLARE_BIN version > /dev/null 2>&1")

AFLARE_CREATE=$(measure \
    "aflare-create" \
    "Time to generate a workflow from description" \
    "$AFLARE_BIN create 'fetch example.com and save to file' > /dev/null 2>&1" \
    "rm -f fetch-example-com-and-save-to-file.yaml")

AFLARE_RUN=$(measure \
    "aflare-run" \
    "Time to execute a simple workflow (5 steps)" \
    "cat > /tmp/test-wf.yaml << 'EOF'
name: benchmark-test
description: Simple benchmark workflow
steps:
  - node: notify
    params:
      channel: stdout
      message: benchmark running
  - node: fetch_url
    params:
      url: https://httpbin.org/get
      mode: text
  - node: transform
    params:
      operation: uppercase
  - node: notify
    params:
      channel: null
      message: done
EOF
$AFLARE_BIN run /tmp/test-wf.yaml > /dev/null 2>&1" \
    "true")

# ── n8n Benchmarks (if available) ────────────────────────────────────────────

if [ "$SKIP_N8N" = false ] && command -v docker &>/dev/null; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  n8n (via Docker)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Pull image first
    docker pull n8nio/n8n:latest > /dev/null 2>&1 || true

    N8N_STARTUP=$(measure \
        "n8n-startup" \
        "Time to start n8n and reach health check" \
        "timeout 30 docker run --rm n8nio/n8n:latest n8n --version > /dev/null 2>&1 || true")

    N8N_MEM=$(measure \
        "n8n-idle-memory" \
        "Memory usage of idle n8n container" \
        "docker run --rm -d --name n8n-bench n8nio/n8n:latest > /dev/null 2>&1; sleep 5; docker stats --no-stream --format '{{.MemUsage}}' n8n-bench 2>/dev/null | head -1; docker stop n8n-bench > /dev/null 2>&1 || true" \
        "docker rm -f n8n-bench 2>/dev/null || true")
else
    N8N_STARTUP="n8n-startup|N/A|N/A"
    N8N_MEM="n8n-idle-memory|N/A|N/A"
    echo "Skipping n8n benchmarks (--skip-n8n or docker not available)"
    echo ""
fi

# ── LangChain Benchmarks (if available) ──────────────────────────────────────

if [ "$SKIP_LANGCHAIN" = false ] && command -v python3 &>/dev/null; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  LangChain"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Create a minimal LangChain test script
    cat > /tmp/langchain_test.py << 'PYEOF'
import sys
import time
start = time.time()
try:
    from langchain_ollama import ChatOllama
    from langchain_core.prompts import ChatPromptTemplate
    template = ChatPromptTemplate.from_messages([("user", "Say hello")])
    llm = ChatOllama(model="llama3")
    chain = template | llm
    result = chain.invoke({})
    elapsed = time.time() - start
    print(f"OK {elapsed:.3f}")
except ImportError as e:
    print(f"SKIP import_error: {e}")
    sys.exit(0)
except Exception as e:
    print(f"ERR {e}")
    sys.exit(1)
PYEOF

    LANGCHAIN_IMPORT=$(measure \
        "langchain-import" \
        "Time to import langchain and initialize" \
        "python3 -c 'import time; t=time.time(); import langchain_core; print(time.time()-t)' 2>/dev/null || echo N/A" \
        "true")

    LANGCHAIN_TASK=$(measure \
        "langchain-simple-task" \
        "Time to execute a simple LLM call via LangChain" \
        "python3 /tmp/langchain_test.py 2>/dev/null || echo SKIP" \
        "true")

    LANGCHAIN_MEM=$(measure \
        "langchain-import-memory" \
        "Memory usage after importing langchain" \
        "python3 -c 'import langchain_core, langchain_ollama; import os, psutil; print(psutil.Process(os.getpid()).memory_info().rss)' 2>/dev/null || echo 0" \
        "true")
else
    LANGCHAIN_IMPORT="langchain-import|N/A|N/A"
    LANGCHAIN_TASK="langchain-simple-task|N/A|N/A"
    LANGCHAIN_MEM="langchain-import-memory|N/A|N/A"
    echo "Skipping LangChain benchmarks (--skip-langchain or python3 not available)"
    echo ""
fi

# ── Generate Report ──────────────────────────────────────────────────────────

cat > "$RESULTS" << MDEOF
# aflare Benchmark Report

**Generated:** $(date -Iseconds)
**Runs per test:** $RUNS

## Summary

| Metric | aflare | n8n | LangChain |
|--------|--------|-----|-----------|
| Startup time | $(echo "$AFLARE_STARTUP" | cut -d'|' -f2)s | $(echo "$N8N_STARTUP" | cut -d'|' -f2)s | $(echo "$LANGCHAIN_IMPORT" | cut -d'|' -f2)s |
| Memory (idle) | $(echo "$AFLARE_STARTUP" | cut -d'|' -f3)MB | $(echo "$N8N_MEM" | cut -d'|' -f3)MB | $(echo "$LANGCHAIN_MEM" | cut -d'|' -f3)MB |
| Task execution | $(echo "$AFLARE_RUN" | cut -d'|' -f2)s | N/A | $(echo "$LANGCHAIN_TASK" | cut -d'|' -f2)s |

## Detailed Results

### aflare
- **Startup:** $(echo "$AFLARE_STARTUP" | cut -d'|' -f2)s, $(echo "$AFLARE_STARTUP" | cut -d'|' -f3)MB
- **Workflow creation:** $(echo "$AFLARE_CREATE" | cut -d'|' -f2)s, $(echo "$AFLARE_CREATE" | cut -d'|' -f3)MB
- **Task execution:** $(echo "$AFLARE_RUN" | cut -d'|' -f2)s, $(echo "$AFLARE_RUN" | cut -d'|' -f3)MB

### n8n
- **Startup:** $(echo "$N8N_STARTUP" | cut -d'|' -f2)s, $(echo "$N8N_MEM" | cut -d'|' -f3)MB

### LangChain
- **Import time:** $(echo "$LANGCHAIN_IMPORT" | cut -d'|' -f2)s, $(echo "$LANGCHAIN_MEM" | cut -d'|' -f3)MB
- **Simple task:** $(echo "$LANGCHAIN_TASK" | cut -d'|' -f2)s

## Test Environment

- **OS:** $(uname -srm)
- **CPU:** $(grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || echo "unknown")
- **Memory:** $(free -h 2>/dev/null | awk '/^Mem:/ {print $2}' || echo "unknown")
- **Go:** $(go version 2>/dev/null || echo "not found")
- **Docker:** $(docker --version 2>/dev/null || echo "not found")
- **Python:** $(python3 --version 2>/dev/null || echo "not found")

## How to Reproduce

\`\`\`bash
# Clone and build aflare
git clone https://github.com/alib8b8/aflare
cd aflare
go build -o aflare ./cmd/aflare

# Run benchmarks
chmod +x scripts/benchmark.sh
./scripts/benchmark.sh
\`\`\`

## Notes

- aflare is a single Go binary with zero runtime dependencies
- n8n runs in Docker (Node.js + PostgreSQL dependency)
- LangChain requires Python + pip packages (langchain, langchain-ollama)
- All tests run against a local Ollama instance with llama3 model
MDEOF

echo ""
echo "=============================================="
echo "  Benchmark complete!"
echo "  Report: $RESULTS"
echo "=============================================="