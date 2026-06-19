#!/usr/bin/env bash
# DevDash 数据恢复脚本
# 用法: ./scripts/restore.sh <备份文件路径> [--dry-run]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
DATA_DIR="${DEVDASH_DATA_DIR:-/opt/devdash/data}"
ENV_FILE="${DEVDASH_ENV_FILE:-/opt/devdash/.env}"
BACKUP_FILE="${1:-}"
DRY_RUN="${2:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

TIMESTAMP=$(date +%Y%m%d_%H%M%S)

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

if [ -z "$BACKUP_FILE" ]; then
    echo "用法: $0 <备份文件路径> [--dry-run]"
    echo ""
    echo "示例:"
    echo "  $0 /opt/devdash/backups/backup_daily_20260619_120000.tar.gz"
    echo "  $0 /opt/devdash/backups/backup_daily_20260619_120000.tar.gz --dry-run"
    echo ""
    echo "可用备份文件:"
    ls -lh /opt/devdash/backups/*.tar.gz 2>/dev/null || echo "  (无备份文件)"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    log_error "备份文件不存在: $BACKUP_FILE"
    exit 1
fi

log_info "备份文件: $BACKUP_FILE"
log_info "数据目录: $DATA_DIR"
log_info "环境文件: $ENV_FILE"

if [ "$DRY_RUN" = "--dry-run" ]; then
    log_warn "DRY RUN 模式 - 仅模拟，不实际恢复"
fi

# 1. 创建临时恢复目录
RESTORE_DIR="/tmp/devdash_restore_${TIMESTAMP}"
mkdir -p "$RESTORE_DIR"

cleanup() {
    rm -rf "$RESTORE_DIR"
}
trap cleanup EXIT

# 2. 解压备份
log_info "解压备份文件..."
tar xzf "$BACKUP_FILE" -C "$RESTORE_DIR"

# 查找实际数据目录（备份可能是嵌套的）
RESTORE_SRC="$RESTORE_DIR"
BACKUP_SUBDIR=$(ls "$RESTORE_DIR" | head -1)
if [ -n "$BACKUP_SUBDIR" ] && [ -d "$RESTORE_DIR/$BACKUP_SUBDIR" ]; then
    RESTORE_SRC="$RESTORE_DIR/$BACKUP_SUBDIR"
fi

echo ""
echo -e "${CYAN}=== 备份内容 ===${NC}"
ls -lh "$RESTORE_SRC/"

# 3. DRY RUN 检查
if [ "$DRY_RUN" = "--dry-run" ]; then
    echo ""
    log_warn "DRY RUN - 以下是将要执行的操作:"
    echo "  1. 停止 devdash 服务"
    echo "  2. 备份当前数据到 ${DATA_DIR}.pre-restore-${TIMESTAMP}"
    [ -f "$RESTORE_SRC/devdash.db" ] && echo "  3. 恢复数据库: ${RESTORE_SRC}/devdash.db → ${DATA_DIR}/devdash.db"
    [ -f "$RESTORE_SRC/.env" ] && echo "  4. 恢复环境配置: ${RESTORE_SRC}/.env → ${ENV_FILE}"
    echo "  5. 启动 devdash 服务"
    echo ""
    log_info "DRY RUN 完成，未做任何更改"
    exit 0
fi

# 4. 停止服务
log_info "停止 devdash 服务..."
if systemctl is-active --quiet devdash 2>/dev/null; then
    systemctl stop devdash
    SERVICE_MODE="systemd"
elif docker compose -f "$PROJECT_DIR/docker-compose.yml" ps -q 2>/dev/null | grep -q .; then
    docker compose -f "$PROJECT_DIR/docker-compose.yml" stop
    SERVICE_MODE="docker"
else
    log_warn "未检测到运行中的服务"
    SERVICE_MODE="none"
fi

# 5. 备份当前数据
if [ -d "$DATA_DIR" ]; then
    BACKUP_CURRENT="${DATA_DIR}.pre-restore-${TIMESTAMP}"
    log_info "备份当前数据到: $BACKUP_CURRENT"
    cp -r "$DATA_DIR" "$BACKUP_CURRENT"
fi

# 6. 恢复数据库
if [ -f "$RESTORE_SRC/devdash.db" ]; then
    mkdir -p "$DATA_DIR"
    log_info "恢复数据库..."
    cp "$RESTORE_SRC/devdash.db" "${DATA_DIR}/devdash.db"
    chmod 644 "${DATA_DIR}/devdash.db"
    log_info "数据库恢复完成"
else
    log_warn "备份中未找到数据库文件"
fi

# 7. 恢复环境配置
if [ -f "$RESTORE_SRC/.env" ]; then
    ENV_DIR=$(dirname "$ENV_FILE")
    mkdir -p "$ENV_DIR"
    log_info "恢复环境配置..."
    cp "$RESTORE_SRC/.env" "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    log_info "环境配置恢复完成"
fi

# 8. 启动服务
if [ "$SERVICE_MODE" = "systemd" ]; then
    log_info "启动 devdash 服务..."
    systemctl start devdash
elif [ "$SERVICE_MODE" = "docker" ]; then
    log_info "启动 docker 容器..."
    docker compose -f "$PROJECT_DIR/docker-compose.yml" start
fi

# 9. 健康检查
log_info "等待服务启动..."
sleep 3
for i in $(seq 1 15); do
    if curl -sf http://localhost:9090/api/v1/health > /dev/null 2>&1; then
        log_info "服务恢复成功！"
        echo ""
        echo -e "${GREEN}恢复完成！${NC}"
        echo "  备份文件: $BACKUP_FILE"
        echo "  恢复时间: $(date)"
        [ -n "$BACKUP_CURRENT" ] && echo "  原数据备份: $BACKUP_CURRENT"
        exit 0
    fi
    sleep 2
done

log_error "服务启动超时，请检查日志"
[ "$SERVICE_MODE" = "systemd" ] && journalctl -u devdash -n 20 --no-pager
[ "$SERVICE_MODE" = "docker" ] && docker compose -f "$PROJECT_DIR/docker-compose.yml" logs -n 20
exit 1
