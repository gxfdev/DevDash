# DevDash 安全部署指南 (Windows)

## 🔒 安全特性总览

| 安全层 | 措施 | 状态 |
|--------|------|------|
| **传输加密** | TLS 1.2/1.3 + HSTS | ✅ 已实现 |
| **身份认证** | JWT (HS256) + bcrypt 密码哈希 | ✅ 已实现 |
| **访问控制** | IP 白名单 + CORS 限制 | ✅ 已实现 |
| **防攻击** | 登录速率限制 + CSRF 防护 | ✅ 已实现 |
| **安全头** | X-Frame-Options, CSP 等 | ✅ 已实现 |
| **审计日志** | Nginx 访问日志 | ✅ 已实现 |

---

## 🚀 生产环境部署步骤

### 前置要求

1. **域名** - 已解析到服务器 IP
2. **服务器** - Linux (Ubuntu 20.04+ / CentOS 8+)
3. **权限** - root 或 sudo 权限
4. **端口** - 开放 80 (HTTP) 和 443 (HTTPS)

### 方式一：一键安全部署（推荐）

```bash
# 1. 克隆仓库
git clone https://github.com/gxfdev/DevDash.git
cd DevDash

# 2. 运行安全部署脚本
sudo chmod +x scripts/deploy-secure.sh
sudo ./scripts/deploy-secure.sh

# 3. 按照提示输入:
#    - 你的域名
#    - 选择 SSL 证书类型
```

脚本会自动完成：
- ✅ 生成强随机 JWT Secret
- ✅ 配置 Let's Encrypt SSL 证书
- ✅ 设置防火墙规则
- ✅ 启动 Docker 容器
- ✅ 配置 HTTPS 强制跳转

### 方式二：手动配置

#### 1. 生成安全密钥

```bash
# 生成随机 JWT Secret (32 字节十六进制)
openssl rand -hex 32

# 输出示例: a1b2c3d4e5f6... (保存好这个值)
```

#### 2. 创建环境变量文件

```bash
cat > .env.production << 'EOF'
JWT_SECRET=你的随机密钥
PORT=9090
TZ=Asia/Shanghai
GIN_MODE=release
CORS_ORIGINS=https://your-domain.com
EOF

chmod 600 .env.production  # 仅允许当前用户读取
```

#### 3. 获取 SSL 证书

**选项 A: Let's Encrypt (免费自动续期)**

```bash
# 安装 Certbot
apt install certbot  # Ubuntu/Debian
yum install certbot  # CentOS/RHEL

# 申请证书 (确保域名已解析)
certbot certonly --standalone -d your-domain.com

# 复制证书
cp /etc/letsencrypt/live/your-domain.com/fullchain.pem ssl/
cp /etc/letsencrypt/live/your-domain.com/privkey.pem ssl/
```

**选项 B: 自有证书**

将 `fullchain.pem` 和 `privkey.pem` 放入 `ssl/` 目录。

#### 4. 配置 Nginx

编辑 `docker/nginx.ssl.conf`:

```nginx
server_name your-domain.com;  # 修改为你的域名
```

#### 5. 启动服务

```bash
export $(grep -v '^#' .env.production | xargs)
docker-compose -f docker-compose.prod.yml up -d --build
```

---

## 🛡️ 安全配置详解

### 1. HTTPS/TLS 加密

```nginx
# docker/nginx.ssl.conf 关键配置
ssl_protocols TLSv1.2 TLSv1.3;           # 只允许安全协议
ssl_ciphers 'ECDHE-...';                  # 强加密套件
add_header Strict-Transport-Security ...; # HSTS 365 天
```

### 2. JWT 认证增强

```go
// server/internal/auth/jwt.go
// - 使用 HS256 算法签名
// - Token 有效期 7 天
// - bcrypt 存储密码哈希
// - Secret 至少 32 字节随机值
```

### 3. 登录防护

```go
// server/internal/auth/security.go
// - 同一 IP 5 次失败后锁定 15 分钟
// - 自动清理过期记录
// - 防止暴力破解
```

### 4. CORS 白名单

```bash
# 仅允许指定域名访问 API
CORS_ORIGINS=https://your-domain.com
```

### 5. 安全响应头

```
X-Frame-Options: SAMEORIGIN          # 防止点击劫持
X-Content-Type-Options: nosniff       # 防止 MIME 嗅探
X-XSS-Protection: 1; mode=block       # XSS 过滤器
Content-Security-Policy: ...          # 内容安全策略
Referrer-Policy: strict-origin        # 引用策略
Permissions-Policy: camera=(), ...    # 权限策略
```

---

## 📋 部署检查清单

部署完成后，请逐项验证：

### 安全测试

- [ ] 访问 `http://domain.com` 自动跳转到 `https://`
- [ ] 浏览器显示 🔒 锁定图标
- [ ] SSL Labs 评级 A 或 A+
- [ ] 默认密码已修改
- [ ] JWT_SECRET 已设置为强随机值
- [ ] `.env.production` 文件权限为 600

### 功能测试

- [ ] 登录功能正常
- [ ] 仪表盘数据展示正常
- [ ] 所有菜单可访问
- [ ] 终端 WebSocket 连接正常
- [ ] 告警通知触发正常

---

## 🔧 运维操作

### 备份数据库

```bash
# 手动备份
cp devdash-data/devdash.db backup/devdash_$(date +%Y%m%d).db

# 定时备份 (crontab -e)
0 2 * * * cp /path/to/devdash-data/devdash.db /backup/devdash_$(date +\%Y\%m\%d).db
```

### 更新版本

```bash
git pull origin main
docker-compose -f docker-compose.prod.yml up -d --build
```

### 查看/导出日志

```bash
# 实时查看
docker-compose logs -f

# 导出 Nginx 日志
docker cp devdash-nginx:/var/log/nginx/access.log nginx_access.log
docker cp devdash-nginx:/var/log/nginx/error.log nginx_error.log
```

### 故障排查

```bash
# 检查容器状态
docker-compose ps

# 检查健康状态
curl -k https://localhost/api/v1/health

# 重启服务
docker-compose restart

# 完全重建
docker-compose down
docker-compose up -d --build
```

---

## ⚠️ 安全最佳实践

### 必须做

1. ✅ **修改默认密码** - 首次登录立即修改
2. ✅ **设置强 JWT_SECRET** - 至少 32 字节随机字符
3. ✅ **启用 HTTPS** - 不要在公网使用 HTTP
4. ✅ **定期备份数据库** - 至少每日备份
5. ✅ **更新依赖包** - 定期运行 `git pull` 更新
6. ✅ **监控异常登录** - 查看日志中的失败尝试

### 禁止做

1. ❌ **不要暴露管理面板到公网** - 除非必须且已加固
2. ❌ **不要使用弱密码** - 至少 12 位包含大小写数字特殊字符
3. ❌ **不要分享 JWT_SECRET** - 即使是团队成员
4. ❌ **不要禁用防火墙** - 保持最小权限原则
5. ❌ **不要在生产环境使用调试模式** - GIN_MODE=release

---

## 🆘 应急处理

### 密码忘记

```bash
# 进入数据库重置密码
sqlite3 devdash-data/devdash.db
UPDATE users SET password_hash='新密码的bcrypt哈希' WHERE username='admin';
```

### 可疑入侵迹象

如果发现以下情况，立即断网排查：

- 异常的大量登录失败记录
- 未知的告警规则被创建
- 数据库文件被修改
- 系统负载异常升高

应急步骤：

```bash
# 1. 立即停止服务
docker-compose down

# 2. 备份证据
cp -r devdash-data/ evidence/

# 3. 修改所有密码
# 4. 检查服务器其他服务
# 5. 分析日志寻找入侵路径
```

---

## 📞 技术支持

如遇安全问题或漏洞：

- GitHub Issues: https://github.com/gxfdev/DevDash/issues
- 安全邮箱: security@example.com (请替换)

**请勿公开披露安全漏洞，先联系维护者修复！**
