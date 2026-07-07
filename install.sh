#!/bin/bash

set -e

# llm-box installation script

REPO="alib8b8/llm-box"
BINARY_NAME="llm-box"
INSTALL_DIR="/usr/local/bin"

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $ARCH in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) 
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case $OS in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        *) 
            echo "Unsupported OS: $OS"
            exit 1
            ;;
    esac
}

download_binary() {
    local archive_ext="tar.gz"
    local archive_name="$BINARY_NAME-$OS-$ARCH.$archive_ext"
    if [ "$OS" = "windows" ]; then
        archive_ext="zip"
        archive_name="$BINARY_NAME-$OS-$ARCH.$archive_ext"
    fi

    echo "Downloading $archive_name..."

    LATEST_RELEASE=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    echo "Latest release: $LATEST_RELEASE"

    URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$archive_name"
    CHECKSUM_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/checksums.txt"

    if ! curl -sL -o "$archive_name" "$URL"; then
        echo "Failed to download $URL"
        exit 1
    fi

    if curl -sL -o checksums.txt "$CHECKSUM_URL"; then
        echo "Verifying checksum..."
        if ! grep "$archive_name" checksums.txt | sha256sum --check --status; then
            echo "❌ Checksum verification failed!"
            rm -f "$archive_name" checksums.txt
            exit 1
        fi
        rm -f checksums.txt
        echo "✅ Checksum verified"
    else
        echo "⚠️  Checksum file not found, skipping verification"
    fi

    local binary_name="$BINARY_NAME"
    if [ "$OS" = "windows" ]; then
        binary_name="$BINARY_NAME.exe"
        unzip -o "$archive_name" "$binary_name"
    else
        tar -xzf "$archive_name" "$binary_name"
    fi
    rm -f "$archive_name"

    chmod +x "$binary_name"

    if [ -w "$INSTALL_DIR" ]; then
        echo "Installing to $INSTALL_DIR..."
        mv "$binary_name" "$INSTALL_DIR/$BINARY_NAME"
        echo "✅ $BINARY_NAME installed successfully!"
        echo "Run 'llm-box' to get started."
    else
        echo "🔒 Need sudo to install to $INSTALL_DIR"
        echo "Please run: sudo mv $binary_name $INSTALL_DIR/$BINARY_NAME"
    fi
}

main() {
    echo "🚀 Installing $BINARY_NAME..."
    
    detect_platform
    echo "Detected platform: $OS/$ARCH"
    
    download_binary
}

main "$@"
