#!/usr/bin/env bash
set -e

AFLARE_NAME="aflare"
AFLARE_VERSION="${AFLARE_TAG:-latest}"
GITHUB_REPO="alib8b8/aflare"
ATOMGIT_REPO="aflare/aflare"
GITCODE_REPO="aflare/aflare"

AFLARE_HOME="${AFLARE_HOME:-$HOME/aflare}"
AFLARE_DATA="${AFLARE_DATA:-$HOME/.aflare}"

MIRROR="${AFLARE_MIRROR:-}"
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
    local asset_name="${AFLARE_NAME}_${AFLARE_VERSION}_${os}_${arch}.tar.gz"

    if [ -n "$MIRROR" ]; then
        echo "${MIRROR%/}/$asset_name"
        return
    fi

    if [ "$USE_ATOMGIT" -eq 1 ]; then
        if [ "$AFLARE_VERSION" = "latest" ]; then
            local tag
            tag=$(get_latest_tag "https://atomgit.com/api/v1/repos/$ATOMGIT_REPO/releases/latest")
            if [ -n "$tag" ]; then
                AFLARE_VERSION="$tag"
                asset_name="${AFLARE_NAME}_${AFLARE_VERSION}_${os}_${arch}.tar.gz"
            fi
        fi
        echo "https://atomgit.com/$ATOMGIT_REPO/-/archive/$AFLARE_VERSION/$asset_name"
    else
        if [ "$AFLARE_VERSION" = "latest" ]; then
            local tag
            tag=$(get_latest_tag "https://api.github.com/repos/$GITHUB_REPO/releases/latest")
            if [ -n "$tag" ]; then
                AFLARE_VERSION="$tag"
                asset_name="${AFLARE_NAME}_${AFLARE_VERSION}_${os}_${arch}.tar.gz"
            fi
        fi
        echo "https://github.com/$GITHUB_REPO/releases/download/$AFLARE_VERSION/$asset_name"
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
    log_info "Building aflare..."
    go build -o "$AFLARE_HOME/bin/aflare" ./cmd/aflare

    cp -r templates "$AFLARE_HOME/" 2>/dev/null || true
    cp README* "$AFLARE_HOME/" 2>/dev/null || true
    cp LICENSE "$AFLARE_HOME/" 2>/dev/null || true

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

    local path_line='export PATH="'"$AFLARE_HOME"'/bin:$PATH"'
    local home_line='export AFLARE_HOME="'"$AFLARE_HOME"'"'
    local data_line='export AFLARE_DATA="'"$AFLARE_DATA"'"'

    if [ -n "$shell_rc" ] && [ -f "$shell_rc" ]; then
        if ! grep -q "AFLARE_HOME" "$shell_rc" 2>/dev/null; then
            log_info "Adding environment variables to $shell_rc"
            {
                echo ""
                echo "# aflare"
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

Install aflare - the AI Agent workflow toolkit.

Options:
  --atomgit            Use AtomGit mirror (faster for China mainland users)
  --mirror <url>       Use custom download mirror
  --home <path>        Set program directory (default: \$HOME/aflare)
  --data <path>        Set data directory (default: \$HOME/.aflare)
  --version <tag>      Install specific version (default: latest)
  --force              Force reinstall even if already installed
  -h, --help           Show this help message

Environment Variables:
  AFLARE_HOME         Program installation directory
  AFLARE_DATA         User data directory
  AFLARE_MIRROR       Custom download mirror
  AFLARE_TAG          Specific version tag to install

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
        --home)    AFLARE_HOME="$2"; shift 2 ;;
        --data)    AFLARE_DATA="$2"; shift 2 ;;
        --version) AFLARE_VERSION="$2"; AFLARE_TAG="$2"; shift 2 ;;
        --force)   FORCE=1; shift ;;
        -h|--help) show_help; exit 0 ;;
        *) log_warn "Unknown option: $1"; shift ;;
    esac
done

echo "╔══════════════════════════════════════════╗"
echo "║           aflare Installer               ║"
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
log_info "Program dir: $AFLARE_HOME"
log_info "Data dir: $AFLARE_DATA"

if [ "$USE_ATOMGIT" -eq 1 ]; then
    log_info "Mirror: AtomGit (China optimized)"
fi

if [ -d "$AFLARE_HOME/bin" ] && [ "$FORCE" -eq 0 ]; then
    log_warn "aflare already installed at $AFLARE_HOME"
    log_warn "Use --force to reinstall"
    exit 0
fi

mkdir -p "$AFLARE_HOME/bin"
mkdir -p "$AFLARE_DATA"

DOWNLOAD_URL=$(resolve_download_url "$OS" "$ARCH")
TMP_FILE=$(mktemp).tar.gz

log_info "Downloading aflare $AFLARE_VERSION..."
if download_file "$DOWNLOAD_URL" "$TMP_FILE"; then
    log_ok "Download complete"
    log_info "Extracting..."
    tar -xzf "$TMP_FILE" -C "$AFLARE_HOME" --strip-components=1 2>/dev/null || {
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

if [ ! -f "$AFLARE_HOME/bin/aflare" ]; then
    log_error "Installation failed: aflare binary not found"
    exit 1
fi

chmod +x "$AFLARE_HOME/bin/aflare"

setup_env

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║      ✅ aflare installed successfully!   ║"
echo "╚══════════════════════════════════════════╝"
echo ""
log_ok "Program: $AFLARE_HOME"
log_ok "Data:    $AFLARE_DATA"
echo ""
log_info "Quick start:"
echo "  aflare skills list"
echo "  aflare run <template>"
echo "  aflare init --mcp all"
echo ""
