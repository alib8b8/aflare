#!/usr/bin/env bash
# install.sh — install a prebuilt aflare binary from GitHub releases into
# the runner environment. Non-interactive variant of the repo's install.sh:
# no mirrors, no prompts, checksum always enforced, installs into a
# runner-local bin dir (no sudo) and prepends it to $GITHUB_PATH.
set -euo pipefail

REPO="alib8b8/aflare"
VERSION="${AFLARE_VERSION:-latest}"
INSTALL_DIR="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/aflare-action-bin"

log()  { echo "[aflare-action] $*"; }
fail() { echo "[aflare-action] ERROR: $*" >&2; exit 1; }

# --- platform detection -----------------------------------------------------
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  loongarch64)   ARCH="loong64" ;;
  *) fail "unsupported architecture: $ARCH" ;;
esac
case "$OS" in
  linux|darwin) : ;;
  *) fail "unsupported OS: $OS (aflare releases cover linux/darwin/windows)" ;;
esac

# --- version resolution -----------------------------------------------------
if [[ "$VERSION" == "latest" ]]; then
  AUTH=()
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    AUTH=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  VERSION="$(curl -fsSL --connect-timeout 15 "${AUTH[@]}" \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')" \
    || fail "could not resolve latest release (API rate limit? pin 'version:' or pass a 'token:')"
  [[ -n "$VERSION" && "$VERSION" != "null" ]] || fail "latest release lookup returned empty"
fi
log "installing aflare ${VERSION} (${OS}/${ARCH})"

# --- download + checksum ----------------------------------------------------
ARCHIVE="aflare-${OS}-${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

log "downloading ${ARCHIVE}..."
curl -fsSL --connect-timeout 15 --max-time 180 --retry 2 \
  -o "${TMP_DIR}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" \
  || fail "download failed: ${BASE_URL}/${ARCHIVE}"

curl -fsSL --connect-timeout 15 --max-time 60 \
  -o "${TMP_DIR}/checksums.txt" "${BASE_URL}/checksums.txt" \
  || fail "checksums.txt download failed (release ${VERSION} incomplete?)"

EXPECTED="$(grep " ${ARCHIVE}\$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"
[[ -n "$EXPECTED" ]] || fail "no checksum entry for ${ARCHIVE} in checksums.txt"
echo "${EXPECTED}  ${TMP_DIR}/${ARCHIVE}" | sha256sum --check --strict - \
  || fail "checksum mismatch for ${ARCHIVE}"
log "checksum verified (sha256)"

# --- extract + install ------------------------------------------------------
mkdir -p "$INSTALL_DIR"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$INSTALL_DIR"
[[ -x "${INSTALL_DIR}/aflare" ]] || fail "archive did not contain an aflare binary"
echo "$INSTALL_DIR" >> "$GITHUB_PATH"

INSTALLED_VERSION="$("${INSTALL_DIR}/aflare" version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+[^ ]*' | head -1 || true)"
echo "version=${INSTALLED_VERSION:-${VERSION}}" >> "$GITHUB_OUTPUT"
log "installed $(${INSTALL_DIR}/aflare version 2>/dev/null | head -1 || echo "$VERSION") to ${INSTALL_DIR}"
