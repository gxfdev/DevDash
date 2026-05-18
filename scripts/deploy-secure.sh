#!/bin/bash

set -e

echo "=========================================="
echo "  DevDash 安全部署脚本"
echo "=========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}错误: 请使用 root 用户运行此脚本${NC}"
    echo "使用方式: sudo ./scripts/deploy-secure.sh"
    exit 1
fi

# 检查 Docker 和 Docker Compose
check_dependencies() {
    echo -e "\n${YELLOW}[1/6] 检查依赖...${NC}"
    
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}错误: Docker 未安装${NC}"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        echo -e "${RED}错误: Docker Compose 未安装${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ 依赖检查通过${NC}"
}

# 生成安全配置
generate_secure_config() {
    echo -e "\n${YELLOW}[2/6] 生成安全配置...${NC}"
    
    # 创建 SSL 目录
    mkdir -p ssl
    
    # 生成随机 JWT Secret
    JWT_SECRET=$(openssl rand -hex 32)
    
    # 创建环境变量文件
    cat > .env.production << EOF
# DevDash 生产环境配置
# 生成时间: $(date)

# 安全密钥 (请妥善保管)
JWT_SECRET=${JWT_SECRET}

# 服务端口 (内部)
PORT=9090

# 时区
TZ=Asia/Shanghai

# 编码
LANG=C.UTF-8
LC_ALL=C.UTF-8

# 运行模式
GIN_MODE=release

# CORS 允许的域名 (修改为你的域名)
CORS_ORIGINS=https://your-domain.com,http://localhost:9090
EOF

    chmod 600 .env.production
    
    echo -e "${GREEN}✓ 安全配置已生成${NC}"
    echo -e "  JWT_SECRET: ${JWT_SECRET:0:16}..."
}

# 配置 SSL 证书
setup_ssl() {
    echo -e "\n${YELLOW}[3/6] 配置 SSL 证书...${NC}"
    
    read -p "请输入域名 (例如: devdash.example.com): " DOMAIN
    
    if [ -z "$DOMAIN" ]; then
        echo -e "${RED}错误: 域名不能为空${NC}"
        exit 1
    fi
    
    # 更新 Nginx 配置中的域名
    sed -i "s/your-domain.com/${DOMAIN}/g" docker/nginx.ssl.conf
    
    # 选择证书类型
    echo -e "\n选择 SSL 证书类型:"
    echo "1) Let's Encrypt 自动申请 (推荐)"
    echo "2) 使用自有证书"
    read -p "请输入选项 [1]: " SSL_OPTION
    
    case ${SSL_OPTION:-1} in
        1)
            install_certbot
            
            # 申请证书
            echo -e "\n正在申请 Let's Encrypt 证书..."
            certbot certonly --standalone --preferred-challenges http -d $DOMAIN || {
                echo -e "${RED}证书申请失败，请检查域名 DNS 解析是否正确${NC}"
                exit 1
            }
            
            # 复制证书到项目目录
            cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem ssl/
            cp /etc/letsencrypt/live/$DOMAIN/privkey.pem ssl/
            
            echo -e "${GREEN}✓ Let's Encrypt 证书已配置${NC}"
            ;;
        2)
            echo -e "\n请将你的证书文件放到 ssl/ 目录:"
            echo "  - fullchain.pem (完整证书链)"
            echo "  - privkey.pem (私钥)"
            read -p "按回车继续..."
            
            if [ ! -f "ssl/fullchain.pem" ] || [ ! -f "ssl/privkey.pem" ]; then
                echo -e "${RED}错误: 证书文件不存在${NC}"
                exit 1
            fi
            echo -e "${GREEN}✓ 自有证书已配置${NC}"
            ;;
        *)
            echo -e "${RED}无效选项${NC}"
            exit 1
            ;;
    esac
    
    # 设置证书权限
    chmod 600 ssl/*.pem
    
    echo -e "${GREEN}✓ SSL 证书配置完成${NC}"
}

install_certbot() {
    if ! command -v certbot &> /dev/null; then
        echo -e "\n安装 Certbot..."
        
        if command -v apt-get &> /dev/null; then
            apt-get update
            apt-get install -y certbot
        elif command -v yum &> /dev/null; then
            yum install -y certbot
        else
            echo -e "${RED}无法自动安装 certbot，请手动安装${NC}"
            exit 1
        fi
    fi
}

# 配置防火墙
setup_firewall() {
    echo -e "\n${YELLOW}[4/6] 配置防火墙...${NC}"
    
    if command -v ufw &> /dev/null; then
        ufw allow 80/tcp comment 'HTTP'
        ufw allow 443/tcp comment 'HTTPS'
        ufw --force enable
        echo -e "${GREEN}✓ UFW 防火墙已配置 (开放 80, 443)${NC}"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-service=http
        firewall-cmd --permanent --add-service=https
        firewall-cmd --reload
        echo -e "${GREEN}✓ Firewalld 已配置 (开放 HTTP, HTTPS)${NC}"
    else
        echo -e "${YELLOW}⚠ 未检测到防火墙管理工具，请手动开放 80 和 443 端口${NC}"
    fi
}

# 启动服务
start_services() {
    echo -e "\n${YELLOW}[5/6] 启动服务...${NC}"
    
    # 使用生产环境配置
    export $(grep -v '^#' .env.production | xargs)
    
    # 构建并启动
    docker-compose -f docker-compose.prod.yml up -d --build
    
    # 等待服务就绪
    echo -e "\n等待服务启动..."
    sleep 10
    
    # 健康检查
    if curl -sf http://localhost:9090/api/v1/health > /dev/null; then
        echo -e "${GREEN}✓ DevDash 服务运行正常${NC}"
    else
        echo -e "${RED}⚠ 服务可能未正常启动，请查看日志: docker-compose logs${NC}"
    fi
}

# 显示部署信息
show_info() {
    echo -e "\n${YELLOW}[6/6] 显示部署信息...${NC}"
    
    cat << EOF

${GREEN}
==========================================
  🎉 DevDash 安全部署完成！
==========================================
${NC}

📍 访问地址:
   https://${DOMAIN}

🔑 默认账号:
   用户名: admin
   密码: admin123
   
   ⚠️  请立即登录并修改默认密码！

🔒 安全措施:
   ✓ TLS 1.2/1.3 加密
   ✓ HSTS 启用
   ✓ 安全头配置
   ✓ JWT 认证
   ✓ 登录速率限制
   ✓ CORS 白名单

📋 重要文件:
   .env.production - 环境变量 (包含 JWT 密钥)
   ssl/             - SSL 证书目录
   devdash-data/    - 数据库持久化目录

🛠 常用命令:
   查看日志:   docker-compose logs -f
   停止服务:   docker-compose down
   重启服务:   docker-compose restart
   备份数据:   cp -r devdash-data/ backup/
   
📝 后续操作:
   1. 登录并修改默认密码
   2. 定期备份数据库: devdash-data/devdash.db
   3. 配置自动续期 SSL 证书 (Let's Encrypt)
   4. 设置定期备份任务

${YELLOW}⚠️  安全提醒:${NC}
   - 妥善保管 .env.production 文件
   - 不要将 JWT_SECRET 泄露给他人
   - 定期更新依赖包
   - 监控异常登录尝试

EOF
}

# 主流程
main() {
    echo -e "${GREEN}开始安全部署...${NC}\n"
    
    check_dependencies
    generate_secure_config
    setup_ssl
    setup_firewall
    start_services
    show_info
}

main "$@"
