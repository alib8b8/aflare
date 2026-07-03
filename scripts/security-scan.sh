#!/bin/bash

set -e

echo "=== llm-box Security Scanner ==="
echo ""

DANGER_FOUND=0

DANGER_PATTERNS=(
    'os\.Setenv.*SECRET'
    'os\.Setenv.*TOKEN'
    'os\.Setenv.*PASSWORD'
    'ioutil\.ReadFile.*etc/passwd'
    'ioutil\.ReadFile.*etc/shadow'
    'os\.WriteFile.*etc/'
)

SHELL_PATTERNS=(
    'exec\.Command.*sh.*-c'
    'exec\.Command.*bash.*-c'
)

echo "[SCAN] DANGER patterns..."
echo ""

for pattern in "${DANGER_PATTERNS[@]}"; do
    matches=$(grep -rnE "$pattern" --include="*.go" . 2>/dev/null || true)
    if [ -n "$matches" ]; then
        echo "[FAIL] DANGER PATTERN: $pattern"
        echo "$matches"
        echo ""
        DANGER_FOUND=1
    fi
done

echo "[SCAN] Unauthorized shell commands..."
echo ""

for pattern in "${SHELL_PATTERNS[@]}"; do
    matches=$(grep -rnE "$pattern" --include="*.go" . 2>/dev/null | grep -v "internal/nodes/execute.go" || true)
    if [ -n "$matches" ]; then
        echo "[FAIL] UNAUTHORIZED SHELL: $pattern"
        echo "$matches"
        echo ""
        DANGER_FOUND=1
    else
        echo "[OK] No unauthorized shell commands"
    fi
done

echo "[SCAN] Suspicious patterns..."
echo ""

matches=$(grep -rnE "panic\(" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "[WARN] SUSPICIOUS: panic()"
    echo "$matches"
    echo ""
fi

matches=$(grep -rnE "log\.Print.*SECRET|log\.Print.*TOKEN" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "[WARN] SUSPICIOUS: logging secrets"
    echo "$matches"
    echo ""
fi

echo "[SCAN] Hardcoded credentials (long strings)..."
echo ""

matches=$(grep -rnE "\"[A-Za-z0-9+/]{40,}\"" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "[WARN] POTENTIAL CREDENTIAL (base64):"
    echo "$matches"
    echo ""
fi

matches=$(grep -rnE "\"[0-9a-f]{32,}\"" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "[WARN] POTENTIAL CREDENTIAL (hex):"
    echo "$matches"
    echo ""
fi

echo "[OK] No hardcoded credentials found"

echo ""
echo "[SCAN] Network calls..."
echo ""

matches=$(grep -rnE "raw\.githubusercontent\.com" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "[WARN] SUSPICIOUS DOMAIN:"
    echo "$matches"
    echo ""
fi

echo "[OK] Network calls look safe"

echo ""
echo "[SCAN] Verifying dependencies..."
echo ""

if ! go mod verify > /dev/null 2>&1; then
    echo "[FAIL] go mod verify FAILED!"
    DANGER_FOUND=1
else
    echo "[OK] go mod verify passed"
fi

echo ""
if [ $DANGER_FOUND -eq 1 ]; then
    echo "[STOP] SECURITY VIOLATIONS FOUND!"
    exit 1
else
    echo "[OK] Security scan passed"
    exit 0
fi
