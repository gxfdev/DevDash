#!/usr/bin/env bash
# 系统性能监控脚本
# 用法: ./scripts/monitor.sh [interval]
set -euo pipefail

INTERVAL="${1:-5}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 获取CPU使用率
get_cpu() {
    if [ -f /proc/stat ]; then
        local idle
        idle=$(top -bn1 | grep "Cpu(s)" | sed 's/.*, *\([0-9.]*\)%* id.*/\1/' 2>/dev/null || echo "0")
        echo "scale=1; 100 - $idle" | bc 2>/dev/null || echo "0"
    else
        echo "0"
    fi
}

# 获取内存使用
get_memory() {
    if [ -f /proc/meminfo ]; then
        local total used
        total=$(grep MemTotal /proc/meminfo | awk '{print $2}')
        local avail
        avail=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
        used=$((total - avail))
        local pct=$((used * 100 / total))
        local used_gb
        used_gb=$(echo "scale=1; $used / 1024 / 1024" | bc 2>/dev/null || echo "?")
        local total_gb
        total_gb=$(echo "scale=1; $total / 1024 / 1024" | bc 2>/dev/null || echo "?")
        echo "${pct}% (${used_gb}G / ${total_gb}G)"
    else
        echo "N/A"
    fi
}

# 获取磁盘使用
get_disk() {
    local path="${1:-/}"
    df -h "$path" 2>/dev/null | awk 'NR==2{print $5 " (" $3 "/" $2 ")"}' || echo "N/A"
}

# 获取网络流量
get_network() {
    if [ -d /proc/net ]; then
        local rx tx
        rx=$(cat /proc/net/dev | grep -E "eth0|ens|enp" | head -1 | awk '{print $2}' 2>/dev/null || echo "0")
        tx=$(cat /proc/net/dev | grep -E "eth0|ens|enp" | head -1 | awk '{print $10}' 2>/dev/null || echo "0")
        local rx_mb tx_mb
        rx_mb=$(echo "scale=1; $rx / 1024 / 1024" | bc 2>/dev/null || echo "?")
        tx_mb=$(echo "scale=1; $tx / 1024 / 1024" | bc 2>/dev/null || echo "?")
        echo "RX: ${rx_mb}MB TX: ${tx_mb}MB"
    else
        echo "N/A"
    fi
}

# 获取负载
get_load() {
    if [ -f /proc/loadavg ]; then
        cat /proc/loadavg | awk '{print $1 " " $2 " " $3}'
    else
        uptime | awk -F'load average:' '{print $2}'
    fi
}

# 获取进程数
get_procs() {
    ls /proc | grep -E '^[0-9]+$' | wc -l 2>/dev/null || echo "?"
}

# 获取Docker容器数
get_containers() {
    if command -v docker > /dev/null 2>&1; then
        local running total
        running=$(docker ps -q 2>/dev/null | wc -l)
        total=$(docker ps -aq 2>/dev/null | wc -l)
        echo "${running}运行 / ${total}总计"
    else
        echo "Docker未安装"
    fi
}

# 检查DevDash服务
check_devdash() {
    if curl -sf http://localhost:9090/api/v1/health > /dev/null 2>&1; then
        echo -e "${GREEN}运行中${NC}"
    else
        echo -e "${RED}不可达${NC}"
    fi
}

# 颜色阈值
color_value() {
    local value="$1"
    local warn="$2"
    local crit="$3"
    local num=$(echo "$value" | grep -oE '[0-9]+(\.[0-9]+)?' | head -1)
    if [ -n "$num" ]; then
        if (( $(echo "$num >= $crit" | bc -l 2>/dev/null || echo 0) )); then
            echo -e "${RED}$value${NC}"
        elif (( $(echo "$num >= $warn" | bc -l 2>/dev/null || echo 0) )); then
            echo -e "${YELLOW}$value${NC}"
        else
            echo -e "${GREEN}$value${NC}"
        fi
    else
        echo "$value"
    fi
}

# 单次快照
snapshot() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              DevDash 系统监控 - $(date '+%H:%M:%S')              ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════╣${NC}"

    local cpu=$(get_cpu)
    local mem=$(get_memory)
    local disk_root=$(get_disk "/")
    local disk_data=$(get_disk "/opt" 2>/dev/null || echo "N/A")
    local net=$(get_network)
    local load=$(get_load)
    local procs=$(get_procs)
    local containers=$(get_containers)
    local devdash=$(check_devdash)

    echo -e "${CYAN}║${NC}  CPU使用率:  $(color_value "${cpu}%" 70 90)                          ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  内存使用:    $(color_value "$mem" 70 85)                          ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  磁盘(/):     $(color_value "$disk_root" 70 90)                       ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  磁盘(/opt):  $(color_value "$disk_data" 70 90)                       ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  网络流量:    $net                          ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  系统负载:    $(color_value "$load" 4 8)                          ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  进程数:      $procs                           ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  Docker容器: $containers                        ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}  DevDash:     $devdash                          ${CYAN}║${NC}"

    echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
}

# 持续监控
if [ "$INTERVAL" = "once" ]; then
    snapshot
else
    while true; do
        clear
        snapshot
        echo ""
        echo "刷新间隔: ${INTERVAL}s | Ctrl+C 退出"
        sleep "$INTERVAL"
    done
fi
