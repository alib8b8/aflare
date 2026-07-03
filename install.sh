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
    local filename="$BINARY_NAME-$OS-$ARCH"
    if [ "$OS" = "windows" ]; then
        filename="$filename.exe"
    fi

    echo "Downloading $filename..."
    
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    echo "Latest release: $LATEST_RELEASE"

    URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/$filename"
    CHECKSUM_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/SHA256SUMS"
    
    if ! curl -sL -o "$filename" "$URL"; then
        echo "Failed to download $URL"
        exit 1
    fi

    if curl -sL -o SHA256SUMS "$CHECKSUM_URL"; then
        echo "Verifying checksum..."
        if ! sha256sum --check --ignore-missing SHA256SUMS; then
            echo "❌ Checksum verification failed!"
            rm -f "$filename" SHA256SUMS
            exit 1
        fi
        rm -f SHA256SUMS
        echo "✅ Checksum verified"
    else
        echo "⚠️  Checksum file not found, skipping verification"
    fi

    chmod +x "$filename"
    
    if [ -w "$INSTALL_DIR" ]; then
        echo "Installing to $INSTALL_DIR..."
        mv "$filename" "$INSTALL_DIR/$BINARY_NAME"
        echo "✅ $BINARY_NAME installed successfully!"
        echo "Run 'llm-box' to get started."
    else
        echo "🔒 Need sudo to install to $INSTALL_DIR"
        echo "Please run: sudo mv $filename $INSTALL_DIR/$BINARY_NAME"
    fi
}

main() {
    echo "🚀 Installing $BINARY_NAME..."
    
    detect_platform
    echo "Detected platform: $OS/$ARCH"
    
    download_binary
}

main "$@"
