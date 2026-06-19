#!/usr/bin/env bash
# DevDash 数据库迁移脚本
# 用法: ./scripts/migrate.sh [up|down|status|create|reset]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="$PROJECT_DIR/server/migrations"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 数据库路径
DB_PATH="${DB_PATH:-$PROJECT_DIR/server/devdash.db}"

# 迁移记录表
MIGRATION_TABLE="_migrations"

CMD="${1:-status}"

# 确保sqlite3可用
check_sqlite() {
    if ! command -v sqlite3 > /dev/null 2>&1; then
        log_error "sqlite3 未安装"
        echo "安装: apt install sqlite3 或 yum install sqlite"
        exit 1
    fi
}

# 初始化迁移表
init_migrations() {
    sqlite3 "$DB_PATH" "
        CREATE TABLE IF NOT EXISTS ${MIGRATION_TABLE} (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );
    " 2>/dev/null
}

# 获取已应用的迁移
get_applied() {
    sqlite3 "$DB_PATH" "SELECT name FROM ${MIGRATION_TABLE} ORDER BY name;" 2>/dev/null
}

# 获取所有迁移文件
get_pending() {
    local applied="$1"
    ls "$MIGRATIONS_DIR"/*.sql 2>/dev/null | sort | while read -r file; do
        local name=$(basename "$file")
        if ! echo "$applied" | grep -q "$name"; then
            echo "$file"
        fi
    done
}

migrate_up() {
    init_migrations
    local applied
    applied=$(get_applied)
    local pending
    pending=$(get_pending "$applied")

    if [ -z "$pending" ]; then
        log_info "没有待执行的迁移"
        return 0
    fi

    echo "$pending" | while read -r file; do
        local name=$(basename "$file")
        log_info "应用迁移: $name"
        sqlite3 "$DB_PATH" < "$file"
        sqlite3 "$DB_PATH" "INSERT INTO ${MIGRATION_TABLE} (name) VALUES ('$name');"
        log_info "完成: $name"
    done

    log_info "所有迁移已应用"
}

migrate_status() {
    init_migrations
    local applied
    applied=$(get_applied)

    echo -e "${CYAN}=== 数据库迁移状态 ===${NC}"
    echo "数据库: $DB_PATH"
    echo ""

    echo -e "${CYAN}已应用的迁移:${NC}"
    if [ -z "$applied" ]; then
        echo "  (无)"
    else
        echo "$applied" | while read -r name; do
            echo -e "  ${GREEN}✓ $name${NC}"
        done
    fi

    echo ""
    echo -e "${CYAN}待执行的迁移:${NC}"
    local pending
    pending=$(get_pending "$applied")
    if [ -z "$pending" ]; then
        echo -e "  ${GREEN}(无)${NC}"
    else
        echo "$pending" | while read -r file; do
            local name=$(basename "$file")
            echo -e "  ${YELLOW}⏳ $name${NC}"
        done
    fi
}

create_migration() {
    local name="${2:-unnamed}"
    local timestamp=$(date +%Y%m%d%H%M%S)
    local filename="${timestamp}_${name}.sql"
    local filepath="$MIGRATIONS_DIR/$filename"

    mkdir -p "$MIGRATIONS_DIR"
    cat > "$filepath" << EOF
-- Migration: $name
-- Created: $(date -Iseconds)
-- Description: TODO

-- Up migration
BEGIN;

-- TODO: 在此添加迁移SQL

COMMIT;
EOF
    log_info "创建迁移文件: $filepath"
}

migrate_reset() {
    log_warn "重置数据库将删除所有数据！"
    read -p "确认重置？输入 'yes' 继续: " -r
    if [ "$REPLY" != "yes" ]; then
        log_warn "已取消"
        exit 0
    fi

    log_info "删除数据库..."
    rm -f "$DB_PATH"
    log_info "重新执行所有迁移..."
    migrate_up
    log_info "数据库已重置"
}

case "$CMD" in
    up)
        check_sqlite
        migrate_up
        ;;
    status)
        check_sqlite
        migrate_status
        ;;
    create)
        create_migration "$@"
        ;;
    reset)
        check_sqlite
        migrate_reset
        ;;
    *)
        echo "用法: $0 {up|status|create|reset}"
        echo ""
        echo "  up            执行所有待执行的迁移"
        echo "  status        查看迁移状态"
        echo "  create <name> 创建新迁移文件"
        echo "  reset         重置数据库（危险操作）"
        echo ""
        echo "环境变量:"
        echo "  DB_PATH       数据库路径（默认: server/devdash.db）"
        exit 1
        ;;
esac
