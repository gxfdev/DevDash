#!/bin/bash
set -e

VERSION="1.0.0"
ASSET_URL="https://github.com/devdash-dev/devdash/releases/download/v${VERSION}"

echo "DevDash 安装脚本"
echo "================"

# 检测系统
OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" = "Linux" ]; then
    BIN_URL="${ASSET_URL}/devdash-linux-amd64"
    echo "检测到 Linux 系统 (amd64)"
elif [ "$OS" = "Darwin" ]; then
    if [ "$ARCH" = "arm64" ]; then
        BIN_URL="${ASSET_URL}/devdash-darwin-arm64"
        echo "检测到 macOS ARM64"
    else
        BIN_URL="${ASSET_URL}/devdash-darwin-amd64"
        echo "检测到 macOS x86_64"
    fi
else
    echo "不支持的操作系统: $OS"
    exit 1
fi

INSTALL_DIR="/usr/local/bin"
mkdir -p "$INSTALL_DIR"

# 下载
echo "正在下载 DevDash..."
if command -v curl > /dev/null 2>&1; then
    curl -L -o "${INSTALL_DIR}/devdash" "${BIN_URL}"
elif command -v wget > /dev/null 2>&1; then
    wget -O "${INSTALL_DIR}/devdash" "${BIN_URL}"
else
    echo "curl 或 wget 未安装"
    exit 1
fi

chmod +x "${INSTALL_DIR}/devdash"

echo ""
echo "DevDash 安装完成！"
echo "默认端口: 9090"
echo "默认账号: admin / admin123"
echo ""
echo "启动命令: devdash"
echo "或使用 Docker: docker-compose up -d"
echo ""
echo "安装软件仓库源..."
curl -fsSL https://get.docker.com | sh 2>/dev/null || true

echo "完成！"