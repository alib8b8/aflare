#!/usr/bin/env bash
# install-skill.sh — Install llm-box workflow skill into TRAE global skill directories
#
# Installs the "llm-box-workflow" skill to:
#   ~/.traecli/skills/   (TRAE CLI global)
#   ~/.trae-cn/skills/   (TRAE IDE global)
#
# Usage:
#   bash install-skill.sh           # install to both
#   bash install-skill.sh --cli     # install to TRAE CLI only
#   bash install-skill.sh --ide     # install to TRAE IDE only
#   bash install-skill.sh --remove  # remove from both

set -euo pipefail

SKILL_NAME="llm-box-workflow"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_SRC="$SCRIPT_DIR/.traecli/skills/$SKILL_NAME"

CLI_DEST="$HOME/.traecli/skills/$SKILL_NAME"
IDE_DEST="$HOME/.trae-cn/skills/$SKILL_NAME"

# --- helpers ----------------------------------------------------------------

info()  { printf "\033[1;34m[info]\033[0m  %s\n" "$*"; }
ok()    { printf "\033[1;32m[ok]\033[0m    %s\n" "$*"; }
warn()  { printf "\033[1;33m[warn]\033[0m  %s\n" "$*"; }
error() { printf "\033[1;31m[err]\033[0m   %s\n" "$*" >&2; }

# --- pre-flight check -------------------------------------------------------

if [ ! -f "$SKILL_SRC/SKILL.md" ]; then
  error "Skill source not found at: $SKILL_SRC/SKILL.md"
  error "Make sure you run this script from the llm-box repository root."
  exit 1
fi

# --- parse args -------------------------------------------------------------

TARGET="both"
ACTION="install"

while [ $# -gt 0 ]; do
  case "$1" in
    --cli)    TARGET="cli" ;;
    --ide)    TARGET="ide" ;;
    --remove) ACTION="remove" ;;
    --help|-h)
      cat <<EOF
install-skill.sh — Install llm-box TRAE skill

Usage:
  bash install-skill.sh [options]

Options:
  (no option)  Install skill to both TRAE CLI and TRAE IDE global dirs
  --cli        Install to TRAE CLI only (~/.traecli/skills/)
  --ide        Install to TRAE IDE only (~/.trae-cn/skills/)
  --remove     Remove the skill from global dirs
  --help       Show this help

The skill files are located at:
  .traecli/skills/llm-box-workflow/
EOF
      exit 0
      ;;
    *)
      error "Unknown option: $1"
      exit 1
      ;;
  esac
  shift
done

# --- remove action ----------------------------------------------------------

if [ "$ACTION" = "remove" ]; then
  info "Removing '$SKILL_NAME' skill..."
  for dest in "$CLI_DEST" "$IDE_DEST"; do
    if [ -d "$dest" ]; then
      rm -rf "$dest"
      ok "Removed: $dest"
    else
      warn "Not found (skip): $dest"
    fi
  done
  ok "Done. Restart TRAE CLI / TRAE IDE to take effect."
  exit 0
fi

# --- install action ---------------------------------------------------------

info "Installing '$SKILL_NAME' skill..."

install_to() {
  local dest="$1"
  local label="$2"

  # Create parent directory
  mkdir -p "$(dirname "$dest")"

  # Remove old version if exists
  if [ -d "$dest" ]; then
    rm -rf "$dest"
  fi

  # Copy skill files
  cp -r "$SKILL_SRC" "$dest"

  # Verify
  if [ -f "$dest/SKILL.md" ]; then
    ok "Installed to $label: $dest"
  else
    error "Failed to install to: $dest"
    exit 1
  fi
}

case "$TARGET" in
  both)
    install_to "$CLI_DEST" "TRAE CLI"
    install_to "$IDE_DEST" "TRAE IDE"
    ;;
  cli)
    install_to "$CLI_DEST" "TRAE CLI"
    ;;
  ide)
    install_to "$IDE_DEST" "TRAE IDE"
    ;;
esac

echo ""
ok "Installation complete."
echo ""
echo "Next steps:"
echo "  1. Restart TRAE CLI or TRAE IDE to load the skill."
echo "  2. Run the /skills command in TRAE CLI to verify."
echo "  3. Try a trigger phrase, e.g.:"
echo "     \"fetch example.com and save to file\""
echo "     \"create a workflow to summarize this article with LLM\""
