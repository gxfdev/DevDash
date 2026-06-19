#!/usr/bin/env bash
# 本地构建并推送镜像到 GitHub Container Registry (GHCR)
# 用法:
#   ./docker/build-push-ghcr.sh                  # 推送 latest + 当前版本
#   ./docker/build-push-ghcr.sh v1.5.0           # 推送指定版本
#   ./docker/build-push-ghcr.sh v1.5.0 --no-push # 仅构建不推送

set -euo pipefail

# ===== 配置 =====
REGISTRY="ghcr.io"
# 从 git remote 获取 owner/repo，或手动设置
OWNER_REPO="${GHCR_REPO:-$(git remote get-url origin 2>/dev/null | sed -n 's#.*github.com[:/]\(.*\)\.git#\1#p' || echo 'gxfdev/DevDash')}"
IMAGE_NAME="${REGISTRY}/${OWNER_REPO}"
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo 'dev')}"
PUSH="${2:---push}"

echo "========================================="
echo " Registry:  ${REGISTRY}"
echo " Image:     ${IMAGE_NAME}"
echo " Version:   ${VERSION}"
echo " Push:      ${PUSH}"
echo "========================================="

# ===== 1. 登录 GHCR =====
if [ "${PUSH}" != "--no-push" ]; then
    if [ -z "${GHCR_TOKEN:-}" ] && [ -z "${GITHUB_TOKEN:-}" ]; then
        echo "未设置 GHCR_TOKEN 或 GITHUB_TOKEN 环境变量"
        echo "请在 https://github.com/settings/tokens 创建 Personal Access Token (read:packages, write:packages)"
        echo "然后运行: export GHCR_TOKEN=ghp_xxxxx"
        exit 1
    fi
    echo "${GHCR_TOKEN:-${GITHUB_TOKEN}}" | docker login "${REGISTRY}" -u "${OWNER_REPO%%/*}" --password-stdin
fi

# ===== 2. 构建参数 =====
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
VCS_REF="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# ===== 3. 构建镜像 =====
echo ""
echo "[1/3] 构建镜像..."
docker build \
    --build-arg BUILD_DATE="${BUILD_DATE}" \
    --build-arg VCS_REF="${VCS_REF}" \
    --build-arg VERSION="${VERSION}" \
    -f docker/Dockerfile.server \
    -t "${IMAGE_NAME}:latest" \
    -t "${IMAGE_NAME}:${VERSION}" \
    .

# ===== 4. 推送镜像 =====
if [ "${PUSH}" != "--no-push" ]; then
    echo ""
    echo "[2/3] 推送镜像..."
    docker push "${IMAGE_NAME}:latest"
    docker push "${IMAGE_NAME}:${VERSION}"
    echo ""
    echo "[3/3] 推送完成！"
    echo ""
    echo "拉取命令:"
    echo "  docker pull ${IMAGE_NAME}:${VERSION}"
    echo "  docker pull ${IMAGE_NAME}:latest"
else
    echo ""
    echo "[2/3] 跳过推送 (--no-push)"
    echo "[3/3] 构建完成！"
fi

echo ""
echo "镜像信息:"
docker images "${IMAGE_NAME}" --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
