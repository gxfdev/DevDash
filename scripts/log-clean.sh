#!/usr/bin/env bash
# 日志清理脚本
# 用法: ./scripts/log-clean.sh [保留天数]
set -euo pipefail

DAYS="${1:-30}"
LOG_DIR="${LOG_DIR:-/var/log/devdash}"
JOURNAL_SIZE="${JOURNAL_SIZE:-500M}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }

echo -e "${CYAN}=== DevDash 日志清理 ===${NC}"
echo "保留天数: $DAYS"
echo ""

# 1. 清理应用日志
if [ -d "$LOG_DIR" ]; then
    log_info "清理应用日志目录: $LOG_DIR"
    echo "  清理前:"
    du -sh "$LOG_DIR" 2>/dev/null || true

    find "$LOG_DIR" -name "*.log" -mtime +$DAYS -delete 2>/dev/null || true
    find "$LOG_DIR" -name "*.log.*" -mtime +$DAYS -delete 2>/dev/null || true
    find "$LOG_DIR" -name "*.gz" -mtime +$DAYS -delete 2>/dev/null || true

    # 压缩旧日志
    find "$LOG_DIR" -name "*.log" -mtime +7 -exec gzip {} \; 2>/dev/null || true

    echo "  清理后:"
    du -sh "$LOG_DIR" 2>/dev/null || true
    log_info "应用日志清理完成"
else
    log_warn "日志目录不存在: $LOG_DIR"
fi
echo ""

# 2. 清理 systemd journal
if command -v journalctl > /dev/null 2>&1; then
    log_info "清理 systemd journal（保留 $JOURNAL_SIZE）..."
    echo "  清理前:"
    journalctl --disk-usage 2>/dev/null || true

    journalctl --vacuum-size="$JOURNAL_SIZE" 2>/dev/null || true
    journalctl --vacuum-time="${DAYS}d" 2>/dev/null || true

    echo "  清理后:"
    journalctl --disk-usage 2>/dev/null || true
    log_info "journal 清理完成"
fi
echo ""

# 3. 清理 Docker 容器日志
if command -v docker > /dev/null 2>&1; then
    log_info "清理 Docker 容器日志..."
    CONTAINERS=$(docker ps -q 2>/dev/null || true)
    if [ -n "$CONTAINERS" ]; then
        for cid in $CONTAINERS; do
            LOG_PATH=$(docker inspect --format='{{.LogPath}}' "$cid" 2>/dev/null || true)
            if [ -n "$LOG_PATH" ] && [ -f "$LOG_PATH" ]; then
                SIZE=$(du -sh "$LOG_PATH" 2>/dev/null | cut -f1 || echo "?")
                echo "  容器 $(docker inspect --format='{{.Name}}' "$cid" 2>/dev/null): $SIZE"
            fi
        done
        log_info "如需清理Docker日志，请配置 /etc/docker/daemon.json 的 log-opts.max-size"
    fi
fi
echo ""

# 4. 清理临时文件
log_info "清理临时文件..."
TMP_DIRS=("/tmp/devdash_*" "/tmp/devdash_restore_*")
for pattern in "${TMP_DIRS[@]}"; do
    find /tmp -maxdepth 1 -name "$pattern" -mtime +1 -delete 2>/dev/null || true
done
log_info "临时文件清理完成"
echo ""

echo -e "${GREEN}=== 日志清理完成 ===${NC}"
