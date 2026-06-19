#!/usr/bin/env bash
# Docker 日志查看脚本
# 用法: ./scripts/docker-logs.sh [服务名] [行数]
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
SERVICE="${1:-}"
LINES="${2:-50}"
FOLLOW="${3:-}"

CYAN='\033[0;36m'
NC='\033[0m'

usage() {
    echo -e "${CYAN}DevDash Docker 日志查看${NC}"
    echo ""
    echo "用法: $0 [服务名] [行数] [follow]"
    echo ""
    echo "参数:"
    echo "  服务名  指定服务（devdash/prometheus/grafana/node-exporter/cadvisor/all）"
    echo "  行数    显示最近N行（默认50）"
    echo "  follow  f=实时跟踪"
    echo ""
    echo "示例:"
    echo "  $0                    # 查看所有服务最近50行"
    echo "  $0 devdash            # 查看devdash服务日志"
    echo "  $0 devdash 100        # 查看devdash最近100行"
    echo "  $0 devdash 100 f     # 实时跟踪devdash日志"
    echo "  $0 all 200            # 查看所有服务最近200行"
}

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "未找到 $COMPOSE_FILE"
    exit 1
fi

if [ -z "$SERVICE" ] || [ "$SERVICE" = "all" ]; then
    echo -e "${CYAN}=== 所有服务日志 ===${NC}"
    if [ "$FOLLOW" = "f" ]; then
        docker compose -f "$COMPOSE_FILE" logs -f
    else
        docker compose -f "$COMPOSE_FILE" logs -n "$LINES"
    fi
else
    echo -e "${CYAN}=== $SERVICE 日志（最近 $LINES 行）===${NC}"
    if [ "$FOLLOW" = "f" ]; then
        docker compose -f "$COMPOSE_FILE" logs -f "$SERVICE"
    else
        docker compose -f "$COMPOSE_FILE" logs -n "$LINES" "$SERVICE"
    fi
fi
