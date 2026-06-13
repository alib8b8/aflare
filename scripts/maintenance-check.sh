#!/bin/bash
# llm-box Auto-Maintenance Check Script
# Runs every 7 days via scheduled task

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="$PROJECT_DIR/maintenance_report_$(date +%Y%m%d_%H%M%S).txt"

cd "$PROJECT_DIR"

echo "================================================" | tee "$LOG_FILE"
echo "  llm-box Auto-Maintenance Report" | tee -a "$LOG_FILE"
echo "  Date: $(date)" | tee -a "$LOG_FILE"
echo "================================================" | tee -a "$LOG_FILE"

echo "" | tee -a "$LOG_FILE"
echo "[1] Project Structure Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

DIRS_OK=true
for dir in "cmd/llm-box" "internal/nodes" "internal/workflow" "examples" "docs"; do
    if [ -d "$dir" ]; then
        echo "  ✓ $dir/ exists" | tee -a "$LOG_FILE"
    else
        echo "  ✗ $dir/ MISSING" | tee -a "$LOG_FILE"
        DIRS_OK=false
    fi
done

echo "" | tee -a "$LOG_FILE"
echo "[2] Go Build Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

if command -v go &> /dev/null; then
    BUILD_OUTPUT=$(go build -o /dev/null ./cmd/llm-box 2>&1)
    if [ $? -eq 0 ]; then
        echo "  ✓ Build succeeds" | tee -a "$LOG_FILE"
    else
        echo "  ✗ Build FAILED" | tee -a "$LOG_FILE"
        echo "$BUILD_OUTPUT" | tee -a "$LOG_FILE"
    fi
else
    echo "  ⚠ Go not found, skipping build" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "[3] Go Vet Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

if command -v go &> /dev/null; then
    VET_OUTPUT=$(go vet ./... 2>&1)
    if [ $? -eq 0 ]; then
        echo "  ✓ Vet clean, no issues" | tee -a "$LOG_FILE"
    else
        echo "  ✗ Vet found issues" | tee -a "$LOG_FILE"
        echo "$VET_OUTPUT" | tee -a "$LOG_FILE"
    fi
else
    echo "  ⚠ Go not found, skipping vet" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "[4] Node Registry Integrity Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

# Registry files (node.go, types.go, parser.go etc are registry infrastructure, not nodes)
REGISTRY_FILES="node.go\|types.go\|parser.go\|executor.go\|generator.go"

# Count built-in nodes
NODE_FILES=$(find internal/nodes -name "*.go" -not -name "*_test.go" | sort)
NODE_COUNT=$(echo "$NODE_FILES" | wc -l)
echo "  Found $NODE_COUNT node files:" | tee -a "$LOG_FILE"
for f in $NODE_FILES; do
    fname=$(basename "$f")
    # Check if this is a registry/infrastructure file (not a node implementation)
    if echo "$fname" | grep -q "^node.go$"; then
        echo "    ℹ $f (registry/infra, no init() needed)" | tee -a "$LOG_FILE"
    elif grep -q "func init()" "$f" 2>/dev/null; then
        echo "    ✓ $f (has init())" | tee -a "$LOG_FILE"
    else
        echo "    ⚠ $f (no init() registration)" | tee -a "$LOG_FILE"
    fi
done

echo "" | tee -a "$LOG_FILE"
echo "[5] Example Workflow Validation" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

# Build list of all registered node names
ALL_NODES=""
# Search for "return \"node_name\"" pattern in Name() functions in Go
for gofile in $(find internal/nodes -name "*.go" -not -name "*_test.go"); do
    found=$(grep -oP 'return\s+"[^"]+"' "$gofile" 2>/dev/null | grep -v "metadata" | head -1 | sed 's/return "//;s/"$//')
    if [ -n "$found" ]; then
        ALL_NODES="$ALL_NODES $found"
    fi
done
# Add external node names from metadata.yaml files
for metafile in $(find nodes -name "metadata.yaml" 2>/dev/null); do
    node_name=$(grep "^name:" "$metafile" | head -1 | sed 's/^name:\s*//' | tr -d '"')
    if [ -n "$node_name" ]; then
        ALL_NODES="$ALL_NODES $node_name"
    fi
done

echo "  Known nodes: $(echo $ALL_NODES | xargs)" | tee -a "$LOG_FILE"

EXAMPLES_OK=true
for yaml_file in examples/*.yaml; do
    if [ -f "$yaml_file" ]; then
        while IFS= read -r line; do
            node_name=$(echo "$line" | sed 's/.*node:\s*//' | tr -d '"' | xargs)
            if [ -n "$node_name" ]; then
                if echo "$ALL_NODES" | grep -qw "$node_name"; then
                    :
                else
                    echo "  ⚠ $yaml_file: node '$node_name' may not be registered" | tee -a "$LOG_FILE"
                    EXAMPLES_OK=false
                fi
            fi
        done < <(grep "^\s*- node:" "$yaml_file")
    fi
done

if $EXAMPLES_OK; then
    echo "  ✓ All example workflows reference valid nodes" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "[6] Git Status Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

if command -v git &> /dev/null; then
    STATUS=$(git status --porcelain 2>/dev/null)
    LAST_COMMIT=$(git log -1 --oneline 2>/dev/null)
    LAST_COMMIT_DATE=$(git log -1 --format="%cd" --date=short 2>/dev/null)
    if [ -z "$STATUS" ]; then
        echo "  ✓ Working tree clean" | tee -a "$LOG_FILE"
    else
        echo "  ⚠ Uncommitted changes:" | tee -a "$LOG_FILE"
        echo "$STATUS" | while read line; do
            echo "    $line" | tee -a "$LOG_FILE"
        done
    fi
    echo "  Last commit: $LAST_COMMIT" | tee -a "$LOG_FILE"
    echo "  Last commit date: $LAST_COMMIT_DATE" | tee -a "$LOG_FILE"
else
    echo "  ⚠ git not found" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "[7] File Size Summary" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

echo "  Top 10 largest files:" | tee -a "$LOG_FILE"
find . -not -path '*/\.*' -type f -exec du -h {} + 2>/dev/null | sort -rh | head -10 | while read size path; do
    echo "    $size  $path" | tee -a "$LOG_FILE"
done

echo "" | tee -a "$LOG_FILE"
echo "[8] Line Count Summary" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

if command -v wc &> /dev/null; then
    GO_LINES=$(find . -name "*.go" -not -path '*/\.*' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
    YAML_LINES=$(find . -name "*.yaml" -not -path '*/\.*' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
    MD_LINES=$(find . -name "*.md" -not -path '*/\.*' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
    SH_LINES=$(find . -name "*.sh" -not -path '*/\.*' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
    echo "  Go lines:     $GO_LINES" | tee -a "$LOG_FILE"
    echo "  YAML lines:   $YAML_LINES" | tee -a "$LOG_FILE"
    echo "  Markdown:     $MD_LINES" | tee -a "$LOG_FILE"
    echo "  Shell:        $SH_LINES" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "[9] Dependency Check" | tee -a "$LOG_FILE"
echo "------------------------------------------------" | tee -a "$LOG_FILE"

if [ -f go.mod ]; then
    DEPS=$(grep -c "^require " go.mod)
    DIRECT_DEPS=$(grep -c "^\s*github" go.mod)
    GO_VERSION=$(grep "^go " go.mod | head -1)
    echo "  go.mod version: $GO_VERSION" | tee -a "$LOG_FILE"
    echo "  require blocks: $DEPS" | tee -a "$LOG_FILE"
fi

echo "" | tee -a "$LOG_FILE"
echo "================================================" | tee -a "$LOG_FILE"
echo "  Maintenance Check Complete" | tee -a "$LOG_FILE"
echo "  Report saved to: $(basename "$LOG_FILE")" | tee -a "$LOG_FILE"
echo "================================================" | tee -a "$LOG_FILE"

echo ""
echo "--- QUICK SUMMARY ---"
grep -E "(✓|✗|⚠|ℹ|FAILED)" "$LOG_FILE"
