#!/bin/bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

INSTALL_DIR="/opt/devdash"
SERVICE_NAME="devdash"
MAX_RESTART_COUNT=3
RESTART_WINDOW=300

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "\n${BLUE}[$1]${NC} $2"; }

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 root 用户运行此脚本"
        exit 1
    fi
}

check_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID="${ID:-unknown}"
        OS_VERSION="${VERSION_ID:-unknown}"
    else
        OS_ID="unknown"
        OS_VERSION="unknown"
    fi
    log_info "操作系统: ${OS_ID} ${OS_VERSION}"
}

check_dependencies() {
    log_step "1/8" "检查系统依赖..."

    local missing=()

    if ! command -v curl &> /dev/null; then missing+=("curl"); fi
    if ! command -v openssl &> /dev/null; then missing+=("openssl"); fi

    if [ ${#missing[@]} -gt 0 ]; then
        log_warn "缺少依赖: ${missing[*]}"
        log_info "正在安装..."
        if command -v apt-get &> /dev/null; then
            apt-get update -qq && apt-get install -y -qq "${missing[@]}"
        elif command -v yum &> /dev/null; then
            yum install -y "${missing[@]}"
        elif command -v dnf &> /dev/null; then
            dnf install -y "${missing[@]}"
        else
            log_error "无法自动安装依赖，请手动安装: ${missing[*]}"
            exit 1
        fi
    fi

    log_info "依赖检查通过"
}

validate_env() {
    log_step "2/8" "验证环境变量..."

    local env_file="${INSTALL_DIR}/.env"
    if [ ! -f "$env_file" ]; then
        log_warn "环境变量文件不存在，正在生成..."
        generate_env
    fi

    source "$env_file"

    local required_vars=("JWT_SECRET" "PORT")
    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            log_error "缺少必要环境变量: $var"
            exit 1
        fi
    done

    if [ "${JWT_SECRET}" = "CHANGE_ME" ] || [ ${#JWT_SECRET} -lt 32 ]; then
        log_error "JWT_SECRET 不安全，请使用至少32字符的随机字符串"
        exit 1
    fi

    log_info "环境变量验证通过"
}

generate_env() {
    mkdir -p "${INSTALL_DIR}"

    local jwt_secret=$(openssl rand -hex 32)

    cat > "${INSTALL_DIR}/.env" << EOF
JWT_SECRET=${jwt_secret}
PORT=9090
TZ=Asia/Shanghai
LANG=C.UTF-8
LC_ALL=C.UTF-8
GIN_MODE=release
DB_PATH=${INSTALL_DIR}/data/devdash.db
LOG_DIR=${INSTALL_DIR}/logs
CORS_ORIGINS=https://localhost,http://localhost:9090
EOF

    chmod 600 "${INSTALL_DIR}/.env"
    log_info "环境变量文件已生成: ${INSTALL_DIR}/.env"
}

install_binary() {
    log_step "3/8" "安装服务..."

    mkdir -p "${INSTALL_DIR}/data" "${INSTALL_DIR}/logs" "${INSTALL_DIR}/ssl"

    if [ -f "./devdash-server" ]; then
        cp -f ./devdash-server "${INSTALL_DIR}/"
        chmod +x "${INSTALL_DIR}/devdash-server"
    elif [ -f "../server/devdash-server" ]; then
        cp -f ../server/devdash-server "${INSTALL_DIR}/"
        chmod +x "${INSTALL_DIR}/devdash-server"
    else
        log_info "编译后端服务..."
        if command -v go &> /dev/null; then
            cd ../server && CGO_ENABLED=0 GOOS=linux go build -o "${INSTALL_DIR}/devdash-server" ./cmd/server && cd -
        else
            log_error "找不到预编译二进制文件，且 Go 未安装"
            exit 1
        fi
    fi

    if ! id -u devdash &> /dev/null; then
        useradd -r -s /sbin/nologin -d "${INSTALL_DIR}" devdash
    fi

    chown -R devdash:devdash "${INSTALL_DIR}"
    log_info "服务安装完成"
}

install_systemd() {
    log_step "4/8" "配置 systemd 服务..."

    local service_src="$(dirname "$0")/devdash.service"
    if [ ! -f "$service_src" ]; then
        service_src="$(pwd)/deploy/devdash.service"
    fi

    if [ -f "$service_src" ]; then
        cp -f "$service_src" /etc/systemd/system/${SERVICE_NAME}.service
    else
        cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=DevDash Operations Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=devdash
Group=devdash
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/devdash-server
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=${RESTART_WINDOW}
StartLimitBurst=${MAX_RESTART_COUNT}
EnvironmentFile=${INSTALL_DIR}/.env
StandardOutput=journal
StandardError=journal
SyslogIdentifier=devdash
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    fi

    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}
    log_info "systemd 服务配置完成"
}

setup_nginx() {
    log_step "5/8" "配置 Nginx 反向代理..."

    if ! command -v nginx &> /dev/null; then
        log_warn "Nginx 未安装，正在安装..."
        if command -v apt-get &> /dev/null; then
            apt-get install -y nginx
        elif command -v yum &> /dev/null; then
            yum install -y nginx
        else
            log_warn "无法自动安装 Nginx，跳过"
            return
        fi
    fi

    read -p "请输入域名 (留空跳过 Nginx 配置): " DOMAIN
    if [ -z "$DOMAIN" ]; then
        log_warn "跳过 Nginx 配置"
        return
    fi

    local nginx_conf_src="$(dirname "$0")/../docker/nginx.ssl.conf"
    if [ ! -f "$nginx_conf_src" ]; then
        nginx_conf_src="$(pwd)/docker/nginx.ssl.conf"
    fi

    if [ -f "$nginx_conf_src" ]; then
        sed "s/your-domain.com/${DOMAIN}/g" "$nginx_conf_src" > /etc/nginx/conf.d/devdash.conf
        sed -i 's|devdash:9090|127.0.0.1:9090|g' /etc/nginx/conf.d/devdash.conf
        sed -i '/devdash2/d' /etc/nginx/conf.d/devdash.conf
        sed -i 's|upstream devdash_backend {|upstream devdash_backend {\n    server 127.0.0.1:9090 max_fails=3 fail_timeout=5s;|' /etc/nginx/conf.d/devdash.conf
    fi

    mkdir -p /etc/nginx/ssl /var/www/certbot

    read -p "是否申请 Let's Encrypt 证书? [y/N]: " USE_LE
    if [[ "$USE_LE" =~ ^[Yy]$ ]]; then
        if ! command -v certbot &> /dev/null; then
            if command -v apt-get &> /dev/null; then
                apt-get install -y certbot python3-certbot-nginx
            elif command -v yum &> /dev/null; then
                yum install -y certbot python3-certbot-nginx
            fi
        fi

        nginx -t && systemctl reload nginx
        certbot certonly --webroot -w /var/www/certbot -d "$DOMAIN" --non-interactive --agree-tos -m "admin@${DOMAIN}"

        cp /etc/letsencrypt/live/${DOMAIN}/fullchain.pem /etc/nginx/ssl/
        cp /etc/letsencrypt/live/${DOMAIN}/privkey.pem /etc/nginx/ssl/
        chmod 600 /etc/nginx/ssl/*.pem

        (crontab -l 2>/dev/null; echo "0 3 1 * * certbot renew --quiet --deploy-hook 'cp /etc/letsencrypt/live/${DOMAIN}/*.pem /etc/nginx/ssl/ && systemctl reload nginx'") | crontab -
        log_info "SSL 证书自动续期已配置"
    fi

    nginx -t && systemctl enable nginx && systemctl reload nginx
    log_info "Nginx 配置完成"
}

configure_firewall() {
    log_step "6/8" "配置防火墙..."

    if command -v ufw &> /dev/null; then
        ufw allow 80/tcp comment 'HTTP' 2>/dev/null || true
        ufw allow 443/tcp comment 'HTTPS' 2>/dev/null || true
        ufw --force enable 2>/dev/null || true
        log_info "UFW 防火墙已配置"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-service=http 2>/dev/null || true
        firewall-cmd --permanent --add-service=https 2>/dev/null || true
        firewall-cmd --reload 2>/dev/null || true
        log_info "Firewalld 已配置"
    else
        log_warn "未检测到防火墙，请手动开放 80/443 端口"
    fi
}

health_check() {
    log_step "7/8" "健康检查..."

    source "${INSTALL_DIR}/.env"
    local port="${PORT:-9090}"
    local max_attempts=10
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl -sf "http://127.0.0.1:${port}/api/v1/health" > /dev/null 2>&1; then
            log_info "健康检查通过 (尝试 ${attempt}/${max_attempts})"
            return 0
        fi
        log_warn "等待服务就绪... (尝试 ${attempt}/${max_attempts})"
        sleep 2
        attempt=$((attempt + 1))
    done

    log_error "健康检查失败，请检查日志: journalctl -u ${SERVICE_NAME} -n 50"
    return 1
}

setup_backup() {
    log_step "8/8" "配置自动备份..."

    local backup_script="${INSTALL_DIR}/backup.sh"
    cat > "$backup_script" << 'BACKUP_EOF'
#!/bin/bash
BACKUP_DIR="/opt/devdash/backups"
DB_PATH="/opt/devdash/data/devdash.db"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
MAX_BACKUPS=30

mkdir -p "$BACKUP_DIR"

if [ -f "$DB_PATH" ]; then
    sqlite3 "$DB_PATH" ".backup '${BACKUP_DIR}/db_${TIMESTAMP}.bak'" 2>/dev/null || \
        cp "$DB_PATH" "${BACKUP_DIR}/db_${TIMESTAMP}.bak"
fi

tar czf "${BACKUP_DIR}/full_${TIMESTAMP}.tar.gz" -C /opt/devdash data/ .env 2>/dev/null || true

find "$BACKUP_DIR" -name "*.bak" -mtime +7 -delete 2>/dev/null || true
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete 2>/dev/null || true

echo "[$(date)] Backup completed" >> "${BACKUP_DIR}/backup.log"
BACKUP_EOF

    chmod +x "$backup_script"
    chown devdash:devdash "$backup_script"

    (crontab -l 2>/dev/null; echo "0 2 * * * ${backup_script}") | crontab -
    log_info "每日凌晨2点自动备份已配置"
}

start_service() {
    log_info "启动服务..."
    systemctl restart ${SERVICE_NAME}
    sleep 3
    systemctl status ${SERVICE_NAME} --no-pager -l || true
}

show_result() {
    source "${INSTALL_DIR}/.env"
    local port="${PORT:-9090}"

    echo ""
    echo -e "${GREEN}=========================================="
    echo "  DevDash 部署完成！"
    echo -e "==========================================${NC}"
    echo ""
    echo "  本地访问: http://localhost:${port}"
    if [ -n "${DOMAIN:-}" ]; then
        echo "  域名访问: https://${DOMAIN}"
    fi
    echo ""
    echo "  默认账号: admin / admin123"
    echo "  ⚠️  请立即修改默认密码！"
    echo ""
    echo "  常用命令:"
    echo "    查看状态: systemctl status ${SERVICE_NAME}"
    echo "    查看日志: journalctl -u ${SERVICE_NAME} -f"
    echo "    重启服务: systemctl restart ${SERVICE_NAME}"
    echo "    手动备份: ${INSTALL_DIR}/backup.sh"
    echo ""
}

main() {
    echo -e "${BLUE}=========================================="
    echo "  DevDash 一键部署脚本 (生产环境)"
    echo -e "==========================================${NC}"

    check_root
    check_os
    check_dependencies
    validate_env
    install_binary
    install_systemd
    setup_nginx
    configure_firewall
    start_service
    health_check
    setup_backup
    show_result
}

main "$@"
