#!/bin/bash
set -e

BINARY_NAME="aflare"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
REPO="alib8b8/aflare"
GITCODE_REPO="aflare/aflare"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; }

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $ARCH in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) 
            error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case $OS in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        *) 
            error "Unsupported OS: $OS"
            exit 1
            ;;
    esac
}

check_command() {
    command -v "$1" >/dev/null 2>&1
}

detect_region() {
    if check_command curl; then
        local result=$(curl -s --connect-timeout 3 https://ipapi.co/country/ 2>/dev/null || echo "")
        if [ "$result" = "CN" ]; then
            echo "cn"
            return
        fi
    fi
    echo "global"
}

get_latest_release() {
    local region="$1"
    
    if [ "$region" = "cn" ]; then
        local gitcode_api="https://gitcode.com/api/v5/repos/$GITCODE_REPO/releases/latest"
        local tag=$(curl -s --connect-timeout 5 "$gitcode_api" 2>/dev/null | grep -o '"tag_name":"[^"]*"' | head -1 | sed 's/"tag_name":"//;s/"//')
        if [ -n "$tag" ] && echo "$tag" | grep -qE '^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9]+)?$'; then
            echo "$tag"
            return 0
        fi
        warn "GitCode release API failed, trying GitHub mirror..."
    fi
    
    local github_api="https://api.github.com/repos/$REPO/releases/latest"
    local mirrors=(
        ""
        "https://ghproxy.com/"
        "https://gh.api.99988866.xyz/"
    )
    
    for mirror in "${mirrors[@]}"; do
        local url="${mirror}${github_api}"
        local tag=$(curl -s --connect-timeout 10 "$url" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -n "$tag" ] && [ "$tag" != "null" ] && echo "$tag" | grep -qE '^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9]+)?$'; then
            echo "$tag"
            return 0
        fi
    done
    
    return 1
}

download_file() {
    local url="$1"
    local output="$2"
    local region="$3"
    
    local mirrors=()
    if [ "$region" = "cn" ]; then
        mirrors=(
            "https://ghproxy.com/"
            "https://gh.api.99988866.xyz/"
            ""
        )
    else
        mirrors=("")
    fi
    
    for mirror in "${mirrors[@]}"; do
        local final_url="${mirror}${url}"
        info "Trying: ${final_url:0:80}..."
        
        if curl -fSL --connect-timeout 15 --max-time 120 --retry 2 --retry-delay 2 -o "$output" "$final_url" 2>/dev/null; then
            local size=$(stat -c%s "$output" 2>/dev/null || stat -f%z "$output" 2>/dev/null || echo 0)
            if [ "$size" -gt 1000000 ]; then
                success "Downloaded ($((size / 1024 / 1024)) MB)"
                return 0
            else
                warn "File too small ($size bytes), trying next mirror..."
                rm -f "$output"
            fi
        fi
    done
    
    return 1
}

main() {
    # --offline <archive>: install from a pre-downloaded release tarball
    # without any network access. Intended for air-gapped/intranet hosts
    # where the GitHub/GitCode release download is unavailable. The user
    # transfers the tarball (e.g. aflare-linux-amd64.tar.gz) onto the host
    # and runs: bash install.sh --offline aflare-linux-amd64.tar.gz
    if [ "${1:-}" = "--offline" ] || [ "${1:-}" = "-o" ]; then
        local archive="${2:-}"
        if [ -z "$archive" ]; then
            error "--offline 需要指定本地安装包路径"
            echo "用法: bash install.sh --offline <aflare-linux-amd64.tar.gz>"
            echo "  请先从有网的机器下载发布包，再传输到本机："
            echo "  GitHub:  https://github.com/$REPO/releases"
            echo "  GitCode: https://gitcode.com/$GITCODE_REPO/-/releases"
            exit 1
        fi
        if [ ! -f "$archive" ]; then
            error "安装包不存在: $archive"
            exit 1
        fi
        local rc=0
        install_from_archive "$archive" || rc=$?
        return $rc
    fi

    echo ""
    echo "╔══════════════════════════════════════════╗"
    echo "║          aflare 安装向导                ║"
    echo "║   AI Workflow Engine Installer          ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""

    detect_platform
    info "检测系统: $OS / $ARCH"

    info "检测网络环境..."
    REGION=$(detect_region)
    if [ "$REGION" = "cn" ]; then
        success "检测到国内网络，将使用镜像加速下载"
    else
        info "检测到国际网络环境"
    fi

    info "获取最新版本..."
    VERSION=$(get_latest_release "$REGION")
    if [ -z "$VERSION" ]; then
        error "无法获取最新版本信息"
        echo ""
        echo "手动下载地址："
        echo "  GitHub:  https://github.com/$REPO/releases"
        echo "  GitCode: https://gitcode.com/$GITCODE_REPO/-/releases"
        echo ""
        echo "离线安装：先下载发布包到本机，再运行："
        echo "  bash install.sh --offline aflare-\$OS-\$ARCH.tar.gz"
        exit 1
    fi
    success "最新版本: $VERSION"

    ARCHIVE_NAME="$BINARY_NAME-$OS-$ARCH.tar.gz"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE_NAME"
    CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    info "下载 $ARCHIVE_NAME..."
    if ! download_file "$DOWNLOAD_URL" "$ARCHIVE_NAME" "$REGION"; then
        error "下载失败"
        echo ""
        echo "请尝试手动下载："
        echo "  GitHub:  $DOWNLOAD_URL"
        echo "  镜像加速: https://ghproxy.com/$DOWNLOAD_URL"
        echo ""
        echo "离线安装：下载后运行 bash install.sh --offline $ARCHIVE_NAME"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    info "下载校验和..."
    if download_file "$CHECKSUMS_URL" "checksums.txt" "$REGION"; then
        info "校验文件完整性..."
        if echo "$(grep "$ARCHIVE_NAME" checksums.txt | awk '{print $1}')  $ARCHIVE_NAME" | sha256sum --check --status 2>/dev/null; then
            success "校验通过"
        else
            warn "校验失败，可能文件已损坏，是否继续？(y/N)"
            read -r answer
            if [ "$answer" != "y" ] && [ "$answer" != "Y" ]; then
                rm -rf "$TMP_DIR"
                exit 1
            fi
        fi
    else
        warn "无法下载校验文件，跳过校验"
    fi

    local rc=0
    install_from_archive "$ARCHIVE_NAME" || rc=$?
    rm -rf "$TMP_DIR"
    return $rc
}

# install_from_archive extracts a release tarball (which must contain the
# aflare binary at its root, matching goreleaser output) and installs it to
# INSTALL_DIR. Shared by both the online and --offline paths so the
# install/permission logic is identical. Caller is responsible for the
# working directory containing the archive; on success the binary is moved
# out of the CWD so the caller's rm -rf TMP_DIR is safe.
install_from_archive() {
    local archive="$1"

    info "解压..."
    tar -xzf "$archive"

    if [ ! -f "$BINARY_NAME" ]; then
        error "解压后未找到 $BINARY_NAME"
        return 1
    fi

    chmod +x "$BINARY_NAME"

    info "安装到 $INSTALL_DIR..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        success "安装成功！"
    else
        if check_command sudo && [ -t 0 ]; then
            info "需要 sudo 权限安装到 $INSTALL_DIR"
            sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
            success "安装成功！"
        else
            warn "无法写入 $INSTALL_DIR，安装到当前目录"
            mv "$BINARY_NAME" "$OLDPWD/$BINARY_NAME"
            cd "$OLDPWD"
            success "文件已保存到 ./$BINARY_NAME"
            echo ""
            echo "请手动添加到 PATH："
            echo "  export PATH=\"\$PATH:$(pwd)\""
            print_post_install_hints
            return 0
        fi
    fi

    print_post_install_hints
    return 0
}

print_post_install_hints() {
    echo ""
    echo "╔══════════════════════════════════════════╗"
    echo "║           安装完成！🎉                   ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""
    echo "快速开始："
    echo "  $BINARY_NAME doctor                          # 环境自检（零配置可跑）"
    echo "  $BINARY_NAME doctor --offline                # 离线/内网自检（跳过外网探测）"
    echo "  $BINARY_NAME run examples/content-processor.yaml  # 零配置示例"
    echo "  $BINARY_NAME init                            # 配置 LLM（交互式向导）"
    echo "  $BINARY_NAME create \"Summarize today's AI news\"   # 关键词匹配，无需 LLM"
    echo "  $BINARY_NAME chat                            # 需先 aflare init"
    echo ""
    echo "更多文档：https://gitcode.com/$GITCODE_REPO"
    echo ""
}

main "$@"