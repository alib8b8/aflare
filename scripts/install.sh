#!/usr/bin/env bash
set -e

LLM_BOX_NAME="llm-box"
LLM_BOX_VERSION="${LLM_BOX_TAG:-latest}"
GITHUB_REPO="alib8b8/llm-box"
ATOMGIT_REPO="llm-box/llm-box"
GITCODE_REPO="llm-box/llm-box"

LLM_BOX_HOME="${LLM_BOX_HOME:-$HOME/llm-box}"
LLM_BOX_DATA="${LLM_BOX_DATA:-$HOME/.llm-box}"

MIRROR="${LLM_BOX_MIRROR:-}"
USE_ATOMGIT=0
FORCE=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

detect_os() {
    case "$(uname -s)" in
        Linux)   echo "linux" ;;
        Darwin)  echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)       echo "unknown" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "unknown" ;;
    esac
}

get_latest_tag() {
    local repo_url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -sL "$repo_url" 2>/dev/null | grep -o '"tag_name":"[^"]*"' | head -1 | sed 's/"tag_name":"//;s/"//'
    fi
}

resolve_download_url() {
    local os="$1"
    local arch="$2"
    local asset_name="${LLM_BOX_NAME}_${LLM_BOX_VERSION}_${os}_${arch}.tar.gz"

    if [ -n "$MIRROR" ]; then
        echo "${MIRROR%/}/$asset_name"
        return
    fi

    if [ "$USE_ATOMGIT" -eq 1 ]; then
        if [ "$LLM_BOX_VERSION" = "latest" ]; then
            local tag
            tag=$(get_latest_tag "https://atomgit.com/api/v1/repos/$ATOMGIT_REPO/releases/latest")
            if [ -n "$tag" ]; then
                LLM_BOX_VERSION="$tag"
                asset_name="${LLM_BOX_NAME}_${LLM_BOX_VERSION}_${os}_${arch}.tar.gz"
            fi
        fi
        echo "https://atomgit.com/$ATOMGIT_REPO/-/archive/$LLM_BOX_VERSION/$asset_name"
    else
        if [ "$LLM_BOX_VERSION" = "latest" ]; then
            local tag
            tag=$(get_latest_tag "https://api.github.com/repos/$GITHUB_REPO/releases/latest")
            if [ -n "$tag" ]; then
                LLM_BOX_VERSION="$tag"
                asset_name="${LLM_BOX_NAME}_${LLM_BOX_VERSION}_${os}_${arch}.tar.gz"
            fi
        fi
        echo "https://github.com/$GITHUB_REPO/releases/download/$LLM_BOX_VERSION/$asset_name"
    fi
}

download_file() {
    local url="$1"
    local output="$2"

    log_info "Downloading: $url"

    if command -v curl >/dev/null 2>&1; then
        curl -fSL --progress-bar -o "$output" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --show-progress -O "$output" "$url"
    else
        log_error "Neither curl nor wget found. Please install one."
        exit 1
    fi
}

install_go_binary() {
    log_info "Building from source (Go required)..."

    if ! command -v go >/dev/null 2>&1; then
        log_error "Go not found. Please install Go 1.21+ from https://go.dev/dl/"
        exit 1
    fi

    local build_dir
    build_dir=$(mktemp -d)
    log_info "Cloning source to $build_dir..."

    local clone_url
    if [ "$USE_ATOMGIT" -eq 1 ]; then
        clone_url="https://atomgit.com/$ATOMGIT_REPO.git"
    else
        clone_url="https://github.com/$GITHUB_REPO.git"
    fi

    if ! git clone --depth 1 "$clone_url" "$build_dir" 2>/dev/null; then
        log_error "Failed to clone repository"
        exit 1
    fi

    cd "$build_dir"
    log_info "Building llm-box..."
    go build -o "$LLM_BOX_HOME/bin/llm-box" ./cmd/llm-box

    cp -r templates "$LLM_BOX_HOME/" 2>/dev/null || true
    cp README* "$LLM_BOX_HOME/" 2>/dev/null || true
    cp LICENSE "$LLM_BOX_HOME/" 2>/dev/null || true

    rm -rf "$build_dir"
}

setup_env() {
    local shell_rc=""
    case "$SHELL" in
        */bash) shell_rc="$HOME/.bashrc" ;;
        */zsh)  shell_rc="$HOME/.zshrc" ;;
        */fish) shell_rc="$HOME/.config/fish/config.fish" ;;
        *)      shell_rc="$HOME/.profile" ;;
    esac

    local path_line='export PATH="'"$LLM_BOX_HOME"'/bin:$PATH"'
    local home_line='export LLM_BOX_HOME="'"$LLM_BOX_HOME"'"'
    local data_line='export LLM_BOX_DATA="'"$LLM_BOX_DATA"'"'

    if [ -n "$shell_rc" ] && [ -f "$shell_rc" ]; then
        if ! grep -q "LLM_BOX_HOME" "$shell_rc" 2>/dev/null; then
            log_info "Adding environment variables to $shell_rc"
            {
                echo ""
                echo "# llm-box"
                echo "$path_line"
                echo "$home_line"
                echo "$data_line"
            } >> "$shell_rc"
        fi
    fi

    log_info "To activate immediately, run:"
    echo "  $path_line"
    echo "  $home_line"
    echo "  $data_line"
}

show_help() {
    cat <<EOF
Usage: $(basename "$0") [options]

Install llm-box - the AI Agent workflow toolkit.

Options:
  --atomgit            Use AtomGit mirror (faster for China mainland users)
  --mirror <url>       Use custom download mirror
  --home <path>        Set program directory (default: \$HOME/llm-box)
  --data <path>        Set data directory (default: \$HOME/.llm-box)
  --version <tag>      Install specific version (default: latest)
  --force              Force reinstall even if already installed
  -h, --help           Show this help message

Environment Variables:
  LLM_BOX_HOME         Program installation directory
  LLM_BOX_DATA         User data directory
  LLM_BOX_MIRROR       Custom download mirror
  LLM_BOX_TAG          Specific version tag to install

Examples:
  # Standard install (GitHub)
  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/master/scripts/install.sh | bash

  # China mirror (AtomGit)
  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/master/scripts/install.sh | bash -s -- --atomgit

  # Install specific version
  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/master/scripts/install.sh | bash -s -- --version v0.6.0
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --atomgit) USE_ATOMGIT=1; shift ;;
        --mirror)  MIRROR="$2"; shift 2 ;;
        --home)    LLM_BOX_HOME="$2"; shift 2 ;;
        --data)    LLM_BOX_DATA="$2"; shift 2 ;;
        --version) LLM_BOX_VERSION="$2"; LLM_BOX_TAG="$2"; shift 2 ;;
        --force)   FORCE=1; shift ;;
        -h|--help) show_help; exit 0 ;;
        *) log_warn "Unknown option: $1"; shift ;;
    esac
done

echo "╔══════════════════════════════════════════╗"
echo "║           llm-box Installer               ║"
echo "╚══════════════════════════════════════════╝"
echo ""

OS=$(detect_os)
ARCH=$(detect_arch)

if [ "$OS" = "unknown" ]; then
    log_error "Unsupported OS: $(uname -s)"
    exit 1
fi
if [ "$ARCH" = "unknown" ]; then
    log_error "Unsupported architecture: $(uname -m)"
    exit 1
fi

log_info "Detected: $OS / $ARCH"
log_info "Program dir: $LLM_BOX_HOME"
log_info "Data dir: $LLM_BOX_DATA"

if [ "$USE_ATOMGIT" -eq 1 ]; then
    log_info "Mirror: AtomGit (China optimized)"
fi

if [ -d "$LLM_BOX_HOME/bin" ] && [ "$FORCE" -eq 0 ]; then
    log_warn "llm-box already installed at $LLM_BOX_HOME"
    log_warn "Use --force to reinstall"
    exit 0
fi

mkdir -p "$LLM_BOX_HOME/bin"
mkdir -p "$LLM_BOX_DATA"

DOWNLOAD_URL=$(resolve_download_url "$OS" "$ARCH")
TMP_FILE=$(mktemp).tar.gz

log_info "Downloading llm-box $LLM_BOX_VERSION..."
if download_file "$DOWNLOAD_URL" "$TMP_FILE"; then
    log_ok "Download complete"
    log_info "Extracting..."
    tar -xzf "$TMP_FILE" -C "$LLM_BOX_HOME" --strip-components=1 2>/dev/null || {
        log_warn "Prebuilt binary not available, building from source..."
        rm -f "$TMP_FILE"
        install_go_binary
    }
    rm -f "$TMP_FILE"
else
    log_warn "Download failed, trying to build from source..."
    rm -f "$TMP_FILE"
    install_go_binary
fi

if [ ! -f "$LLM_BOX_HOME/bin/llm-box" ]; then
    log_error "Installation failed: llm-box binary not found"
    exit 1
fi

chmod +x "$LLM_BOX_HOME/bin/llm-box"

setup_env

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║      ✅ llm-box installed successfully!   ║"
echo "╚══════════════════════════════════════════╝"
echo ""
log_ok "Program: $LLM_BOX_HOME"
log_ok "Data:    $LLM_BOX_DATA"
echo ""
log_info "Quick start:"
echo "  llm-box skills list"
echo "  llm-box run <template>"
echo "  llm-box init --mcp all"
echo ""
