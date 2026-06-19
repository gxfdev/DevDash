#!/usr/bin/env bash
# DevDash 服务管理脚本
# 支持 systemd 和 docker compose 两种部署方式
# 用法: ./scripts/service.sh {start|stop|restart|status|enable|disable|logs}
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

SERVICE_NAME="devdash"
COMPOSE_FILE="${PROJECT_DIR}/docker-compose.yml"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检测部署方式
detect_mode() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "systemd"
    elif [ -f "$COMPOSE_FILE" ] && docker compose ps -q 2>/dev/null | grep -q .; then
        echo "docker"
    elif [ -f "$COMPOSE_FILE" ]; then
        echo "docker"
    else
        echo "systemd"
    fi
}

MODE=$(detect_mode)

start_service() {
    log_info "启动 DevDash (模式: $MODE)..."
    case "$MODE" in
        systemd)
            systemctl start "$SERVICE_NAME"
            log_info "服务已启动"
            systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null | head -10
            ;;
        docker)
            docker compose -f "$COMPOSE_FILE" up -d
            log_info "容器已启动"
            docker compose -f "$COMPOSE_FILE" ps
            ;;
    esac
}

stop_service() {
    log_info "停止 DevDash (模式: $MODE)..."
    case "$MODE" in
        systemd)
            systemctl stop "$SERVICE_NAME"
            log_info "服务已停止"
            ;;
        docker)
            docker compose -f "$COMPOSE_FILE" down
            log_info "容器已停止"
            ;;
    esac
}

restart_service() {
    log_info "重启 DevDash (模式: $MODE)..."
    case "$MODE" in
        systemd)
            systemctl restart "$SERVICE_NAME"
            log_info "服务已重启"
            systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null | head -10
            ;;
        docker)
            docker compose -f "$COMPOSE_FILE" restart
            log_info "容器已重启"
            docker compose -f "$COMPOSE_FILE" ps
            ;;
    esac
}

status_service() {
    echo -e "\n${CYAN}=== DevDash 服务状态 ===${NC}"
    case "$MODE" in
        systemd)
            systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null || log_warn "服务未安装"
            ;;
        docker)
            docker compose -f "$COMPOSE_FILE" ps
            echo ""
            echo -e "${CYAN}=== 资源使用 ===${NC}"
            docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" \
                $(docker compose -f "$COMPOSE_FILE" ps -q 2>/dev/null) 2>/dev/null || true
            ;;
    esac

    # 健康检查
    echo ""
    echo -e "${CYAN}=== 健康检查 ===${NC}"
    if curl -sf http://localhost:9090/api/v1/health > /dev/null 2>&1; then
        echo -e "  API: ${GREEN}健康${NC}"
    else
        echo -e "  API: ${RED}不可达${NC}"
    fi
    echo ""
}

enable_service() {
    case "$MODE" in
        systemd)
            systemctl enable "$SERVICE_NAME"
            log_info "已设置开机自启"
            ;;
        docker)
            log_warn "Docker模式请使用 restart: unless-stopped 实现自启"
            ;;
    esac
}

disable_service() {
    case "$MODE" in
        systemd)
            systemctl disable "$SERVICE_NAME"
            log_info "已取消开机自启"
            ;;
        docker)
            log_warn "Docker模式请在compose文件中修改 restart 策略"
            ;;
    esac
}

show_logs() {
    local lines="${1:-50}"
    case "$MODE" in
        systemd)
            journalctl -u "$SERVICE_NAME" -n "$lines" --no-pager -f
            ;;
        docker)
            docker compose -f "$COMPOSE_FILE" logs -n "$lines" -f
            ;;
    esac
}

usage() {
    echo -e "${CYAN}DevDash 服务管理脚本${NC}"
    echo ""
    echo "用法: $0 <命令> [参数]"
    echo ""
    echo "命令:"
    echo "  start       启动服务"
    echo "  stop        停止服务"
    echo "  restart     重启服务"
    echo "  status      查看状态"
    echo "  enable      设置开机自启"
    echo "  disable     取消开机自启"
    echo "  logs [N]    查看最近N行日志（默认50，实时跟踪）"
    echo ""
    echo "示例:"
    echo "  $0 start          # 启动服务"
    echo "  $0 status         # 查看状态"
    echo "  $0 logs 100        # 查看最近100行日志"
    echo ""
}

case "${1:-}" in
    start)   start_service ;;
    stop)    stop_service ;;
    restart) restart_service ;;
    status)  status_service ;;
    enable)  enable_service ;;
    disable) disable_service ;;
    logs)    show_logs "${2:-50}" ;;
    *)       usage; exit 1 ;;
esac
