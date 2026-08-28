#!/usr/bin/env bash
# run.sh — execute the requested aflare workflow (or validate it), surface
# the final output as a step output, and propagate aflare's exit code so
# CI jobs gate on workflow failure.
set -euo pipefail

WORKFLOW="${AFLARE_WORKFLOW:-}"
SET_INPUT="${AFLARE_SET:-}"
SAFE_MODE="${AFLARE_SAFE_MODE:-false}"
VALIDATE_ONLY="${AFLARE_VALIDATE_ONLY:-false}"
WORKDIR="${AFLARE_WORKING_DIRECTORY:-.}"

log() { echo "[aflare-action] $*"; }

# Install-only mode: no workflow given, the CLI is on PATH for later steps.
if [[ -z "$WORKFLOW" ]]; then
  log "no 'workflow' input — CLI installed only; use 'aflare ...' in later steps"
  echo "output=" >> "$GITHUB_OUTPUT"
  exit 0
fi

cd "$WORKDIR"
[[ -f "$WORKFLOW" ]] || { echo "[aflare-action] ERROR: workflow file not found: $WORKFLOW (in $WORKDIR)" >&2; exit 1; }

# Assemble the command. --safe-mode is a global flag and must precede the
# subcommand; --set entries are consumed after the workflow path.
CMD=(aflare)
[[ "$SAFE_MODE" == "true" ]] && CMD+=(--safe-mode)

if [[ "$VALIDATE_ONLY" == "true" ]]; then
  CMD+=(validate "$WORKFLOW")
else
  CMD+=(run "$WORKFLOW")
  while IFS= read -r line; do
    [[ -z "${line// /}" ]] && continue
    CMD+=(--set "$line")
  done <<< "$SET_INPUT"
fi

log "running: ${CMD[*]}"

LOG="$(mktemp)"
set +e
"${CMD[@]}" 2>&1 | tee "$LOG"
RC="${PIPESTATUS[0]}"
set -e

# Surface the final-output block (text between the '=== Final Output ==='
# marker and the first blank line) as a step output for downstream steps.
OUTPUT="$(sed -n '/^=== Final Output ===$/,/^$/{ /^=== Final Output ===$/d; /^$/d; p; }' "$LOG")"
{
  echo "output<<AFLARE_OUTPUT_EOF"
  echo "$OUTPUT"
  echo "AFLARE_OUTPUT_EOF"
} >> "$GITHUB_OUTPUT"

# Step summary: a compact run record visible directly on the PR/run page.
{
  echo "### aflare"
  echo ""
  echo "- command: \`${CMD[*]}\`"
  echo "- exit code: ${RC}"
  if [[ -n "$OUTPUT" ]]; then
    echo "- final output:"
    echo ""
    echo '```'
    echo "$OUTPUT"
    echo '```'
  fi
} >> "$GITHUB_STEP_SUMMARY"

exit "$RC"
