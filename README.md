<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat-square&logo=vue.js" alt="Vue" />
  <img src="https://img.shields.io/badge/TypeScript-5.x-3178C6?style=flat-square&logo=typescript" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker" alt="Docker" />
  <img src="https://img.shields.io/github/actions/workflow/status/gxfdev/DevDash/ci.yml?style=flat-square&label=CI" alt="CI" />
  <img src="https://img.shields.io/github/v/tag/gxfdev/DevDash?style=flat-square&label=version" alt="Version" />
  <img src="https://img.shields.io/badge/ghcr.io-gxfdev%2Fdevdash-blue?style=flat-square&logo=github" alt="GHCR" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License" />
</p>

<h1 align="center">DevDash - 轻量级运维监控面板</h1>

<p align="center">
  一款面向开发者和运维工程师的轻量级服务器监控与管理平台<br/>
  支持 Windows / Linux 多主机监控，采用「对等节点 + Agent」架构，可公网或内网部署
</p>

---

## 📑 目录

- [核心特性](#-核心特性)
- [功能预览](#-功能预览)
- [技术架构](#-技术架构)
- [快速开始](#-快速开始)
  - [一键 Docker 部署（推荐）](#方式一一键-docker-部署推荐)
  - [Docker Compose 部署](#方式二docker-compose-部署)
  - [本地开发](#方式三本地开发)
  - [二进制部署](#方式四二进制部署)
- [配置说明](#-配置说明)
- [功能模块](#-功能模块)
- [项目结构](#-项目结构)
- [CI/CD](#-cicd)
- [部署工具](#-部署工具)
  - [一键部署脚本](#一键部署脚本)
  - [Kubernetes 部署](#kubernetes-部署)
- [数据采集实现原理](#-数据采集实现原理)
- [API 文档](#-api-文档)
- [安全特性](#-安全特性)
- [常见问题](#-常见问题)
- [参与贡献](#-参与贡献)
- [许可证](#-许可证)

---

## ✨ 核心特性

| 功能 | 状态 | 描述 |
|------|------|------|
| 🖥️ **本机实时监控** | ✅ 已实现 | CPU / 内存 / 磁盘 / 网络 / TCP 连接 / 磁盘 I/O 速率 / 系统负载 / 主机信息 |
| 📊 **趋势分析与对比** | ✅ 已实现 | 实时趋势图 + 历史数据回溯 + 前后期趋势对比 + 异常检测 (±2σ) + 周报导出 |
| ⏱️ **监控频率配置** | ✅ 已实现 | 自定义采集间隔（3-300 秒），前后端同步生效 |
| 📁 **文件管理** | ✅ 已实现 | 双栏浏览器 + 在线代码编辑 + 上传下载 + 权限管理 |
| ⏰ **计划任务** | ✅ 已实现 | crontab / Task Scheduler 可视化配置 |
| 🔴 **告警中心** | ✅ 已实现 | 自定义规则引擎 + 飞书/钉钉/邮件/Webhook 多渠道通知 + 配置持久化 |
| 🐳 **容器监控** | ✅ 已实现 | Docker 容器列表 / 状态 / 资源使用 / 日志流 / Compose 编排 |
| 💻 **Web 终端** | ✅ 已实现 | 浏览器内终端，直接操作本机 |
| 🔐 **安全加固** | ✅ 已实现 | JWT 认证 + 速率限制 + CORS 白名单 + 安全头 |
| 🏪 **应用商店** | 🔧 开发中 | 一键安装 Nginx / MySQL / Redis / Node.js 等 20+ 常用软件 |
| 🔥 **防火墙** | 🔧 开发中 | ufw / firewalld / Windows Firewall 统一管理 |
| 🗄️ **数据库管理** | 🔧 开发中 | MySQL / PostgreSQL Web 管理界面 |
| 🌐 **多主机监控** | 📋 规划中 | Agent 远程采集 + 多节点统一管理 |
| ⚖️ **负载均衡** | 📋 规划中 | Nginx 反向代理 + 主备故障转移 |

---

## 🖼️ 功能预览

### 仪表盘 - 实时监控
实时展示本机 CPU、内存、磁盘、网络、TCP 连接等关键指标，5 秒自动刷新，异常高亮提示。

### 趋势分析 - 数据洞察
- **系统指标**：CPU / 内存 / 磁盘 I/O 速率 / 网络流量趋势图
- **趋势对比**：当前周期 vs 前期数据对比，自动计算变化趋势和幅度
- **异常检测**：基于统计学 ±2σ 方法自动检测异常数据点，标注正常范围
- **数据过滤**：系统关机或未监控期间自动断开趋势线，不显示虚假零值

---

## 🏗️ 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端浏览器                          │
│              Vue 3 + Naive UI + ECharts + Pinia             │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP / WebSocket
┌──────────────────────────▼──────────────────────────────────┐
│                   Go 后端 (Gin Framework)                    │
│                                                              │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐   │
│  │ REST API │ │ WebSocket│ │ 数据采集  │ │ 告警引擎      │   │
│  │ Handler  │ │ Terminal │ │ Collector │ │ Alert Engine  │   │
│  └────┬────┘ └────┬─────┘ └────┬─────┘ └───────┬───────┘   │
│       │           │            │                │            │
│  ┌────▼───────────▼────────────▼────────────────▼───────┐   │
│  │              SQLite 存储 (CGO-free)                   │   │
│  └──────────────────────────┬───────────────────────────┘   │
└─────────────────────────────┼───────────────────────────────┘
                              │
                        ┌─────▼─────┐
                        │ 本机节点   │
                        │ (gopsutil) │
                        └───────────┘
```

### 技术栈详情

| 层级 | 技术 | 版本 | 说明 |
|------|------|------|------|
| **后端框架** | Go + Gin | 1.22+ / v1.9 | 高性能 HTTP 框架 |
| **前端框架** | Vue 3 + Vite | 3.4+ | Composition API + SFC |
| **UI 组件** | Naive UI | 2.38+ | Vue 3 组件库 |
| **开发语言** | TypeScript | 5.x | 前端类型安全 |
| **图表可视化** | ECharts | 5.5+ | 数据可视化引擎 |
| **状态管理** | Pinia | 2.1+ | Vue 3 官方状态管理 |
| **终端模拟** | xterm.js | 5.3+ | Web 终端 |
| **系统监控** | gopsutil | v4 | 跨平台系统信息采集 |
| **数据库** | SQLite (默认) / PostgreSQL | - | 可切换存储后端 |
| **认证** | JWT (HS256) + bcrypt | v5 | 安全认证机制 |
| **容器化** | Docker multi-stage | - | 镜像 < 50MB |
| **反向代理** | Nginx + TLS | - | HTTPS + 负载均衡 |

---

## 🚀 快速开始

> 🐳 **Docker 镜像已发布到 GitHub Container Registry (GHCR)**，支持 `linux/amd64` 和 `linux/arm64` 双架构。
> 镜像地址：`ghcr.io/gxfdev/devdash` | [查看所有版本](https://github.com/gxfdev/DevDash/pkgs/container/devdash)

> ⚡ **国内加速拉取**：GHCR 在国内直连较慢，推荐使用镜像加速：
> ```bash
> # 方式1：使用南京大学镜像（免配置，直接替换前缀）
> docker pull ghcr.nju.edu.cn/gxfdev/devdash:latest
> docker tag ghcr.nju.edu.cn/gxfdev/devdash:latest ghcr.io/gxfdev/devdash:latest
>
> # 方式2：配置 Docker daemon 镜像加速（一次性配置，永久生效）
> # /etc/docker/daemon.json 添加：
> # { "registry-mirrors": ["https://ghcr.nju.edu.cn"] }
> # 然后：systemctl restart docker
> ```

### 方式一：一键 Docker 部署（推荐）

无需克隆代码，直接拉取预构建镜像运行，**30 秒内启动**：

```bash
# 0. 如已存在旧容器和旧镜像，先清理（首次部署可跳过）
docker rm -f devdash 2>/dev/null
docker rmi ghcr.io/gxfdev/devdash:latest 2>/dev/null

# Linux — 一键启动（完整功能：文件管理+脚本执行+定时任务）
docker run -d \
  --name devdash \
  --restart unless-stopped \
  --pid=host \
  --cap-add SYS_PTRACE \
  --cap-add SYS_ADMIN \
  --cap-add SYS_CHROOT \
  -p 9090:9090 \
  -v devdash-data:/data \
  -v /:/host:rw \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro \
  -v /etc/hostname:/etc/hostname:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e "JWT_SECRET=$(openssl rand -hex 32)" \
  -e HOST_ROOT=/host \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev \
  -e HOST_ETC=/host/etc \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  ghcr.io/gxfdev/devdash:latest
```

```powershell
# Windows PowerShell — 一键启动
docker run -d `
  --name devdash `
  --restart unless-stopped `
  -p 9090:9090 `
  -v devdash-data:/data `
  -e "JWT_SECRET=$(-join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) }))" `
  -e GIN_MODE=release `
  -e TZ=Asia/Shanghai `
  ghcr.io/gxfdev/devdash:latest
```

访问：**http://localhost:9090**  
默认账号：`admin` / `admin123`（首次登录请立即修改密码）

> 💡 **提示**：Windows 上不需要挂载 `/proc`、`/sys` 等目录，Docker 管理功能也不可用（无需挂载 `docker.sock`）。

#### 使用 Docker Compose 一键部署（更推荐）

```bash
# 1. 下载编排文件
curl -O https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.ghcr.yml

# 2. 生成密钥并启动
export JWT_SECRET=$(openssl rand -hex 32)
docker compose -f docker-compose.ghcr.yml up -d
```

```powershell
# Windows PowerShell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.ghcr.yml -OutFile docker-compose.ghcr.yml
$env:JWT_SECRET = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
docker compose -f docker-compose.ghcr.yml up -d
```

#### CentOS 7 部署（NAT 模式虚拟机）

CentOS 7 已 EOL，需切换镜像源后安装 Docker：

```bash
# 1. 切换 yum 源（官方源已下线）
sudo sed -i 's|^mirrorlist=|#mirrorlist=|g' /etc/yum.repos.d/CentOS-Base.repo
sudo sed -i 's|^#baseurl=http://mirror.centos.org|baseurl=https://mirrors.aliyun.com|g' /etc/yum.repos.d/CentOS-Base.repo

# 2. 安装 Docker（兼容版本 20.10.x）
sudo yum remove -y docker docker-client docker-common docker-engine 2>/dev/null
sudo yum install -y yum-utils device-mapper-persistent-data lvm2
sudo yum-config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
sudo yum install -y docker-ce-20.10.24 docker-ce-cli-20.10.24 containerd.io
sudo systemctl start docker && sudo systemctl enable docker

# 3. 配置镜像加速（国内网络必须）
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json <<'EOF'
{
  "registry-mirrors": ["https://docker.1ms.run", "https://docker.xuanyuan.me"]
}
EOF
sudo systemctl daemon-reload && sudo systemctl restart docker

# 4. 拉取并运行 DevDash
docker run -d \
  --name devdash \
  --restart unless-stopped \
  -p 9090:9090 \
  -v devdash-data:/data \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro \
  -v /etc/hostname:/etc/hostname:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e "JWT_SECRET=$(openssl rand -hex 32)" \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev \
  -e HOST_ETC=/host/etc \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  ghcr.io/gxfdev/devdash:latest

# 5. 开放防火墙端口
sudo firewall-cmd --permanent --add-port=9090/tcp
sudo firewall-cmd --reload

# 6. 查看虚拟机 IP
ip addr show | grep "inet " | grep -v 127.0.0.1
```

然后在 Windows 宿主机浏览器访问 `http://虚拟机IP:9090`。  
如果 NAT 模式下无法直接访问，需在 VMware/VirtualBox 中配置端口转发（主机端口 9090 → 虚拟机端口 9090），然后访问 `http://localhost:9090`。

#### 指定版本

```bash
# 使用特定版本（推荐生产环境固定版本）
docker pull ghcr.io/gxfdev/devdash:v1.8.1
docker run -d ... ghcr.io/gxfdev/devdash:v1.8.1

# 查看所有可用版本
# https://github.com/gxfdev/DevDash/pkgs/container/devdash
```

#### 带监控面板部署（Prometheus + Grafana）

```bash
# 1. 下载编排文件（主服务 + 监控栈）
curl -O https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.ghcr.yml
curl -O https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.monitoring.yml
curl -O -o docker/prometheus.yml --create-dirs https://raw.githubusercontent.com/gxfdev/DevDash/main/docker/prometheus.yml
curl -O -o docker/alert-rules.yml https://raw.githubusercontent.com/gxfdev/DevDash/main/docker/alert-rules.yml

# 2. 生成密钥并启动（主服务 + 监控）
export JWT_SECRET=$(openssl rand -hex 32)
export GRAFANA_PASSWORD=$(openssl rand -hex 8)
docker compose -f docker-compose.ghcr.yml -f docker-compose.monitoring.yml up -d

# 3. 访问各服务
# DevDash:      http://localhost:9090  (admin / admin123)
# Grafana:      http://localhost:3001  (admin / $GRAFANA_PASSWORD)
# Prometheus:   http://localhost:9091
# cAdvisor:     http://localhost:8080
```

#### 常用管理命令

```bash
docker ps                          # 查看运行状态
docker logs -f devdash             # 查看实时日志
docker restart devdash             # 重启容器

# 更新到最新版（清理旧容器+旧镜像，重新部署完整功能）
# 国内用户可用 ghcr.nju.edu.cn 替换 ghcr.io 加速拉取
docker pull ghcr.io/gxfdev/devdash:latest && \
docker rm -f devdash && \
docker run -d --name devdash --restart unless-stopped \
  --pid=host --cap-add SYS_PTRACE --cap-add SYS_ADMIN --cap-add SYS_CHROOT \
  -p 9090:9090 \
  -v devdash-data:/data -v /:/host:rw \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro -v /etc/hostname:/etc/hostname:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e "JWT_SECRET=$(openssl rand -hex 32)" -e HOST_ROOT=/host \
  -e HOST_PROC=/host/proc -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev -e HOST_ETC=/host/etc \
  -e GIN_MODE=release -e TZ=Asia/Shanghai ghcr.io/gxfdev/devdash:latest

# 清理悬空的旧镜像（释放磁盘空间）
docker image prune -f

# 备份数据
docker cp devdash:/data/devdash.db ./backup-$(date +%Y%m%d).db
```

### 方式二：Docker Compose 部署

从源码构建镜像，适合需要自定义修改的场景：

```bash
# 1. 克隆项目
git clone https://github.com/gxfdev/DevDash.git
cd DevDash

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 JWT_SECRET 等

# 3. 启动服务（含 Nginx 反向代理）
docker compose up -d --build
```

访问：`http://localhost:80`（Nginx 代理）或 `http://localhost:9090`（直连后端）

#### 生产环境（含 TLS + PostgreSQL + 负载均衡）

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

访问：`https://your-domain.com`

#### 开发环境（含热重载）

```bash
docker compose -f docker-compose.dev.yml up -d --build
```

### 方式三：本地开发

**前置要求：**
- Go 1.26+
- Node.js 20+
- npm 11+

```bash
# 1. 克隆项目
git clone https://github.com/gxfdev/DevDash.git
cd DevDash

# 2. 配置环境变量
cp .env.example .env.local
# .env.local 已包含开发默认值，无需额外修改

# 3. 一键启动（Linux/macOS）
bash dev.sh start

# 3. 一键启动（Windows PowerShell）
.\dev.ps1 start
```

访问：
- Web UI: http://localhost:5173
- API: http://localhost:9090/api/v1

**脚本参数：**

| 参数 | 说明 |
|------|------|
| `SKIP_BACKEND=1` | 仅启动前端 |
| `SKIP_FRONTEND=1` | 仅启动后端 |
| `HOT_RELOAD=0` | 禁用后端热重载 |
| `VITE_BACKEND_URL` | 自定义后端地址 |

```bash
# 示例：仅启动后端
SKIP_FRONTEND=1 bash dev.sh start

# 查看服务状态
bash dev.sh status

# 查看日志
bash dev.sh logs backend

# 停止所有服务
bash dev.sh stop
```

#### 手动启动

```bash
# 1. 启动后端
cd server
go mod download
JWT_SECRET=devdash-dev-secret-key-min-32ch!! ENCRYPTION_KEY=devdash-encryption-key-32ch!! go run ./cmd/server

# 2. 新终端，启动前端
cd web
npm install
VITE_BACKEND_URL=http://localhost:9090 npm run dev
```

### 方式四：二进制部署

从 [Releases](https://github.com/gxfdev/DevDash/releases) 下载对应平台：

```bash
# Linux amd64
wget https://github.com/gxfdev/DevDash/releases/latest/download/devdash-linux-amd64
chmod +x devdash-linux-amd64
JWT_SECRET=$(openssl rand -hex 32) ./devdash-linux-amd64

# Linux arm64 (树莓派等)
wget https://github.com/gxfdev/DevDash/releases/latest/download/devdash-linux-arm64
chmod +x devdash-linux-arm64
JWT_SECRET=$(openssl rand -hex 32) ./devdash-linux-arm64

# macOS (Apple Silicon)
wget https://github.com/gxfdev/DevDash/releases/latest/download/devdash-darwin-arm64
chmod +x devdash-darwin-arm64
JWT_SECRET=$(openssl rand -hex 32) ./devdash-darwin-arm64

# Windows
# 下载 devdash-windows-amd64.exe，然后：
set JWT_SECRET=your-random-secret-here
devdash-windows-amd64.exe
```

> ⚠️ **注意**：Windows 二进制需要 CGO 编译（ConPTY 终端支持），需安装 MinGW-w64 或使用 Visual Studio Build Tools。

---

## 🔧 配置说明

### 环境变量

| 变量 | 默认值 | 必填 | 说明 |
|------|--------|------|------|
| `PORT` | `9090` | 否 | 服务监听端口 |
| `JWT_SECRET` | `change-this-random-secret` | ✅ **是** | JWT 签名密钥（**生产环境必须修改**，建议 `openssl rand -hex 32`） |
| `ENCRYPTION_KEY` | - | 否 | 数据加密密钥（32 字节） |
| `DB_TYPE` | `sqlite` | 否 | 数据库类型：`sqlite` 或 `postgres` |
| `DB_PATH` | `devdash.db` | 否 | SQLite 数据库路径（仅 SQLite 模式） |
| `DB_HOST` | `localhost` | 否 | PostgreSQL 主机 |
| `DB_PORT` | `5432` | 否 | PostgreSQL 端口 |
| `DB_USER` | `devdash` | 否 | PostgreSQL 用户名 |
| `DB_PASSWORD` | - | PostgreSQL 时必填 | PostgreSQL 密码 |
| `DB_NAME` | `devdash` | 否 | PostgreSQL 数据库名 |
| `DB_SSLMODE` | `disable` | 否 | PostgreSQL SSL 模式 |
| `INTERVAL` | `5` | 否 | 数据采集间隔（秒） |
| `TZ` | `Asia/Shanghai` | 否 | 时区 |
| `GIN_MODE` | `debug` | 否 | Gin 运行模式：`debug` 或 `release` |
| `CORS_ORIGINS` | `*` | 否 | CORS 允许的来源（**生产环境建议限制**） |
| `STATIC_DIR` | - | 否 | 前端静态文件目录（二进制部署时使用） |
| `AGENT_TOKEN` | - | 否 | Agent 认证令牌 |
| `HOST_PROC` | - | Docker 时建议 | 宿主机 /proc 挂载路径 |
| `HOST_SYS` | - | Docker 时建议 | 宿主机 /sys 挂载路径 |
| `HOST_DEV` | - | Docker 时建议 | 宿主机 /dev 挂载路径 |
| `HOST_ETC` | - | Docker 时建议 | 宿主机 /etc 挂载路径 |

### Docker Compose 配置文件对比

| 文件 | 用途 | 镜像来源 | 包含服务 |
|------|------|----------|----------|
| `docker-compose.ghcr.yml` | **一键部署（推荐）** | GHCR 预构建镜像 | DevDash |
| `docker-compose.yml` | 源码构建部署 | 本地构建 | DevDash + Nginx + PostgreSQL |
| `docker-compose.prod.yml` | 生产环境 | 本地构建 | DevDash + Nginx (TLS) + PostgreSQL + 备用节点 |
| `docker-compose.dev.yml` | 开发环境 | 本地构建 | DevDash (热重载) + 前端 (Vite Dev) |

### docker-compose.yml 示例

```yaml
services:
  devdash:
    image: ghcr.io/gxfdev/devdash:latest
    container_name: devdash
    ports:
      - "9090:9090"
    volumes:
      - devdash-data:/data
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /dev:/host/dev:ro
      - /etc/hostname:/etc/hostname:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - PORT=9090
      - JWT_SECRET=your-random-secret-here
      - TZ=Asia/Shanghai
      - GIN_MODE=release
      - HOST_PROC=/host/proc
      - HOST_SYS=/host/sys
      - HOST_DEV=/host/dev
      - HOST_ETC=/host/etc
    restart: unless-stopped

volumes:
  devdash-data:
```

---

## 📋 功能模块

### 1. 📊 仪表盘 Dashboard
实时展示所有节点的关键指标，5 秒自动刷新：

| 指标 | 说明 |
|------|------|
| CPU 使用率 | 实时百分比 + 告警线 |
| 内存使用率 | 已用/总量 + 百分比 |
| 磁盘使用率 | 各分区使用情况 |
| 磁盘 I/O 速率 | 读取/写入速率 (MB/s) |
| 网络流量 | 入站/出站速率 (MB/s) |
| TCP 连接 | 各状态连接数统计 |
| 系统负载 | 1/5/15 分钟平均负载 |
| 主机信息 | 操作系统、内核版本、运行时间、CPU 型号 |

### 2. 📈 趋势分析 Trends
- **系统指标**：CPU / 内存 / 磁盘 I/O 速率 / 网络流量趋势图
- **趋势对比**：当前周期 vs 前期数据对比，自动计算变化趋势和幅度
- **异常检测**：基于 ±2σ 统计方法自动检测异常数据点，标注正常范围和均值线
- **周报摘要**：一键导出文本格式报告

### 3. 🖥️ 主机管理 Hosts（规划中）
> 当前仅支持本机监控，多主机 Agent 远程采集功能规划中。

### 4. 🏪 应用商店 Store（开发中）

支持的软件类别：
- **Web 服务**: Nginx, Apache, Caddy, Tomcat
- **数据库**: MySQL, PostgreSQL, MongoDB, Redis, SQLite
- **运行时**: Node.js, Python, JDK, Go, .NET
- **工具**: Git, Vim, htop, curl, wget

### 5. 🔴 告警中心 Alerts
- 内置默认告警规则（CPU > 80%, 内存 > 85%, 磁盘 > 90%）
- 支持自定义告警规则（指标、阈值、级别）
- **多渠道通知**：
  - 🔔 浏览器弹窗通知
  - 📱 飞书机器人（Webhook 卡片消息）
  - 📱 钉钉机器人（Webhook + 加签验证）
  - 📧 邮件通知（SMTP）
  - 🔗 自定义 Webhook
- 告警历史记录与静默功能
- **可视化通知配置**（告警中心 → 通知配置 Tab）
- **一键测试告警**推送
- 配置持久化存储，重启不丢失

#### 飞书机器人接入指南
1. 打开飞书群 → 群设置 → 群机器人 → 添加机器人 → 自定义机器人
2. 复制 Webhook URL（格式：`https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx`）
3. 在 DevDash 告警中心 → 通知配置 → 开启"飞书机器人" → 填入 URL
4. 点击"保存配置" → "发送测试告警"验证

#### 钉钉机器人接入指南
1. 打开钉钉群 → 群设置 → 智能群助手 → 添加机器人 → 自定义
2. 安全设置选择"加签"，复制加签密钥（以 `SEC` 开头）
3. 复制 Webhook URL（格式：`https://oapi.dingtalk.com/robot/send?access_token=xxxxx`）
4. 在 DevDash 告警中心 → 通知配置 → 开启"钉钉机器人" → 填入 URL 和密钥
5. 点击"保存配置" → "发送测试告警"验证

### 6. 📁 文件管理 Files
- 双栏文件浏览器
- 在线编辑代码文件（语法高亮）
- 批量上传/下载
- 权限管理

### 7. 🔥 防火墙 Firewall（开发中）

### 8. ⏰ 计划任务 CronJobs
可视化创建和管理定时任务，支持 crontab 和 Windows Task Scheduler。

### 9. 💻 Web 终端 Terminal
基于 xterm.js 的 Web 终端，直接在浏览器中操作本机。

**终端特性：**
- 跨平台支持：Linux (PTY) / Windows (ConPTY)
- WebSocket 实时通信，JWT 认证
- 支持自定义 Shell（bash / zsh / PowerShell / cmd）
- 连接超时检测 + 自动重连
- 错误码反馈（认证失败、节点离线、权限不足）

### 10. 🐳 容器管理 Docker
- 容器列表与状态监控
- 容器资源使用统计
- 实时日志流
- Docker Compose 编排
- 容器监控面板

### 11. 🗄️ 数据库管理 Database
MySQL / PostgreSQL Web 管理界面，支持 SQL 查询执行。

### 12. ⚙️ 系统设置 Settings
- 个人设置：用户名、密码修改
- 系统设置：采集间隔、告警阈值、通知渠道配置

---

## 📦 项目结构

```
DevDash/
├── server/                          # Go 后端
│   ├── cmd/server/main.go           # 程序入口
│   ├── internal/
│   │   ├── api/                     # API 处理器
│   │   │   ├── handler.go           # 核心 API（仪表盘、趋势、历史、终端 WS）
│   │   │   ├── monitor_handler.go   # 监控相关 API
│   │   │   ├── agent_handler.go     # Agent 通信 API
│   │   │   ├── docker_handler.go    # Docker 管理 API
│   │   │   └── container_monitor_handler.go  # 容器监控 API
│   │   ├── auth/                    # 认证与安全
│   │   │   ├── jwt.go               # JWT 令牌管理
│   │   │   ├── middleware.go        # 认证中间件（HTTP + WebSocket）
│   │   │   ├── security.go          # 安全策略
│   │   │   └── validation.go        # 输入校验
│   │   ├── collector/               # 系统数据采集
│   │   │   ├── collector.go         # 核心采集器
│   │   │   └── gpu_collector.go     # GPU 信息采集
│   │   ├── store/store.go           # 数据库操作
│   │   ├── model/models.go          # 数据模型
│   │   ├── alert/engine.go          # 告警引擎
│   │   ├── node/manager.go          # 节点管理
│   │   ├── docker/                  # Docker 管理
│   │   │   ├── manager.go           # 容器管理
│   │   │   ├── compose.go           # Compose 编排
│   │   │   ├── container_monitor.go # 容器监控
│   │   │   └── websocket_stream.go  # 日志 WebSocket
│   │   ├── terminal/                # Web 终端
│   │   │   ├── terminal.go          # 终端核心（连接管理、消息转发）
│   │   │   ├── terminal_unix.go     # Linux/macOS PTY 实现
│   │   │   └── terminal_windows.go  # Windows ConPTY 实现
│   │   ├── filemgr/filemgr.go       # 文件管理
│   │   ├── firewall/firewall.go     # 防火墙管理
│   │   ├── cronjob/cronjob.go       # 计划任务
│   │   ├── dbmgr/dbmgr.go           # 数据库管理
│   │   ├── software/                # 软件商店
│   │   │   ├── catalog.go           # 软件目录
│   │   │   └── installer.go         # 安装器
│   │   ├── settings/settings.go     # 系统设置
│   │   ├── exporter/exporter.go     # Prometheus 导出
│   │   ├── ha/ha.go                 # 高可用
│   │   ├── kubernetes/monitor.go    # K8s 监控
│   │   ├── config/config.go         # 配置管理
│   │   ├── logger/logger.go         # 日志管理
│   │   └── middleware/encoding.go   # 中间件
│   ├── migrations/                  # 数据库迁移
│   │   ├── 001_init.sql             # 初始化表结构
│   │   └── 002_metrics.sql          # 指标表
│   └── tests/                       # 集成测试
├── web/                             # Vue 3 前端
│   ├── src/
│   │   ├── views/                   # 页面组件
│   │   │   ├── DashboardView.vue    # 仪表盘
│   │   │   ├── TrendsView.vue       # 趋势分析
│   │   │   ├── HostListView.vue     # 主机列表
│   │   │   ├── HostDetailView.vue   # 主机详情
│   │   │   ├── AlertView.vue        # 告警中心
│   │   │   ├── DockerView.vue       # Docker 管理
│   │   │   ├── ContainerMonitorView.vue  # 容器监控
│   │   │   ├── FileMgrView.vue      # 文件管理
│   │   │   ├── FirewallView.vue     # 防火墙
│   │   │   ├── CronView.vue         # 计划任务
│   │   │   ├── DatabaseView.vue     # 数据库管理
│   │   │   ├── TerminalView.vue     # Web 终端
│   │   │   ├── StoreView.vue        # 应用商店
│   │   │   ├── SettingsView.vue     # 系统设置
│   │   │   └── LoginView.vue        # 登录页
│   │   ├── components/              # 公共组件
│   │   │   ├── AppLayout.vue        # 布局组件
│   │   │   └── MetricCard.vue       # 指标卡片
│   │   ├── stores/                  # Pinia 状态管理
│   │   │   ├── auth.ts              # 认证状态
│   │   │   ├── snapshot.ts          # 快照数据
│   │   │   ├── nodes.ts             # 节点管理
│   │   │   └── metrics.ts           # 指标数据
│   │   ├── api/                     # API 客户端
│   │   │   ├── client.ts            # Axios 实例
│   │   │   └── index.ts             # API 接口
│   │   ├── types/index.ts           # TypeScript 类型定义
│   │   ├── router/index.ts          # 路由配置
│   │   ├── composables/useTheme.ts  # 主题切换
│   │   └── utils/sanitize.ts        # 工具函数
│   └── package.json
├── docker/                          # Docker 文件
│   ├── Dockerfile.server            # 后端多阶段构建（< 50MB）
│   ├── Dockerfile.agent             # Agent 构建
│   ├── nginx.conf                   # Nginx 开发配置
│   ├── nginx.ssl.conf               # Nginx TLS + 负载均衡配置
│   ├── init-db/01-init.sql          # PostgreSQL 初始化
│   └── build-multiarch.sh           # 多架构构建脚本
├── .github/workflows/               # GitHub Actions
│   ├── ci.yml                       # CI 流水线
│   └── release.yml                  # CD 发布流水线
├── docker-compose.ghcr.yml          # 一键部署（GHCR 预构建镜像）
├── docker-compose.yml               # 源码构建部署
├── docker-compose.prod.yml          # 生产环境编排（含 TLS + 负载均衡）
├── docker-compose.dev.yml           # 开发环境编排（含热重载）
├── .env.example                     # 环境变量模板
├── .env.prod                        # 生产环境配置模板
├── dev.sh / dev.ps1                 # 开发启动脚本
├── deploy.sh / deploy.ps1           # 部署脚本
├── Makefile                         # 构建命令
├── SECURITY.md                      # 安全指南
└── LICENSE                          # MIT 许可证
```

---

## 🔄 CI/CD

### CI 流水线（`.github/workflows/ci.yml`）

每次推送到 `main` / `develop` 分支或创建 PR 时自动运行：

```
┌──────────────┐    ┌────────────────┐
│  Lint & Vet  │───▶│  Server Tests  │───┐
└──────────────┘    └────────────────┘   │
┌──────────────┐                          │    ┌──────────────┐
│Security Scan │                          ├───▶│ Docker Build │───▶ CI Success
└──────────────┘                          │    └──────────────┘
┌──────────────┐    ┌────────────────┐   │
│  Frontend    │───▶│  Build & Type  │───┘
│  Build       │    │  Check         │
└──────────────┘    └────────────────┘
┌──────────────────────┐
│ Windows Tests (CGO)  │──────────────────────▶ CI Success
└──────────────────────┘
```

| Job | 运行环境 | 说明 |
|-----|---------|------|
| **Server Lint & Vet** | Ubuntu | `gofmt` 格式检查 + `go vet` 静态分析 |
| **Security Scan** | Ubuntu | `go mod verify` 依赖完整性 + `govulncheck` 漏洞扫描 |
| **Server Tests** | Ubuntu | `go build` + `go test -race` + 覆盖率报告 |
| **Server Tests (Windows)** | Windows | CGO 构建（ConPTY）+ 单元测试 |
| **Frontend Build** | Ubuntu | `vue-tsc --noEmit` 类型检查 + `vite build` 构建 |
| **Docker Build Test** | Ubuntu | 构建镜像 + 健康检查验证 |

**并发控制**：同一分支的 CI 运行会自动取消旧的运行，节省资源。

### CD 发布流水线（`.github/workflows/release.yml`）

推送 `v*` 标签时自动触发：

```bash
# 创建发布
git tag v1.0.0
git push origin v1.0.0
```

```
┌─────────────────────┐
│  Build & Push       │───▶ ghcr.io/gxfdev/devdash:v1.0.0
│  Docker Image       │───▶ ghcr.io/gxfdev/devdash:1.0
│  (amd64 + arm64)    │───▶ ghcr.io/gxfdev/devdash:latest
└─────────────────────┘
┌─────────────────────┐
│  Build Binaries     │───▶ devdash-linux-amd64
│  (Linux/macOS)      │───▶ devdash-linux-arm64
│                     │───▶ devdash-darwin-amd64
│                     │───▶ devdash-darwin-arm64
└─────────────────────┘
┌─────────────────────┐
│  Build Windows      │───▶ devdash-windows-amd64.exe
│  Binary (CGO)       │     (ConPTY 终端支持)
└─────────────────────┘
┌─────────────────────┐
│  Build Frontend     │───▶ frontend-dist.tar.gz
│  Dist               │     (可配合二进制部署使用)
└─────────────────────┘
         │
         ▼
┌─────────────────────┐
│  Create GitHub      │───▶ Release 页面 + 所有产物
│  Release            │     + 自动生成 Release Notes
└─────────────────────┘
```

**Docker 镜像标签策略：**

| 标签 | 示例 | 说明 |
|------|------|------|
| `{{version}}` | `v1.0.0` | 完整版本号 |
| `{{major}}.{{minor}}` | `1.0` | 次版本号（自动获取补丁更新） |
| `latest` | - | 最新稳定版 |
| `sha-xxxxxx` | `sha-abc1234` | Git commit SHA |

---

## � 容器化部署详解

### 容器化概念与优势

**容器化**是将应用程序及其所有依赖项打包到一个标准化单元（容器）中的技术，确保应用在任何环境中都能一致运行。

**核心优势**：
- **环境一致性**：开发、测试、生产环境完全相同，消除"在我机器上能跑"问题
- **快速启动**：容器秒级启动，相比虚拟机快100倍
- **资源高效**：共享主机内核，无Guest OS开销，密度提升5-10倍
- **版本可控**：镜像版本化管理，支持快速回滚
- **弹性伸缩**：结合Kubernetes实现自动扩缩容
- **微服务友好**：每个服务独立容器，独立部署和升级

**常用容器技术对比**：

| 技术 | 类型 | 特点 | 适用场景 |
|------|------|------|----------|
| **Docker** | 容器运行时 | 最流行，生态丰富，简单易用 | 开发、测试、单机生产 |
| **containerd** | 容器运行时 | 轻量级，K8s默认运行时 | Kubernetes集群 |
| **Podman** | 容器引擎 | 无守护进程，rootless | 安全要求高的环境 |
| **Kubernetes** | 容器编排 | 自动扩缩容、服务发现、滚动更新 | 大规模生产集群 |

### 镜像构建（多阶段Dockerfile）

DevDash采用**三阶段构建**，最终镜像仅约30MB：

```
阶段1: node:20-alpine     → 构建前端静态文件
阶段2: golang:1.26-alpine → 编译Go二进制
阶段3: alpine:3.21        → 运行时（最小化）
```

**优化策略**：
- 多阶段构建：构建工具不进入最终镜像
- `-ldflags="-s -w"`：去除调试信息，二进制减小40%
- `-trimpath`：去除文件路径信息
- `rm -rf node_modules`：构建后清理缓存
- `.dockerignore`：排除.git、测试、文档等无关文件
- Alpine基础镜像：仅5MB，比Ubuntu小20倍

**本地构建**：

```bash
# 构建镜像
docker build -f docker/Dockerfile.server -t devdash:latest .

# 查看镜像大小
docker images devdash:latest

# 运行容器
docker run -d --name devdash --restart unless-stopped \
  --pid=host --cap-add SYS_PTRACE --cap-add SYS_ADMIN --cap-add SYS_CHROOT \
  -p 9090:9090 \
  -v devdash-data:/data \
  -v /:/host:rw -v /proc:/host/proc:ro -v /sys:/host/sys:ro \
  -v /dev:/host/dev:ro -v /etc/hostname:/etc/hostname:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e HOST_ROOT=/host -e HOST_PROC=/host/proc -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev -e HOST_ETC=/host/etc \
  -e JWT_SECRET=your-secret -e TZ=Asia/Shanghai \
  devdash:latest
```

### 镜像推送到 GitHub Container Registry (GHCR)

#### 1. 认证配置

```bash
# 创建 Personal Access Token
# 访问 https://github.com/settings/tokens
# 勾选权限: write:packages, read:packages

# 登录 GHCR
export GHCR_TOKEN=ghp_your_token_here
echo $GHCR_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

#### 2. 标签规范

DevDash遵循 [Semantic Versioning](https://semver.org/)：

| 标签 | 说明 | 示例 |
|------|------|------|
| `latest` | 最新稳定版 | `ghcr.io/gxfdev/devdash:latest` |
| `vX.Y.Z` | 完整版本号 | `ghcr.io/gxfdev/devdash:v1.8.1` |
| `X.Y` | 主次版本 | `ghcr.io/gxfdev/devdash:1.7` |
| `X` | 主版本 | `ghcr.io/gxfdev/devdash:1` |
| `sha-xxxxxx` | Git提交哈希 | `ghcr.io/gxfdev/devdash:sha-2232c4f` |

#### 3. 推送命令

```bash
# 方式一：使用脚本（推荐）
chmod +x docker/build-push-ghcr.sh
./docker/build-push-ghcr.sh v1.8.1

# 方式二：手动操作
docker tag devdash:latest ghcr.io/gxfdev/devdash:v1.8.1
docker tag devdash:latest ghcr.io/gxfdev/devdash:latest
docker push ghcr.io/gxfdev/devdash:v1.8.1
docker push ghcr.io/gxfdev/devdash:latest
```

#### 4. CI/CD 自动推送

推送git tag自动触发GitHub Actions构建并推送（见 [`.github/workflows/release.yml`](docker/../.github/workflows/release.yml)）：

```bash
git tag v1.8.1
git push origin v1.8.1
# → 自动构建 linux/amd64 + linux/arm64 双架构镜像
# → 推送到 ghcr.io/gxfdev/devdash:v1.8.1
# → 创建 GitHub Release
```

#### 5. 拉取镜像

```bash
# 拉取最新版
docker pull ghcr.io/gxfdev/devdash:latest

# 拉取指定版本
docker pull ghcr.io/gxfdev/devdash:v1.8.1
```

---

## 📊 容器监控方案

### 监控架构

```
┌─────────────────────────────────────────────────────────┐
│                    Grafana (可视化)                      │
│                   http://localhost:3001                  │
└────────────────────────┬────────────────────────────────┘
                         │ 查询
┌────────────────────────▼────────────────────────────────┐
│                Prometheus (时序数据库)                   │
│                   http://localhost:9091                 │
│              存储30天 / 最大5GB / 15s采集               │
└───┬───────────────┬───────────────┬────────────────────┘
    │ scrape        │ scrape        │ scrape
┌───▼──────┐  ┌────▼──────┐  ┌────▼──────┐
│ DevDash  │  │  Node     │  │ cAdvisor  │
│ /metrics │  │ Exporter  │  │           │
│ :9090    │  │ :9100     │  │ :8080     │
└──────────┘  └───────────┘  └───────────┘
   应用指标      主机指标       容器指标
```

### 监控组件

| 组件 | 端口 | 作用 | 资源限制 |
|------|------|------|----------|
| **Prometheus** | 9091 | 时序数据存储与查询 | 512MB / 1CPU |
| **Node Exporter** | 9100 | 主机CPU/内存/磁盘/网络指标 | 128MB / 0.5CPU |
| **cAdvisor** | 8080 | 容器资源使用指标 | 256MB / 0.5CPU |
| **Grafana** | 3001 | 可视化仪表盘 | 256MB / 1CPU |

### 启动监控栈

```bash
# 启动监控服务（与DevDash主服务一起运行）
docker compose -f docker-compose.yml -f docker-compose.monitoring.yml up -d

# 查看状态
docker compose -f docker-compose.monitoring.yml ps

# 查看日志
docker compose -f docker-compose.monitoring.yml logs -f prometheus
```

**访问地址**：
- Prometheus: http://localhost:9091
- Grafana: http://localhost:3001 (admin / 自定义密码)
- cAdvisor: http://localhost:8080
- Node Exporter: http://localhost:9100

### DevDash 自定义指标

DevDash通过 `/api/metrics` 端点暴露以下Prometheus指标：

| 指标 | 类型 | 说明 |
|------|------|------|
| `devdash_http_requests_total` | Counter | HTTP请求总数（按method/path/status） |
| `devdash_http_request_duration_seconds` | Histogram | HTTP请求延迟分布 |
| `devdash_collect_duration_seconds` | Histogram | 系统指标采集耗时 |
| `devdash_cronjob_executions_total` | Counter | Cron任务执行次数（按job_id/status） |
| `devdash_terminal_sessions_active` | Gauge | 活跃终端会话数 |
| `devdash_alerts_active` | Gauge | 活跃告警数 |

### 告警规则

告警规则定义在 [`docker/alert-rules.yml`](docker/alert-rules.yml)：

| 告警名称 | 触发条件 | 持续时间 | 严重级别 |
|----------|----------|----------|----------|
| DevDashServiceDown | 服务不可达 | 1m | critical |
| HighCPUUsage | CPU > 80% | 5m | warning |
| HighMemoryUsage | 内存 > 85% | 5m | warning |
| LowDiskSpace | 磁盘 > 90% | 5m | critical |
| ContainerOOMRisk | 容器内存 > 90% | 2m | warning |
| HighHTTPErrorRate | 5xx错误率 > 10% | 2m | warning |
| CollectTimeout | 采集耗时 > 30s | 2m | warning |

### Grafana 仪表盘

预置仪表盘自动加载（[`docker/grafana/dashboards/devdash-overview.json`](docker/grafana/dashboards/devdash-overview.json)）：

- **CPU使用率**（仪表盘）
- **内存使用率**（仪表盘）
- **磁盘使用率**（柱状图）
- **网络流量**（时序图，RX/TX）
- **HTTP请求速率**（时序图，按方法）
- **HTTP请求延迟**（p50/p95分位）
- **HTTP状态码分布**（堆叠图）
- **活跃终端会话数**（统计）
- **活跃告警数**（统计）
- **Cron任务执行速率**（时序图）

### 自定义告警通知（可选）

如需邮件/Webhook通知，添加Alertmanager：

```yaml
# docker-compose.monitoring.yml 追加
  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./docker/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
    networks:
      - devdash-net
```

```yaml
# docker/prometheus.yml 取消注释 alertmanager targets
alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']
```

---

## �️ 部署工具

### 一键部署脚本

支持通过 SSH 一键部署到远程服务器，包含环境检查、自动备份、文件传输、服务启停和回滚功能。

```bash
# Linux/macOS
DEPLOY_HOST=your-server-ip bash deploy.sh deploy

# Windows PowerShell
.\deploy.ps1 -DeployHost your-server-ip -Command deploy
```

**部署命令：**

| 命令 | 说明 |
|------|------|
| `deploy` | 完整部署（构建 → 备份 → 传输 → 启动 → 验证） |
| `check` | 检查服务器环境（Docker、磁盘、端口等） |
| `rollback` | 回滚到上一个部署版本 |
| `status` | 查看远程服务状态 |

**环境切换：**

```bash
# 部署到生产环境
DEPLOY_HOST=prod.example.com DEPLOY_ENV=production bash deploy.sh deploy

# 部署到测试环境
DEPLOY_HOST=staging.example.com DEPLOY_ENV=staging bash deploy.sh deploy
```

### Kubernetes 部署

项目提供完整的 Kubernetes 部署配置，位于 `k8s/` 目录。

**配置文件说明：**

| 文件 | 说明 |
|------|------|
| `k8s/deployment.yml` | 完整 K8s 部署清单（Namespace + Secret + ConfigMap + Deployment + PVC + Service + Ingress） |
| `k8s/deploy.sh` | K8s 部署管理脚本 |

**一键部署：**

```bash
# 1. 修改 Secret 中的密钥
# 编辑 k8s/deployment.yml，将 JWT_SECRET 和 ENCRYPTION_KEY 替换为随机值
# 生成方法: openssl rand -hex 32

# 2. 部署到集群
kubectl apply -f k8s/

# 3. 等待 Pod 就绪
kubectl -n devdash rollout status deployment/devdash

# 4. 端口转发测试
kubectl -n devdash port-forward svc/devdash 9090:9090
# 访问 http://localhost:9090
```

**使用部署脚本：**

```bash
# 部署
./k8s/deploy.sh deploy

# 查看状态
./k8s/deploy.sh status

# 更新镜像
./k8s/deploy.sh upgrade ghcr.io/gxfdev/devdash:v1.8.1

# 查看日志
./k8s/deploy.sh logs

# 扩缩容
./k8s/deploy.sh scale 2

# 删除
./k8s/deploy.sh destroy
```

**K8s 部署架构：**

```
┌──────────────────────────────────────────────────┐
│  Ingress (nginx)                                 │
│  devdash.example.com → svc/devdash:9090          │
└──────────────────┬───────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────┐
│  Service (ClusterIP)                             │
│  devdash:9090                                    │
└──────────────────┬───────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────┐
│  Deployment (1 replica)                          │
│  ┌─────────────────────────────────────────────┐ │
│  │ Pod                                         │ │
│  │  Container: ghcr.io/gxfdev/devdash:latest   │ │
│  │  Limits: 1 CPU / 512Mi RAM                  │ │
│  │  Mounts: /data (PVC), /host/proc, /host/sys │ │
│  └─────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘
```

> **注意**：K8s 部署需要将宿主机的 `/proc`、`/sys`、`/dev`、`/etc` 挂载到容器中才能采集宿主机指标。Pod 仅调度到 Linux 节点。

---

## 🔬 数据采集实现原理

DevDash 使用 [gopsutil](https://github.com/shirou/gopsutil) 库实现跨平台的系统指标采集，核心代码位于 `server/internal/collector/collector.go`。

### 采集架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Collector.Collect()                       │
│                                                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ collectCPU│ │collectMem│ │collectDisk│ │ collectNetwork│  │
│  │  (并发)   │ │  (并发)   │ │  (并发)   │ │   (并发)      │  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────┬────────┘  │
│       │            │            │               │            │
│  ┌────▼────────────▼────────────▼───────────────▼────────┐  │
│  │              sync.WaitGroup (6 goroutines)             │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                   │
│  ┌───────────────────────▼───────────────────────────────┐  │
│  │  collectProcesses / collectGPU / collectDiskIO /       │  │
│  │  collectTCPConns / collectSensors (顺序执行)           │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          ▼                                   │
│                    model.Snapshot                            │
└─────────────────────────────────────────────────────────────┘
```

### CPU 采集原理

**技术**：`gopsutil/v4/cpu` 包

**实现流程**：

1. **核心数**：调用 `runtime.NumCPU()` 获取逻辑 CPU 核心数
2. **总使用率**：调用 `cpu.Percent(0, false)` 获取整体 CPU 使用百分比
   - 参数 `0` 表示不等待，直接返回自上次调用以来的 CPU 使用率
   - 参数 `false` 表示返回总体使用率而非每核使用率
3. **每核使用率**：调用 `cpu.Percent(0, true)` 获取每个逻辑核心的使用率
4. **防抖处理**：通过 `prevCPUTime` 记录上次采集时间，如果两次采集间隔超过 30 秒或为首次采集，使用缓存值避免异常波动

**底层原理**（Linux）：
- 读取 `/proc/stat` 文件获取 CPU 时间片（user/nice/system/idle/iowait/irq/softirq/steal）
- 计算公式：`usage = (total - idle) / total * 100`
- Docker 环境下通过 `HOST_PROC=/host/proc` 读取宿主机的 `/proc`

**底层原理**（Windows）：
- 调用 Windows API `GetSystemTimes()` 获取 idle/kernel/user 时间
- 同样通过差值计算使用率

### 内存采集原理

**技术**：`gopsutil/v4/mem` 包

**实现流程**：

1. **物理内存**：调用 `mem.VirtualMemory()` 获取
   - `Total`：总物理内存（字节）
   - `Used`：已用内存（字节）
   - `Available`：可用内存（字节）
   - `UsedPercent`：使用百分比
2. **交换分区**：调用 `mem.SwapMemory()` 获取
   - `Total`：交换分区总量
   - `Used`：交换分区已用
3. **单位转换**：字节 → GB（`bytes / 1024^3`），保留两位小数

**底层原理**（Linux）：
- 读取 `/proc/meminfo` 文件
- 关键字段：`MemTotal`、`MemFree`、`MemAvailable`、`Buffers`、`Cached`、`SwapTotal`、`SwapFree`
- `Used = Total - Available`（更准确的方式，因为 Available 包含了可回收的缓存）

**底层原理**（Windows）：
- 调用 `GlobalMemoryStatusEx()` API 获取 `MEMORYSTATUSEX` 结构体

### 磁盘采集原理

**技术**：`gopsutil/v4/disk` 包

**实现流程**：

1. **分区列表**：调用 `disk.Partitions(false)` 获取所有挂载分区
   - 参数 `false` 表示不显示虚拟/伪文件系统
2. **分区使用率**：对每个分区调用 `disk.Usage(mountpoint)` 获取
   - `Total`：分区总容量
   - `Used`：已用空间
   - `Free`：可用空间
   - `UsedPercent`：使用百分比
3. **主分区选择**：自动选择容量最大的分区作为主显示指标
4. **Windows 兼容**：如果 `Partitions()` 失败，回退到直接查询 `C:` 盘

**磁盘 I/O 速率采集**：

1. **累计字节数**：调用 `disk.IOCounters()` 获取所有磁盘设备的累计读写字节数
2. **速率计算**：通过两次采集的差值除以时间间隔计算实时速率
   ```
   ReadRateMB = (currentReadBytes - lastReadBytes) / elapsedSeconds / 1024 / 1024
   WriteRateMB = (currentWriteBytes - lastWriteBytes) / elapsedSeconds / 1024 / 1024
   ```
3. **线程安全**：使用 `sync.RWMutex` 保护上次采集值，防止并发读写

**底层原理**（Linux）：
- 分区信息：读取 `/proc/mounts` 和 `/etc/mtab`
- 使用率：调用 `statvfs()` 系统调用获取文件系统统计
- I/O 统计：读取 `/proc/diskstats` 或 `/sys/block/*/stat`

**底层原理**（Windows）：
- 使用率：调用 `GetDiskFreeSpaceExW()` API
- I/O 统计：通过 `Win32_PerfFormattedData_PerfDisk_LogicalDisk` WMI 查询

### 网络采集原理

**技术**：`gopsutil/v4/net` 包

**实现流程**：

1. **累计流量**：调用 `net.IOCounters(false)` 获取网络接口总流量
   - `BytesRecv`：总接收字节数
   - `BytesSent`：总发送字节数
2. **速率计算**：与磁盘 I/O 类似，通过差值/时间间隔计算实时速率
   ```
   RecvRateMB = (currentRecv - lastRecv) / elapsed / 1024 / 1024
   SentRateMB = (currentSent - lastSent) / elapsed / 1024 / 1024
   ```

### Docker 环境下的采集

在容器中运行时，默认只能看到容器自身的资源信息。DevDash 通过挂载宿主机目录来采集宿主机数据：

| 环境变量 | 挂载目录 | 用途 |
|---------|---------|------|
| `HOST_PROC=/host/proc` | `-v /proc:/host/proc:ro` | CPU、内存、进程、TCP 连接 |
| `HOST_SYS=/host/sys` | `-v /sys:/host/sys:ro` | 磁盘 I/O、传感器 |
| `HOST_DEV=/host/dev` | `-v /dev:/host/dev:ro` | 磁盘设备信息 |
| `HOST_ETC=/host/etc` | `-v /etc:/host/etc:ro` | 主机名、OS 信息 |

gopsutil 在 `init()` 阶段检测这些环境变量，自动将读取路径从 `/proc` 重定向到 `/host/proc`，从而在容器内获取宿主机指标。

### 采集调度

- **默认间隔**：5 秒（可通过 `INTERVAL` 环境变量或设置页面调整，范围 3-300 秒）
- **并发采集**：CPU、内存、磁盘、网络、负载、主机信息 6 个指标并发采集（`sync.WaitGroup`）
- **顺序采集**：进程列表、GPU、传感器、磁盘 I/O、TCP 连接在并发采集后顺序执行
- **超时控制**：整体采集超时 30 秒（`context.WithTimeout`）
- **容错机制**：每个采集函数都有 `recover()` 防止 panic 导致整体崩溃

---

### Makefile 快捷命令

| 命令 | 说明 |
|------|------|
| `make dev-start` | 一键启动开发环境 |
| `make dev-stop` | 停止开发环境 |
| `make dev-status` | 查看服务状态 |
| `make dev-logs` | 查看服务日志 |
| `make deploy` | 部署到服务器 |
| `make deploy-check` | 检查服务器环境 |
| `make deploy-rollback` | 回滚部署 |
| `make lint` | 代码检查（前后端） |
| `make security-scan` | 安全漏洞扫描 |
| `make test` | 运行所有测试 |
| `make docker-up` | 启动 Docker 容器 |
| `make docker-dev-up` | 启动开发环境容器 |

---

## 📖 API 文档

完整的 RESTful API 文档位于 `docs/openapi.yaml`，可使用 [Swagger Editor](https://editor.swagger.io/) 在线查看。

### API 规范

- **基础路径**: `/api/v1`
- **认证方式**: Bearer Token (JWT)
- **响应格式**:

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 用户登录，返回 JWT |
| POST | `/api/auth/change-password` | 修改密码 |

### 监控数据

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/health` | 健康检查（无需认证） |
| GET | `/api/v1/snapshot` | 获取系统快照 |
| GET | `/api/v1/latest` | 最新监控数据 |
| GET | `/api/v1/history` | 历史数据（支持 `limit` 参数） |
| GET | `/api/v1/trend/compare` | 趋势对比数据（支持 `period` 和 `node_id`） |

### 节点管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/nodes` | 获取所有节点 |
| POST | `/api/v1/nodes` | 添加节点 |
| DELETE | `/api/v1/nodes/:id` | 删除节点 |

### Docker 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/docker/ping` | 检查 Docker 状态 |
| GET | `/api/v1/docker/containers` | 容器列表 |
| POST | `/api/v1/docker/containers/:id/start` | 启动容器 |
| POST | `/api/v1/docker/containers/:id/stop` | 停止容器 |
| GET | `/api/v1/docker/containers/:id/logs` | 容器日志 |

### WebSocket

| 端点 | 说明 |
|------|------|
| `/ws/terminal/:nodeId?token=<jwt>` | Web 终端连接 |

> 所有 API（除 `/health` 和 `/auth/login` 外）均需在请求头中携带 `Authorization: Bearer <token>`

---

## 🔒 安全特性

> **生产环境部署必读**: [完整安全指南](SECURITY.md)

### 已内置的安全措施

| 安全层 | 技术 | 说明 |
|--------|------|------|
| 🔐 **传输加密** | TLS 1.2/1.3 + HSTS | HTTPS 强制跳转，安全密码套件 |
| 🛂 **身份认证** | JWT (HS256) + bcrypt | 7天有效期 + 密码哈希 |
| 🚫 **防暴力破解** | 登录速率限制 | 5次失败锁定15分钟 |
| 🛡️ **CSRF 防护** | Token 验证 | 防止跨站请求伪造 |
| 🔒 **CORS 白名单** | 域名限制 | 仅允许指定域名访问 |
| 📋 **安全头** | X-Frame-Options / CSP 等 | 防止 XSS / 点击劫持 |
| 🚦 **API 限流** | 600 次/分钟 | 防止 API 滥用 |
| 📍 **IP 白名单** | 可选配置 | 限制管理面板访问 IP |
| 🐳 **容器安全** | rootless + 只读文件系统 | 最小权限原则 |

### TLS 负载均衡架构

```
客户端 ──HTTPS──▶ Nginx (TLS 终止)
                    ├── /           → 静态文件 (try_files)
                    ├── /api/       → upstream (主 + 备故障转移)
                    └── /ws         → WebSocket 代理
```

- 主服务器故障自动切换到备用服务器
- `max_fails=3 fail_timeout=5s` 健康检查
- `proxy_next_upstream` 自动重试

### ⚠️ 生产环境必做清单

1. ✅ **修改默认密码** - 首次登录立即修改 admin/admin123
2. ✅ **设置强 JWT_SECRET** - `openssl rand -hex 32` 生成随机密钥
3. ✅ **启用 HTTPS** - 使用 Let's Encrypt 或自有证书
4. ✅ **配置防火墙** - 仅开放 80/443 端口
5. ✅ **限制 CORS_ORIGINS** - 不要使用 `*`
6. ✅ **定期备份** - 每日备份数据库文件
7. ✅ **监控日志** - 检查异常登录尝试

---

## ❓ 常见问题

### Q: 一键 Docker 部署后如何更新？
A:
```bash
docker pull ghcr.io/gxfdev/devdash:latest
docker compose -f docker-compose.ghcr.yml up -d
```
数据保存在 Docker Volume 中，更新不会丢失。

### Q: 控制台出现 429 错误？
A: API 请求频率超过限制（600次/分钟）。请检查前端轮询间隔是否过短，或调整后端 `handler.go` 中的速率限制值。

### Q: 控制台出现网络错误？
A: 请确保：
- 后端服务正常运行（检查 `http://localhost:9090/api/v1/health`）
- 前端 API 地址正确（`.env.development` 或环境变量）
- 浏览器控制台无网络错误

### Q: 中文显示乱码？
A: Docker 部署会自动设置 `LANG=C.UTF-8`、`TZ=Asia/Shanghai`。如仍有问题，请确认终端编码为 UTF-8。

### Q: Web 终端连接失败？
A: 检查：
1. WebSocket 连接地址是否正确（应为 `ws://host:port/ws/terminal/self`）
2. JWT Token 是否有效（过期需重新登录）
3. 后端终端服务是否正常启动
4. Windows 上需要 CGO 编译（ConPTY 支持）

### Q: Docker 管理页面显示不可用？
A: 确保已挂载 Docker Socket：`-v /var/run/docker.sock:/var/run/docker.sock:ro`，且宿主机 Docker 服务正在运行。

### Q: 如何添加新节点？
A: 在「主机管理」页面点击「添加节点」，输入 IP 和端口即可。Agent 节点需要先部署 Agent 服务。

### Q: 告警不触发？
A: 检查：
1. 告警规则是否启用
2. 采集间隔是否正常
3. 节点是否在线
4. 通知渠道是否配置正确

### Q: 趋势对比没有数据？
A: 趋势对比需要至少 2 个采集周期的数据。请确保系统运行足够长时间，并选择合适的时间范围。

### Q: 如何切换到 PostgreSQL？
A: 设置环境变量 `DB_TYPE=postgres`，并配置 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`。使用 `docker-compose.yml` 会自动配置 PostgreSQL（需启用 `postgres` profile）。

### Q: 如何备份数据？
A:
```bash
# SQLite
docker cp devdash:/data/devdash.db ./backup/

# PostgreSQL
docker exec devdash-postgres pg_dump -U devdash devdash > backup.sql
```

### Q: Windows 二进制无法运行？
A: Windows 版本需要 CGO 编译（ConPTY 终端支持），需要安装 [MinGW-w64](https://www.mingw-w64.org/) 或 Visual Studio Build Tools。推荐使用 Docker 部署方式。

---

## 🤝 参与贡献

欢迎贡献代码！请遵循以下步骤：

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add some amazing feature'`
4. 推送分支：`git push origin feature/amazing-feature`
5. 提交 Pull Request

### 开发规范

- 后端代码遵循 Go 标准格式（`gofmt`）
- 前端代码遵循 ESLint + TypeScript 严格模式
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)
- 所有 API 必须有错误处理和输入验证
- 新功能需要添加对应的测试

---

## 📝 更新日志

### v1.8.1 (2026-06-23) - 监控曲线精度修复 + 技术文档完善

**关键修复**：
- ✅ **CPU采集精度提升** - per-core CPU使用率采集增加200ms采样延迟窗口，避免间隔过短返回0或不准确值
- ✅ **网络速率曲线全0修复** - 后端增加`lastRecvRate`/`lastSentRate`速率缓存，采集间隔<0.5s时自动复用上次速率；前端速率为0时从累计字节数推算
- ✅ **磁盘IO速率曲线全0修复** - 同网络速率修复方案，后端增加`lastDiskReadRate`/`lastDiskWriteRate`缓存；前端回退到累计读写量差分计算
- ✅ **计数器重置负值保护** - 网络和磁盘计数器在系统重启后可能重置，增加负值检测将差值置0防止曲线异常尖峰
- ✅ **TrendsView速率回退不生效** - 速率判断条件从`!== null`改为`!== null && > 0`，确保零值触发累计推算逻辑
- ✅ **技术文档生成** - 新增论文格式技术文档（封面/中英文摘要/目录/EER图/流程图/时序图/参考文献）
- ✅ **图表目录补充** - 新增diagrams/目录包含EER图、认证/采集/终端流程图、告警/采集/容器/登录时序图

**变更文件**：
- `server/internal/collector/collector.go` - CPU/网络/磁盘采集逻辑优化
- `web/src/views/DashboardView.vue` - 历史数据速率回退计算
- `web/src/views/TrendsView.vue` - 速率系列回退条件修复

### v1.8.0 (2026-06-19) - Linux容器适配修复

**关键修复**：
- ✅ **文件管理Linux适配** - 默认使用Linux路径`/`，移除浏览器UA依赖，仅在后端返回`server_os=windows`时切换Windows路径
- ✅ **脚本管理Linux适配** - 默认解释器改为`/bin/bash`，移除浏览器UA判断
- ✅ **终端nsenter优化** - 容器模式下通过nsenter进入主机命名空间执行shell，默认fallback到`/bin/sh`
- ✅ **磁盘监控修复** - 容器模式下将主机挂载点映射到容器内路径，正确显示主机磁盘
- ✅ **CPU监控修复** - 使用500ms延迟采集避免首次返回不准确值
- ✅ **Dockerfile优化** - 移除非root用户限制（nsenter/crontab/docker.sock需要root），添加bash和coreutils依赖
- ✅ **路径安全策略** - 容器模式下放宽系统目录限制，允许访问/etc、/root等主机目录
- ✅ **Docker构建加速** - 添加BuildKit缓存挂载（npm和Go模块），移除不必要的python3/make/g++

### v1.7.0 (2026-06-19) - 运维脚本工具集

**重大新功能**：
- ✅ **11个运维脚本** - 新增4类共11个实用脚本，覆盖运维全流程
- ✅ **运维部署脚本** - `service.sh`（服务管理，自动检测systemd/docker）、`restore.sh`（数据恢复，支持dry-run预演）
- ✅ **Docker管理脚本** - `docker-clean.sh`（资源清理4种模式）、`docker-health.sh`（容器健康检查+端口+HTTP端点）、`docker-logs.sh`（日志查看）
- ✅ **开发工具脚本** - `test.sh`（单元/集成/覆盖率/lint测试）、`migrate.sh`（数据库迁移管理）
- ✅ **系统运维脚本** - `log-clean.sh`（日志清理）、`monitor.sh`（实时性能监控）、`ssl-renew.sh`（SSL证书续期）、`firewall.sh`（防火墙配置，支持ufw/firewalld/iptables）
- ✅ **README部署文档更新** - 更新版本引用至v1.7.0，新增带监控面板的部署说明

### v1.6.0 (2026-06-19) - 容器监控方案

**重大新功能**：
- ✅ **Prometheus指标端点** - 新增`/api/metrics`端点，暴露6个自定义指标（HTTP请求/延迟/Cron任务/终端会话/告警/采集耗时）
- ✅ **metrics指标收集包** - 新增`server/internal/metrics`包，使用prometheus/client_golang自动注册Counter/Histogram/Gauge
- ✅ **告警规则配置** - 新增7条告警规则（服务宕机/CPU/内存/磁盘/容器OOM/HTTP错误率/采集超时）
- ✅ **Grafana仪表盘** - 预置10个监控面板（CPU/内存/磁盘/网络/HTTP/Cron/终端/告警），自动provisioning加载
- ✅ **GHCR推送脚本** - 新增`docker/build-push-ghcr.sh`脚本，支持认证/标签/推送全流程
- ✅ **Dockerfile优化** - 添加GOPROXY镜像源、构建后清理node_modules、添加OCI标准LABEL
- ✅ **.dockerignore** - 排除.git/测试/文档/构建产物，减小构建上下文
- ✅ **容器化部署文档** - README新增容器化概念/优势/技术对比/构建优化/GHCR推送/监控方案完整文档

### v1.5.0 (2026-06-19) - Docker容器主机访问支持

**重大新功能**：
- ✅ **HOST_ROOT主机路径映射** - 新增`hostpath`包，容器内通过`HOST_ROOT=/host`环境变量映射主机文件系统
- ✅ **文件管理操作主机** - filemgr模块所有操作（ListDir/ReadFile/WriteFile/Delete/Mkdir/Rename/Upload/Chmod）支持映射到主机路径
- ✅ **脚本执行操作主机** - executeScript将脚本复制到主机/tmp后通过`nsenter`执行，确保操作主机环境
- ✅ **命令执行操作主机** - executeCommand通过`nsenter -m -u -i -n -p -t 1`进入主机命名空间执行
- ✅ **Crontab注册操作主机** - Linux的crontab注册/注销通过nsenter在主机上执行
- ✅ **终端访问主机** - 容器内终端通过nsenter连接主机shell
- ✅ **Docker配置更新** - docker-compose挂载`/:/host:rw`，添加SYS_ADMIN/SYS_CHROOT capability，安装nsenter工具
- ✅ **前端Cron更新修复** - 修复前端使用PATCH而非PUT导致cron任务更新失败的问题

### v1.4.1 (2026-06-19) - Windows计划任务修复

**重大修复**：
- ✅ **Windows计划任务触发器重写** - 完全重写`parseCronToTrigger`为`buildWindowsTrigger`，支持所有cron表达式模式：
  - `*/N * * * *` → 每N分钟重复（使用`-RepetitionInterval`）
  - `M * * * *` → 每小时第M分钟
  - `M H * * *` → 每天H:M（使用`-Daily -At`）
  - `M H * * D` → 每周D的H:M（使用`-Weekly -DaysOfWeek`）
  - `M H D * *` → 每月D号（Windows不支持月度，用Weekly近似）
  - `*/N */M * * *` → 每M小时N分钟间隔
- ✅ **PowerShell命令构建修复** - 使用变量赋值方式构建触发器，避免内联注释导致`Register-ScheduledTask`被忽略
- ✅ **runCronJob类型断言修复** - 修复`runCronJob`中`id`字段类型断言失败导致任务无法运行的问题（`int` vs `float64`）
- ✅ **错误信息增强** - Windows计划任务注册失败时返回PowerShell stderr详细信息

### v1.4.0 (2026-06-19) - 企业级安全加固与跨平台兼容性修复

**重大修复**：
- ✅ **Cron任务管理重构** - API层现在正确调用cronjob模块进行验证和OS调度器注册，修复了之前绕过所有安全验证的严重问题
- ✅ **脚本安全检查修复** - `checkScriptSecurity` 从字符串匹配改为正则表达式匹配，正确检测 `curl|sh`、`wget|bash` 等危险管道命令
- ✅ **脚本执行扩展名修复** - 临时脚本文件根据解释器自动添加正确扩展名（.ps1/.py/.pl/.rb/.js/.bat/.sh），解决Windows下PowerShell脚本无法执行的问题
- ✅ **命令执行安全验证** - `executeCommand` 新增命令安全验证，防止执行危险命令（rm -rf /、dd、mkfs等）
- ✅ **Cron任务enabled状态修复** - enabled=false时不再尝试注册OS调度器，避免创建失败
- ✅ **文件上传安全策略调整** - 允许上传.sh和.ps1脚本文件，仅阻止Windows可执行文件（.exe/.bat/.cmd）和Web脚本（.php/.jsp/.asp）
- ✅ **CPU采集器逻辑简化** - 移除不必要的过期值回退逻辑，直接使用当前采集值
- ✅ **Cron命令验证放宽** - 允许wget/curl/nc等常用工具，支持管道符、重定向、引号等shell字符
- ✅ **飞书消息格式修复** - 修复飞书卡片消息template字段格式错误

**跨平台兼容性验证**：
- ✅ Windows编译通过
- ✅ Linux交叉编译通过（GOOS=linux GOARCH=amd64）
- ✅ PowerShell脚本执行（-File参数）
- ✅ CMD脚本执行（/c参数）
- ✅ Bash/Shell脚本执行（直接路径）
- ✅ Cron任务crontab注册（Linux）
- ✅ Task Scheduler注册（Windows）
- ✅ PTY终端（Linux）+ ConPTY终端（Windows）
- ✅ 文件路径处理（filepath.Join跨平台）
- ✅ 文件权限（0755/0644 Linux模式）

**企业级标准改进**：
- 输入验证：所有API接口进行输入验证和类型检查
- 错误处理：统一的错误响应格式和HTTP状态码
- 安全审计：所有操作记录审计日志
- 权限控制：基于角色的访问控制（admin/user）
- 速率限制：API请求频率限制（600次/分钟）
- CSRF防护：所有写操作需要CSRF Token
- 安全头：CSP、X-Frame-Options、X-Content-Type-Options等

### v1.3.2 (2026-06-17) - Docker镜像发布与文档完善

**改进**：
- ✅ **GHCR镜像发布** - Docker镜像已推送至 `ghcr.io/gxfdev/devdash:latest`，支持 `linux/amd64` + `linux/arm64` 双架构
- ✅ **CI/CD流水线优化** - 修复 `go vet` 参数错位、`govulncheck` 误报过滤（jwt.ParseUnverified）
- ✅ **README徽章** - 新增GHCR镜像徽章，调整徽章排列
- ✅ **一键Docker部署** - 完善Linux/Windows/macOS一键部署命令，新增CentOS 7兼容部署指南

### v1.3.1 (2026-06-17) - 文档与CI/CD完善

**改进**：
- ✅ **技术报告文档** - 新增学术论文格式的技术报告（Word格式），包含系统架构、技术选型、实现细节、测试分析等完整章节
- ✅ **CI/CD验证** - GitHub Actions流水线全面验证通过（Lint → Test → Build → Docker Build Test → Push）
- ✅ **README更新** - 更新告警中心功能描述，完善多渠道通知说明
- ✅ **Docker镜像构建** - 推送代码后自动触发CI/CD，构建并推送Docker镜像至GHCR
- ✅ **CI流水线修复** - 修复`go vet`报错（fmt.Sprintf参数对齐）和`govulncheck`误报过滤（jwt.ParseUnverified已知误报）

**Docker镜像信息**：
- 镜像地址：`ghcr.io/gxfdev/devdash:latest`
- 支持架构：`linux/amd64` + `linux/arm64`
- CI构建验证：[CI #57](https://github.com/gxfdev/DevDash/actions/runs/27692003852) ✅ 全部通过（8/8 jobs）
- 快速部署：`docker rm -f devdash 2>/dev/null; docker rmi ghcr.io/gxfdev/devdash:latest 2>/dev/null; docker pull ghcr.io/gxfdev/devdash:latest && docker run -d --name devdash --pid=host --cap-add SYS_PTRACE --cap-add SYS_ADMIN --cap-add SYS_CHROOT -p 9090:9090 -v devdash-data:/data -v /:/host:rw -v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /dev:/host/dev:ro -v /etc/hostname:/etc/hostname:ro -v /var/run/docker.sock:/var/run/docker.sock:ro -e JWT_SECRET=$(openssl rand -hex 32) -e HOST_ROOT=/host -e HOST_PROC=/host/proc -e HOST_SYS=/host/sys -e HOST_DEV=/host/dev -e HOST_ETC=/host/etc -e TZ=Asia/Shanghai ghcr.io/gxfdev/devdash:latest`

### v1.3.0 (2026-06-17) - 告警通知增强

**重大新功能**：
- ✅ **钉钉机器人告警** - 支持钉钉群机器人推送告警消息，支持加签验证
- ✅ **飞书机器人告警** - 支持飞书群机器人推送告警消息，卡片格式
- ✅ **告警通知配置界面** - 前端新增"通知配置"Tab，可视化配置所有通知渠道
- ✅ **配置持久化** - 告警通知配置存储在数据库中，重启不丢失
- ✅ **测试告警功能** - 一键发送测试告警，验证通知渠道是否正常
- ✅ **自定义Webhook** - 支持推送告警到任意Webhook端点
- ✅ **邮件通知** - 支持SMTP邮件告警推送

**新增API**：
| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/alert-notify/config` | GET | 获取通知配置 |
| `/api/v1/alert-notify/config` | PUT | 更新通知配置 |
| `/api/v1/alert-notify/test` | POST | 发送测试告警 |

**支持的通知渠道**：
- 🔔 浏览器通知（系统默认）
- 📱 飞书机器人（Webhook）
- 📱 钉钉机器人（Webhook + 加签）
- 📧 邮件通知（SMTP）
- 🔗 自定义Webhook

### v1.2.1 (2026-06-16) - 图表数据修复

**重要修复**：
- ✅ **CPU/内存趋势图修复** - `getVal` 函数现在正确返回 `null` 而非 `0`，数据缺失时图表线条会断开，不再显示虚假的零值
- ✅ **网络流量计算修复** - 总流量计算改为使用首尾差值法（`last - first`），替代错误的累加算法
- ✅ **磁盘 I/O 速率显示优化** - 移除过度的零值过滤（`> 0`），低速率或零速率数据现在能正常显示
- ✅ **代码清理** - 移除未使用的变量和代码，提高可维护性

**技术细节**：
```typescript
// 修复前：缺失数据显示为0（错误）
function getVal(h, keys) { return v || 0 }

// 修复后：缺失数据返回null（正确）
function getVal(h, keys) {
  if (h?._gap) return null  // 断开图表线
  return null  // 缺失数据不显示
}

// 网络总量计算：使用差值法
const totalNet = (inV[last] - inV[0] + outV[last] - outV[0]) / GB
```

**影响范围**：
- 趋势分析页面的所有4个图表（CPU、内存、磁盘、网络）
- 统计卡片中的均值/峰值/低谷计算
- 数据导出和异常检测功能

---

## �📄 许可证

本项目基于 [MIT License](LICENSE) 开源。

```
MIT License

Copyright (c) 2024-2026 DevDash Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## ⚠️ 免责声明

本软件按 "原样" 提供，不作任何明示或暗示的保证。使用本软件所造成的任何直接或间接损失，开发者不承担任何责任。请在生产环境中充分测试后再部署使用。

---

<p align="center">
  Made with ❤️ by DevDash Contributors<br/>
  <sub>If you find this project helpful, please consider giving it a ⭐️</sub>
</p>
