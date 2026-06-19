#!/usr/bin/env bash
# 防火墙配置脚本
# 用法: ./scripts/firewall.sh [setup|status|allow|block|reset]
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# DevDash 需要的端口
# 9090  - DevDash API
# 9091  - Prometheus
# 3001  - Grafana
# 9100  - Node Exporter
# 8080  - cAdvisor
# 80/443 - HTTP/HTTPS

DEVDASH_PORTS=(9090 9091 3001 9100 8080)
WEB_PORTS=(80 443)
SSH_PORT=22

# 检测防火墙类型
detect_firewall() {
    if command -v ufw > /dev/null 2>&1; then
        echo "ufw"
    elif command -v firewall-cmd > /dev/null 2>&1; then
        echo "firewalld"
    elif command -v iptables > /dev/null 2>&1; then
        echo "iptables"
    else
        echo "none"
    fi
}

FW=$(detect_firewall)

setup_ufw() {
    log_info "配置 UFW 防火墙..."

    # 默认策略
    ufw default deny incoming
    ufw default allow outgoing

    # SSH
    ufw allow ${SSH_PORT}/tcp comment 'SSH'

    # Web
    for port in "${WEB_PORTS[@]}"; do
        ufw allow ${port}/tcp comment "Web ${port}"
    done

    # DevDash
    for port in "${DEVDASH_PORTS[@]}"; do
        ufw allow ${port}/tcp comment "DevDash ${port}"
    done

    # 启用
    ufw --force enable
    log_info "UFW 配置完成"
    ufw status verbose
}

setup_firewalld() {
    log_info "配置 firewalld..."

    # SSH
    firewall-cmd --permanent --add-port=${SSH_PORT}/tcp

    # Web
    for port in "${WEB_PORTS[@]}"; do
        firewall-cmd --permanent --add-port=${port}/tcp
    done

    # DevDash
    for port in "${DEVDASH_PORTS[@]}"; do
        firewall-cmd --permanent --add-port=${port}/tcp
    done

    firewall-cmd --reload
    log_info "firewalld 配置完成"
    firewall-cmd --list-all
}

setup_iptables() {
    log_info "配置 iptables..."

    # 清空现有规则
    iptables -F
    iptables -X

    # 默认策略
    iptables -P INPUT DROP
    iptables -P FORWARD DROP
    iptables -P OUTPUT ACCEPT

    # 本地回环
    iptables -A INPUT -i lo -j ACCEPT
    iptables -A OUTPUT -o lo -j ACCEPT

    # 已建立连接
    iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

    # SSH
    iptables -A INPUT -p tcp --dport ${SSH_PORT} -j ACCEPT

    # Web
    for port in "${WEB_PORTS[@]}"; do
        iptables -A INPUT -p tcp --dport ${port} -j ACCEPT
    done

    # DevDash
    for port in "${DEVDASH_PORTS[@]}"; do
        iptables -A INPUT -p tcp --dport ${port} -j ACCEPT
    done

    # 保存规则
    if command -v iptables-save > /dev/null 2>&1; then
        iptables-save > /etc/iptables/rules.v4 2>/dev/null || \
        iptables-save > /etc/iptables.rules 2>/dev/null || true
    fi

    log_info "iptables 配置完成"
    iptables -L -n -v
}

show_status() {
    echo -e "${CYAN}=== 防火墙状态 ($FW) ===${NC}"
    case "$FW" in
        ufw)
            ufw status verbose
            ;;
        firewalld)
            firewall-cmd --list-all
            ;;
        iptables)
            iptables -L -n -v
            ;;
        none)
            log_warn "未检测到防火墙工具"
            ;;
    esac
}

allow_port() {
    local port="$1"
    local proto="${2:-tcp}"
    case "$FW" in
        ufw)
            ufw allow ${port}/${proto}
            log_info "已放行端口 $port/$proto"
            ;;
        firewalld)
            firewall-cmd --permanent --add-port=${port}/${proto}
            firewall-cmd --reload
            log_info "已放行端口 $port/$proto"
            ;;
        iptables)
            iptables -A INPUT -p ${proto} --dport ${port} -j ACCEPT
            log_info "已放行端口 $port/$proto"
            ;;
    esac
}

block_port() {
    local port="$1"
    local proto="${2:-tcp}"
    case "$FW" in
        ufw)
            ufw deny ${port}/${proto}
            log_info "已封禁端口 $port/$proto"
            ;;
        firewalld)
            firewall-cmd --permanent --remove-port=${port}/${proto}
            firewall-cmd --reload
            log_info "已封禁端口 $port/$proto"
            ;;
        iptables)
            iptables -A INPUT -p ${proto} --dport ${port} -j DROP
            log_info "已封禁端口 $port/$proto"
            ;;
    esac
}

reset_firewall() {
    log_warn "重置防火墙规则..."
    case "$FW" in
        ufw)
            ufw --force reset
            ufw --force enable
            ;;
        firewalld)
            firewall-cmd --permanent --remove-all-ports
            firewall-cmd --reload
            ;;
        iptables)
            iptables -F
            iptables -X
            iptables -P INPUT ACCEPT
            iptables -P FORWARD ACCEPT
            iptables -P OUTPUT ACCEPT
            ;;
    esac
    log_info "防火墙已重置"
}

CMD="${1:-status}"

case "$CMD" in
    setup)
        case "$FW" in
            ufw)        setup_ufw ;;
            firewalld)  setup_firewalld ;;
            iptables)   setup_iptables ;;
            none)
                log_error "未检测到防火墙工具"
                echo "安装: apt install ufw 或 yum install firewalld"
                exit 1
                ;;
        esac
        ;;
    status)
        show_status
        ;;
    allow)
        if [ -z "${2:-}" ]; then
            echo "用法: $0 allow <端口> [协议]"
            exit 1
        fi
        allow_port "$2" "${3:-tcp}"
        ;;
    block)
        if [ -z "${2:-}" ]; then
            echo "用法: $0 block <端口> [协议]"
            exit 1
        fi
        block_port "$2" "${3:-tcp}"
        ;;
    reset)
        reset_firewall
        ;;
    *)
        echo "用法: $0 {setup|status|allow|block|reset}"
        echo ""
        echo "  setup            配置DevDash防火墙规则（放行9090/9091/3001/9100/8080/80/443/22）"
        echo "  status           查看防火墙状态"
        echo "  allow <端口>     放行指定端口"
        echo "  block <端口>     封禁指定端口"
        echo "  reset            重置防火墙规则"
        echo ""
        echo "DevDash 端口:"
        echo "  9090  - DevDash API"
        echo "  9091  - Prometheus"
        echo "  3001  - Grafana"
        echo "  9100  - Node Exporter"
        echo "  8080  - cAdvisor"
        echo "  80/443 - HTTP/HTTPS"
        exit 1
        ;;
esac
