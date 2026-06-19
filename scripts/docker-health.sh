#!/usr/bin/env bash
# Docker 容器健康检查脚本
# 用法: ./scripts/docker-health.sh [容器名]
set -euo pipefail

CONTAINER="${1:-}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# 检查容器健康状态
check_container() {
    local name="$1"
    local status
    status=$(docker inspect --format='{{.State.Status}}' "$name" 2>/dev/null || echo "not_found")

    if [ "$status" = "not_found" ]; then
        echo -e "  ${RED}$name: 不存在${NC}"
        return 1
    fi

    local health
    health=$(docker inspect --format='{{.State.Health.Status}}' "$name" 2>/dev/null || echo "no_healthcheck")

    local restarts
    restarts=$(docker inspect --format='{{.RestartCount}}' "$name" 2>/dev/null || echo "0")

    local started
    started=$(docker inspect --format='{{.State.StartedAt}}' "$name" 2>/dev/null || echo "unknown")

    case "$status" in
        running)
            if [ "$health" = "healthy" ] || [ "$health" = "no_healthcheck" ]; then
                echo -e "  ${GREEN}$name: 运行中${NC} (健康: $health, 重启: $restarts, 启动: $started)"
                return 0
            elif [ "$health" = "unhealthy" ]; then
                echo -e "  ${RED}$name: 运行中但不健康${NC} (重启: $restarts)"
                return 1
            else
                echo -e "  ${YELLOW}$name: 运行中${NC} (健康检查中: $health)"
                return 0
            fi
            ;;
        *)
            echo -e "  ${RED}$name: $status${NC}"
            return 1
            ;;
    esac
}

# 检查端口连通性
check_port() {
    local host="$1"
    local port="$2"
    local name="$3"
    if command -v nc > /dev/null 2>&1; then
        if nc -z -w3 "$host" "$port" 2>/dev/null; then
            echo -e "  ${GREEN}$name ($host:$port): 可达${NC}"
        else
            echo -e "  ${RED}$name ($host:$port): 不可达${NC}"
        fi
    elif command -v curl > /dev/null 2>&1; then
        if curl -sf --connect-timeout 3 "http://$host:$port" > /dev/null 2>&1; then
            echo -e "  ${GREEN}$name ($host:$port): 可达${NC}"
        else
            echo -e "  ${RED}$name ($host:$port): 不可达${NC}"
        fi
    fi
}

# 检查HTTP端点
check_http() {
    local url="$1"
    local name="$2"
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 "$url" 2>/dev/null || echo "000")
    if [ "$code" = "200" ] || [ "$code" = "401" ]; then
        echo -e "  ${GREEN}$name ($url): $code${NC}"
    else
        echo -e "  ${RED}$name ($url): $code${NC}"
    fi
}

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}     DevDash 容器健康检查${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# 容器状态检查
echo -e "${CYAN}[1/4] 容器状态${NC}"
ALL_HEALTHY=true
if [ -n "$CONTAINER" ]; then
    check_container "$CONTAINER" || ALL_HEALTHY=false
else
    # 从docker-compose获取所有容器
    if [ -f "$COMPOSE_FILE" ]; then
        CONTAINERS=$(docker compose -f "$COMPOSE_FILE" ps -q 2>/dev/null || true)
        if [ -n "$CONTAINERS" ]; then
            for cid in $CONTAINERS; do
                name=$(docker inspect --format='{{.Name}}' "$cid" 2>/dev/null | sed 's|^/||')
                check_container "$name" || ALL_HEALTHY=false
            done
        else
            echo -e "  ${YELLOW}未找到运行中的容器${NC}"
        fi
    fi
fi
echo ""

# 端口连通性检查
echo -e "${CYAN}[2/4] 端口连通性${NC}"
check_port "localhost" 9090 "DevDash API"
check_port "localhost" 9091 "Prometheus"
check_port "localhost" 3001 "Grafana"
check_port "localhost" 9100 "Node Exporter"
check_port "localhost" 8080 "cAdvisor"
echo ""

# HTTP端点检查
echo -e "${CYAN}[3/4] HTTP 端点${NC}"
check_http "http://localhost:9090/api/v1/health" "DevDash Health"
check_http "http://localhost:9090/api/metrics" "DevDash Metrics"
check_http "http://localhost:9091/-/healthy" "Prometheus Health"
check_http "http://localhost:3001/api/health" "Grafana Health"
echo ""

# 资源使用检查
echo -e "${CYAN}[4/4] 资源使用${NC}"
if [ -n "$CONTAINERS" ]; then
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}" $CONTAINERS 2>/dev/null || true
fi
echo ""

# 总结
if [ "$ALL_HEALTHY" = "true" ]; then
    echo -e "${GREEN}✓ 所有容器健康${NC}"
    exit 0
else
    echo -e "${RED}✗ 部分容器异常，请检查${NC}"
    exit 1
fi
