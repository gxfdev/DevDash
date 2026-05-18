# DevDash - 轻量级运维面板

> 一款对标宝塔面板的轻量级运维管理工具，支持 Windows + Linux，采用「对等节点 + Agent」架构，通过公网或内网部署。

## ✨ 核心特性

| 功能 | 描述 |
|------|------|
| 🖥️ **多主机监控** | CPU / 内存 / 磁盘 / 网络 / 进程 / GPU / 温度 / 磁盘IO |
| 🏪 **应用商店** | 一键安装 Nginx / MySQL / Redis / Node.js 等 20+ 常用软件 |
| 📁 **文件管理** | 双栏浏览器 + 在线代码编辑 + 上传下载 |
| 🔥 **防火墙** | ufw / firewalld / Windows Firewall 统一管理 |
| ⏰ **计划任务** | crontab / Task Scheduler 可视化配置 |
| 🗄️ **数据库** | MySQL / PostgreSQL Web 管理界面 |
| 🔴 **告警中心** | 自定义规则引擎 + 浏览器通知 + 飞书 Webhook |
| 📊 **趋势分析** | 实时趋势图 + 历史数据 + 周报/月报 |
| 🐳 **容器化** | Docker 一键部署，镜像 < 50MB |

## 🚀 快速开始

### 方式一：Docker 部署（推荐）

```bash
git clone https://github.com/your-username/devdash.git
cd devdash
docker-compose up -d --build
```

访问：http://localhost:9090

### 方式二：本地开发

**前置要求：**
- Go 1.22+
- Node.js 18+
- npm 或 pnpm

```bash
# 1. 克隆项目
git clone https://github.com/your-username/devdash.git
cd devdash

# 2. 启动后端
cd server
go mod download
go run ./cmd/server

# 3. 新终端，启动前端
cd web
npm install
npm run dev
```

访问：
- Web UI: http://localhost:5173
- API: http://localhost:9090/api/v1

### 方式三：二进制部署

从 [Releases](https://github.com/your-username/devdash/releases) 下载对应平台：

```bash
# Linux amd64
wget https://github.com/your-username/devdash/releases/latest/download/devdash-linux-amd64.tar.gz
tar -xzf devdash-linux-amd64.tar.gz
./devdash
```

## 🔧 配置说明

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `9090` | 服务端口 |
| `JWT_SECRET` | `change-this-random-secret` | JWT 密钥（生产环境必须修改） |
| `DB_PATH` | `devdash.db` | SQLite 数据库路径 |
| `INTERVAL` | `30` | 数据采集间隔（秒） |
| `TZ` | `Asia/Shanghai` | 时区 |
| `CORS_ORIGINS` | `*` | CORS 允许的来源 |

### docker-compose.yml 示例

```yaml
services:
  devdash:
    build:
      context: .
      dockerfile: docker/Dockerfile.server
    container_name: devdash
    ports:
      - "9090:9090"
    volumes:
      - devdash-data:/data
      # 挂载自定义配置（可选）
      # - ./config.yaml:/config/config.yaml:ro
    environment:
      - PORT=9090
      - JWT_SECRET=your-random-secret-here
      - TZ=Asia/Shanghai
      - LANG=C.UTF-8
      - LC_ALL=C.UTF-8
      - GIN_MODE=release
    restart: unless-stopped

volumes:
  devdash-data:
```

## 📋 功能模块

### 1. 仪表盘 Dashboard
实时展示所有节点的 CPU、内存、磁盘使用率，支持异常高亮和快速定位。

### 2. 主机管理 Hosts
- 添加/删除被管主机
- 查看 Agent 连接状态
- 远程执行命令

### 3. 应用商店 Store
内置常用软件目录，支持一键安装/卸载/启停服务。

支持的软件类别：
- **Web 服务**: Nginx, Apache, Caddy, Tomcat
- **数据库**: MySQL, PostgreSQL, MongoDB, Redis, SQLite
- **运行时**: Node.js, Python, JDK, Go, .NET
- **工具**: Git, Vim, htop, curl, wget

### 4. 告警中心 Alerts
- 内置默认告警规则（CPU > 80%, 内存 > 85%, 磁盘 > 90%）
- 支持自定义告警规则（指标、阈值、级别）
- 多渠道通知：浏览器弹窗、飞书 Webhook
- 告警历史记录与静默功能

### 5. 文件管理 Files
- 双栏文件浏览器
- 在线编辑代码文件
- 批量上传/下载
- 权限管理

### 6. 防火墙 Firewall
统一管理 Linux (ufw/firewalld) 和 Windows 防火墙规则。

### 7. 计划任务 CronJobs
可视化创建和管理定时任务。

### 8. 终端 Terminal
Web SSH 终端，直接在浏览器中操作远程主机。

## 🛠 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 后端 | Go + Gin | 1.23+ |
| 前端 | Vue 3 + Vite + Naive UI | 3.x |
| 语言 | TypeScript | 5.x |
| 图表 | ECharts | 5.x |
| 数据库 | SQLite (默认) | modernc.org/sqlite |
| 构建 | Docker + multi-stage build | - |

## 📦 项目结构

```
DevDash/
├── server/                    # Go 后端
│   ├── cmd/server/main.go     # 入口
│   ├── internal/
│   │   ├── api/handler.go     # API 处理器
│   │   ├── collector/         # 数据采集
│   │   ├── software/          # 软件目录 & 安装器
│   │   ├── store/store.go     # SQLite 存储
│   │   ├── settings/settings.go # 系统设置
│   │   └── model/models.go    # 数据模型
│   └── migrations/            # 数据库迁移
├── web/                       # Vue 3 前端
│   ├── src/
│   │   ├── views/             # 页面组件
│   │   ├── components/        # 公共组件
│   │   ├── stores/            # Pinia 状态
│   │   └── api/client.ts      # HTTP 客户端
│   └── package.json
├── docker/                    # Docker 文件
│   ├── Dockerfile.server      # 多阶段构建
│   └── nginx.conf             # Nginx 配置
├── docker-compose.yml         # 编排配置
├── Makefile                   # 构建脚本
└── scripts/                   # 安装脚本
```

## 🔒 安全特性 (重要!)

> **生产环境部署必须阅读**: [完整安全指南](SECURITY.md)

### 已内置的安全措施

| 安全层 | 技术 | 说明 |
|--------|------|------|
| 🔐 **传输加密** | TLS 1.2/1.3 + HSTS | HTTPS 强制跳转 |
| 🛂 **身份认证** | JWT (HS256) + bcrypt | 7天有效期 + 密码哈希 |
| 🚫 **防暴力破解** | 登录速率限制 | 5次失败锁定15分钟 |
| 🛡️ **CSRF 防护** | Token 验证 | 防止跨站请求伪造 |
| 🔒 **CORS 白名单** | 域名限制 | 仅允许指定域名访问 |
| 📋 **安全头** | X-Frame-Options/CSP 等 | 防止 XSS/点击劫持 |
| 📍 **IP 白名单** | 可选配置 | 限制管理面板访问 IP |

### 快速安全部署

```bash
# 一键部署 (自动配置 HTTPS + SSL + 防火墙)
sudo ./scripts/deploy-secure.sh
```

### ⚠️ 生产环境必做清单

1. ✅ **修改默认密码** - 首次登录立即修改 admin/admin123
2. ✅ **设置强 JWT_SECRET** - `openssl rand -hex 32` 生成随机密钥
3. ✅ **启用 HTTPS** - 使用 Let's Encrypt 或自有证书
4. ✅ **配置防火墙** - 仅开放 80/443 端口
5. ✅ **定期备份** - 每日备份数据库文件
6. ✅ **监控日志** - 检查异常登录尝试

详细配置请查看 [SECURITY.md](SECURITY.md)

## ❓ 常见问题

### Q: 控制台出现错误？
A: 请确保：
- 后端服务正常运行（检查 `http://localhost:9090/api/v1/health`）
- 前端 API 地址正确（`.env.development` 或环境变量）
- 浏览器控制台无网络错误

### Q: 中文显示乱码？
A: 已修复。Docker 部署会自动设置：
- `LANG=C.UTF-8`
- `LC_ALL=C.UTF-8`
- `TZ=Asia/Shanghai`

如仍有问题，请确认终端编码为 UTF-8。

### Q: 如何添加新节点？
A: 在「主机管理」页面点击「添加节点」，输入 IP 和端口即可。

### Q: 告警不触发？
A: 检查：
1. 告警规则是否启用
2. 采集间隔是否正常
3. 节点是否在线

## 📄 License

[MIT](LICENSE)

---

<p align="center">
  Made with ❤️ by DevDash Team
</p>
