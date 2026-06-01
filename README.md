# WebShell - Linux Web 终端管理工具

<p align="center">
  <img src="https://img.shields.io/github/actions/workflow/status/gxfdev/WebShell/ci.yml?style=flat-square&label=CI" alt="CI" />
  <img src="https://img.shields.io/github/v/tag/gxfdev/WebShell?style=flat-square&label=version" alt="Version" />
  <img src="https://img.shields.io/docker/pulls/ghcr.io/gxfdev/webshell?style=flat-square" alt="Docker Pulls" />
  <img src="https://img.shields.io/github/license/gxfdev/WebShell?style=flat-square" alt="License" />
</p>

WebShell 是一个轻量级的 Linux Web 终端管理工具，提供 Web 终端、系统监控、定时任务管理、文件编辑和脚本管理功能。单二进制文件部署，零依赖，Docker 一键启动。

## ✨ 功能特性

| 功能 | 说明 |
|------|------|
| 💻 **Web 终端** | 基于 WebSocket + PTY 的浏览器终端，支持 bash/sh/zsh |
| 📊 **系统监控** | 实时 CPU、内存、磁盘、网络监控，ECharts 可视化 |
| ⏰ **定时任务** | 创建、编辑、删除定时任务，一键同步到系统 Crontab |
| 📁 **文件管理** | 浏览目录、查看/编辑/创建/删除文件和目录 |
| 📜 **脚本管理** | 编写、保存、执行 Shell/Python/Perl 脚本 |
| 🔐 **权限管理** | JWT 认证 + 角色权限（admin/user），审计日志 |

**不包含**：节点管理、软件商店、防火墙、数据库管理、容器管理

## 🏗️ 技术架构

```
┌─────────────────────────────────────────┐
│              浏览器 (Vue 3)              │
│  Naive UI + xterm.js + ECharts + Pinia  │
└──────────────┬──────────────────────────┘
               │ HTTP / WebSocket
┌──────────────┴──────────────────────────┐
│           Go 后端 (Gin)                  │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐ │
│  │ Terminal │ │ Monitor  │ │  CronMgr  │ │
│  │ PTY+WS  │ │ gopsutil │ │  crontab  │ │
│  └─────────┘ └──────────┘ └───────────┘ │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐ │
│  │ FileMgr │ │  Script  │ │   Auth    │ │
│  │   I/O   │ │  exec()  │ │ JWT+RBAC  │ │
│  └─────────┘ └──────────┘ └───────────┘ │
│              SQLite (WAL)                │
└─────────────────────────────────────────┘
```

## 🚀 快速开始

### 方式一：Docker 一键部署（推荐）

```bash
# Linux / macOS
docker run -d \
  --name webshell \
  --restart unless-stopped \
  -p 9090:9090 \
  -v webshell-data:/data \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro \
  -v /etc/hostname:/etc/hostname:ro \
  -e "JWT_SECRET=$(openssl rand -hex 32)" \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev \
  -e HOST_ETC=/host/etc \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  ghcr.io/gxfdev/webshell:latest
```

### 方式二：Docker Compose

```bash
# 下载编排文件
curl -O https://raw.githubusercontent.com/gxfdev/WebShell/main/docker-compose.ghcr.yml

# 生成密钥并启动
export JWT_SECRET=$(openssl rand -hex 32)
docker compose -f docker-compose.ghcr.yml up -d
```

### 方式三：二进制文件

```bash
# 下载最新版本
wget https://github.com/gxfdev/WebShell/releases/latest/download/webshell-linux-amd64
chmod +x webshell-linux-amd64

# 运行
export JWT_SECRET=$(openssl rand -hex 32)
./webshell-linux-amd64
```

### 方式四：源码编译

```bash
git clone https://github.com/gxfdev/WebShell.git
cd WebShell

# 编译前端
cd web && npm ci && npm run build && cd ..

# 编译后端
cd server && CGO_ENABLED=1 go build -o webshell ./cmd/server

# 运行
JWT_SECRET=$(openssl rand -hex 32) ./webshell
```

### 访问

打开浏览器访问 **http://localhost:9090**

默认账号：`admin` / `admin123`（⚠️ 首次登录请立即修改密码）

## ⚙️ 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `JWT_SECRET` | ✅ | - | JWT 签名密钥，至少 32 字符 |
| `PORT` | ❌ | `9090` | 服务监听端口 |
| `GIN_MODE` | ❌ | `release` | Gin 模式（debug/release） |
| `CORS_ORIGINS` | ❌ | `*` | CORS 允许的来源，逗号分隔 |
| `DATA_DIR` | ❌ | `/data` | 数据存储目录 |
| `DB_PATH` | ❌ | `{DATA_DIR}/webshell.db` | SQLite 数据库路径 |
| `TZ` | ❌ | `UTC` | 时区 |
| `HOST_PROC` | ❌ | - | Docker 中宿主机 /proc 挂载路径 |
| `HOST_SYS` | ❌ | - | Docker 中宿主机 /sys 挂载路径 |
| `HOST_DEV` | ❌ | - | Docker 中宿主机 /dev 挂载路径 |
| `HOST_ETC` | ❌ | - | Docker 中宿主机 /etc 挂载路径 |

## 📖 API 文档

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录获取 Token |
| GET | `/api/profile` | 获取当前用户信息 |
| PUT | `/api/password` | 修改密码 |

### 监控

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/monitor` | 获取系统状态 |
| GET | `/api/monitor/stream` | SSE 实时推送 |

### 定时任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/cron` | 列出任务 |
| POST | `/api/cron` | 创建任务 |
| PUT | `/api/cron/:id` | 更新任务 |
| DELETE | `/api/cron/:id` | 删除任务 |
| POST | `/api/cron/sync` | 同步到系统 Crontab |
| GET | `/api/cron/system` | 查看系统 Crontab |

### 文件管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/files?path=` | 列出目录 |
| GET | `/api/files/content?path=` | 读取文件 |
| PUT | `/api/files/content` | 写入文件 |
| DELETE | `/api/files?path=` | 删除文件/目录 |
| POST | `/api/files/mkdir` | 创建目录 |
| GET | `/api/files/tree?root=&depth=` | 目录树 |

### 脚本管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/scripts` | 列出脚本 |
| GET | `/api/scripts/:id` | 获取脚本 |
| POST | `/api/scripts` | 创建脚本 |
| PUT | `/api/scripts/:id` | 更新脚本 |
| DELETE | `/api/scripts/:id` | 删除脚本 |
| POST | `/api/scripts/:id/run` | 执行脚本 |

### WebSocket

| 路径 | 说明 |
|------|------|
| `/ws/terminal?token=&shell=` | Web 终端连接 |

## 🔄 CI/CD

### CI 流水线

每次推送到 `main`/`develop` 或创建 PR 时自动运行：

| Job | 说明 |
|-----|------|
| Server Lint & Vet | `gofmt` + `go vet` |
| Server Tests | CGO 编译 + 单元测试 |
| Frontend Build | `vue-tsc` 类型检查 + `vite build` |
| Docker Build Test | 构建镜像 + 健康检查 |

### CD 发布流水线

推送 `v*` 标签时自动触发：

```bash
git tag v1.0.0
git push origin v1.0.0
```

自动完成：
1. 构建 `linux/amd64` + `linux/arm64` Docker 镜像推送到 GHCR
2. 编译 Linux 二进制文件
3. 打包前端产物
4. 创建 GitHub Release

## 🖥️ CentOS 7 部署

```bash
# 1. 切换 yum 源
sudo sed -i 's|^mirrorlist=|#mirrorlist=|g' /etc/yum.repos.d/CentOS-Base.repo
sudo sed -i 's|^#baseurl=http://mirror.centos.org|baseurl=https://mirrors.aliyun.com|g' /etc/yum.repos.d/CentOS-Base.repo

# 2. 安装 Docker
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
sudo yum install -y docker-ce-20.10.24 docker-ce-cli-20.10.24 containerd.io
sudo systemctl start docker && sudo systemctl enable docker

# 3. 配置镜像加速
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<'EOF'
{"registry-mirrors": ["https://docker.1ms.run"]}
EOF
sudo systemctl daemon-reload && sudo systemctl restart docker

# 4. 运行 WebShell
docker run -d \
  --name webshell \
  --restart unless-stopped \
  -p 9090:9090 \
  -v webshell-data:/data \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro \
  -v /etc/hostname:/etc/hostname:ro \
  -e "JWT_SECRET=$(openssl rand -hex 32)" \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev \
  -e HOST_ETC=/host/etc \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  ghcr.io/gxfdev/webshell:latest

# 5. 开放端口
sudo firewall-cmd --permanent --add-port=9090/tcp
sudo firewall-cmd --reload
```

## 🔒 安全建议

- **必须修改默认密码**：首次登录后立即修改 `admin` 账户密码
- **设置强 JWT_SECRET**：至少 32 字符随机字符串
- **限制 CORS**：生产环境设置 `CORS_ORIGINS` 为具体域名
- **使用 HTTPS**：通过 Nginx/Caddy 反向代理并配置 TLS
- **定期备份**：`docker cp webshell:/data/webshell.db ./backup.db`

## 📄 许可证

MIT License
