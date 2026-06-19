#!/usr/bin/env bash
# Docker 资源清理脚本
# 用法: ./scripts/docker-clean.sh [--all] [--images] [--volumes]
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

MODE="${1:---safe}"

show_usage() {
    echo ""
    echo -e "${CYAN}清理前磁盘使用:${NC}"
    docker system df
    echo ""
}

show_usage

case "$MODE" in
    --safe)
        log_info "安全清理模式（不删除运行中容器的资源）..."
        # 清理已停止的容器
        STOPPED=$(docker ps -a -f status=exited -f status=dead -q)
        if [ -n "$STOPPED" ]; then
            log_info "删除已停止的容器..."
            docker rm $STOPPED 2>/dev/null || true
        fi
        # 清理悬空镜像
        log_info "删除悬空镜像（<none>）..."
        docker image prune -f
        # 清理悬空卷
        log_info "删除悬空卷..."
        docker volume prune -f
        # 清理构建缓存
        log_info "清理构建缓存..."
        docker builder prune -f
        ;;
    --all)
        log_warn "深度清理模式（删除所有未使用资源）..."
        read -p "确认清理所有未使用的镜像、容器、网络？(y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            docker system prune -a -f --volumes
            log_info "深度清理完成"
        else
            log_warn "已取消"
            exit 0
        fi
        ;;
    --images)
        log_info "清理未使用的镜像..."
        docker image prune -a -f
        ;;
    --volumes)
        log_warn "清理未使用的卷（注意：可能删除数据）..."
        read -p "确认删除所有未使用的卷？(y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            docker volume prune -a -f
        else
            log_warn "已取消"
        fi
        ;;
    *)
        echo "用法: $0 [--safe|--all|--images|--volumes]"
        echo ""
        echo "  --safe      安全清理（默认）：停止的容器+悬空镜像+悬空卷+构建缓存"
        echo "  --all       深度清理：所有未使用的镜像+容器+网络+卷"
        echo "  --images    仅清理未使用的镜像"
        echo "  --volumes   仅清理未使用的卷"
        exit 1
        ;;
esac

echo ""
echo -e "${CYAN}清理后磁盘使用:${NC}"
docker system df
echo ""

# 显示DevDash相关资源
echo -e "${CYAN}DevDash 相关资源:${NC}"
echo "  镜像:"
docker images --filter "reference=*devdash*" --format "    {{.Repository}}:{{.Tag}} ({{.Size}})" 2>/dev/null || true
echo "  容器:"
docker ps -a --filter "name=devdash" --format "    {{.Names}}: {{.Status}}" 2>/dev/null || true
echo "  卷:"
docker volume ls --filter "name=devdash" --format "    {{.Name}}" 2>/dev/null || true
