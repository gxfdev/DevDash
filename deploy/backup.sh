#!/bin/bash
set -euo pipefail

BACKUP_DIR="/opt/devdash/backups"
DATA_DIR="/opt/devdash/data"
ENV_FILE="/opt/devdash/.env"
MAX_DAILY=7
MAX_WEEKLY=4
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

create_backup() {
    local backup_type="${1:-daily}"
    local backup_name="backup_${backup_type}_${TIMESTAMP}"
    local backup_path="${BACKUP_DIR}/${backup_name}"

    mkdir -p "$backup_path"

    log_info "创建 ${backup_type} 备份: ${backup_name}"

    if [ -f "${DATA_DIR}/devdash.db" ]; then
        sqlite3 "${DATA_DIR}/devdash.db" ".backup '${backup_path}/devdash.db'" 2>/dev/null || \
            cp "${DATA_DIR}/devdash.db" "${backup_path}/devdash.db"
        log_info "数据库备份完成"
    fi

    if [ -f "$ENV_FILE" ]; then
        cp "$ENV_FILE" "${backup_path}/.env"
        log_info "环境配置备份完成"
    fi

    if [ -d "${DATA_DIR}" ]; then
        tar czf "${backup_path}/data.tar.gz" -C "$(dirname $DATA_DIR)" "$(basename $DATA_DIR)" 2>/dev/null || true
        log_info "数据目录备份完成"
    fi

    tar czf "${BACKUP_DIR}/${backup_name}.tar.gz" -C "$BACKUP_DIR" "$backup_name" 2>/dev/null || true
    rm -rf "$backup_path"

    local size=$(du -sh "${BACKUP_DIR}/${backup_name}.tar.gz" 2>/dev/null | cut -f1 || echo "unknown")
    log_info "备份完成: ${backup_name}.tar.gz ($size)"

    cleanup_old_backups "$backup_type"

    echo "[$(date -Iseconds)] ${backup_type} backup created: ${backup_name}.tar.gz ($size)" >> "${BACKUP_DIR}/backup.log"
}

restore_backup() {
    local backup_file="$1"

    if [ ! -f "$backup_file" ]; then
        log_error "备份文件不存在: $backup_file"
        exit 1
    fi

    log_info "停止服务..."
    systemctl stop devdash 2>/dev/null || true

    local restore_dir="${BACKUP_DIR}/restore_${TIMESTAMP}"
    mkdir -p "$restore_dir"

    log_info "解压备份..."
    tar xzf "$backup_file" -C "$restore_dir"

    local backup_subdir=$(ls "$restore_dir" | head -1)
    if [ -n "$backup_subdir" ] && [ -d "$restore_dir/$backup_subdir" ]; then
        restore_dir="$restore_dir/$backup_subdir"
    fi

    if [ -f "${restore_dir}/devdash.db" ]; then
        cp "${DATA_DIR}/devdash.db" "${DATA_DIR}/devdash.db.pre-restore" 2>/dev/null || true
        cp "${restore_dir}/devdash.db" "${DATA_DIR}/devdash.db"
        log_info "数据库已恢复"
    fi

    if [ -f "${restore_dir}/.env" ]; then
        cp "$ENV_FILE" "${ENV_FILE}.pre-restore" 2>/dev/null || true
        cp "${restore_dir}/.env" "$ENV_FILE"
        log_info "环境配置已恢复"
    fi

    log_info "启动服务..."
    systemctl start devdash 2>/dev/null || true

    log_info "恢复完成"
}

cleanup_old_backups() {
    local backup_type="$1"

    case "$backup_type" in
        daily)
            find "$BACKUP_DIR" -name "backup_daily_*.tar.gz" -mtime +${MAX_DAILY} -delete 2>/dev/null || true
            ;;
        weekly)
            find "$BACKUP_DIR" -name "backup_weekly_*.tar.gz" -mtime +$((MAX_WEEKLY * 7)) -delete 2>/dev/null || true
            ;;
    esac
}

case "${1:-create}" in
    create)
        create_backup "${2:-daily}"
        ;;
    restore)
        restore_backup "$2"
        ;;
    weekly)
        create_backup "weekly"
        ;;
    *)
        echo "Usage: $0 {create|restore|weekly} [daily|weekly|backup_file]"
        exit 1
        ;;
esac
