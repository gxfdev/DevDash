#!/usr/bin/env bash
# SSL 证书续期脚本
# 用法: ./scripts/ssl-renew.sh [domain] [email]
set -euo pipefail

DOMAIN="${1:-}"
EMAIL="${2:-admin@${DOMAIN:-example.com}}"
CERT_DIR="${CERT_DIR:-/etc/letsencrypt}"
WEBROOT="${WEBROOT:-/var/www/certbot}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 certbot
check_certbot() {
    if ! command -v certbot > /dev/null 2>&1; then
        log_error "certbot 未安装"
        echo "安装: apt install certbot 或 yum install certbot"
        exit 1
    fi
}

# 获取证书过期时间
check_expiry() {
    local domain="$1"
    local cert_path="${CERT_DIR}/live/${domain}/cert.pem"

    if [ ! -f "$cert_path" ]; then
        echo "未找到"
        return
    fi

    local expiry_date
    expiry_date=$(openssl x509 -enddate -noout -in "$cert_path" 2>/dev/null | cut -d= -f2)
    local expiry_epoch
    expiry_epoch=$(date -d "$expiry_date" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$expiry_date" +%s 2>/dev/null || echo 0)
    local now_epoch
    now_epoch=$(date +%s)
    local days_left=$(( (expiry_epoch - now_epoch) / 86400 ))

    if [ $days_left -le 7 ]; then
        echo -e "${RED}${days_left}天（紧急续期！）${NC}"
    elif [ $days_left -le 30 ]; then
        echo -e "${YELLOW}${days_left}天（即将到期）${NC}"
    else
        echo -e "${GREEN}${days_left}天${NC}"
    fi
}

# 申请新证书
request_cert() {
    local domain="$1"
    log_info "申请新证书: $domain"

    mkdir -p "$WEBROOT"

    certbot certonly \
        --webroot \
        --webroot-path="$WEBROOT" \
        --email "$EMAIL" \
        --agree-tos \
        --no-eff-email \
        -d "$domain" \
        -d "www.${domain}"

    log_info "证书申请完成"
    log_info "证书路径: ${CERT_DIR}/live/${domain}/"
}

# 续期证书
renew_cert() {
    local domain="$1"
    local cert_path="${CERT_DIR}/live/${domain}/cert.pem"

    if [ ! -f "$cert_path" ]; then
        log_warn "证书不存在，执行新申请"
        request_cert "$domain"
        return
    fi

    log_info "检查证书: $domain"
    echo "  过期时间: $(check_expiry "$domain")"

    log_info "尝试续期..."
    certbot renew --cert-name "$domain" --dry-run 2>&1
    if [ $? -eq 0 ]; then
        certbot renew --cert-name "$domain"
        log_info "续期完成"
    else
        log_error "续期测试失败，请检查配置"
    fi
}

# 重载服务
reload_services() {
    log_info "重载服务..."

    # Nginx
    if systemctl is-active --quiet nginx 2>/dev/null; then
        systemctl reload nginx
        log_info "Nginx 已重载"
    fi

    # DevDash (如果使用TLS)
    if systemctl is-active --quiet devdash 2>/dev/null; then
        systemctl reload devdash 2>/dev/null || systemctl restart devdash
        log_info "DevDash 已重载"
    fi

    # Docker容器
    if docker ps -q -f name=devdash 2>/dev/null | grep -q .; then
        docker compose -f /opt/devdash/docker-compose.yml restart 2>/dev/null || true
        log_info "Docker容器已重启"
    fi
}

# 列出所有证书
list_certs() {
    echo -e "${CYAN}=== SSL 证书列表 ===${NC}"
    if [ -d "${CERT_DIR}/live" ]; then
        for domain_dir in "${CERT_DIR}/live"/*/; do
            local domain=$(basename "$domain_dir")
            local expiry=$(check_expiry "$domain")
            echo "  $domain: $expiry"
        done
    else
        log_warn "未找到证书目录: ${CERT_DIR}/live"
    fi
}

# 主逻辑
if [ -z "$DOMAIN" ]; then
    list_certs
    echo ""
    echo "用法: $0 <domain> [email]"
    echo "  $0 example.com              # 续期/申请证书"
    echo "  $0 example.com admin@x.com  # 指定邮箱"
    exit 0
fi

check_certbot
renew_cert "$DOMAIN"
reload_services
echo ""
log_info "完成！证书路径: ${CERT_DIR}/live/${DOMAIN}/"
