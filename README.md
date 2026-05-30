<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat-square&logo=vue.js" alt="Vue" />
  <img src="https://img.shields.io/badge/TypeScript-5.x-3178C6?style=flat-square&logo=typescript" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker" alt="Docker" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License" />
  <img src="https://img.shields.io/github/actions/workflow/status/gxfdev/DevDash/ci.yml?style=flat-square&label=CI" alt="CI" />
  <img src="https://img.shields.io/github/v/tag/gxfdev/DevDash?style=flat-square&label=version" alt="Version" />
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
- [API 文档](#-api-文档)
- [安全特性](#-安全特性)
- [常见问题](#-常见问题)
- [参与贡献](#-参与贡献)
- [许可证](#-许可证)

---

## ✨ 核心特性

| 功能 | 描述 |
|------|------|
| 🖥️ **多主机实时监控** | CPU / 内存 / 磁盘 / 网络 / TCP 连接 / 磁盘 I/O 速率 / 系统负载 / 主机信息 |
| 📊 **趋势分析与对比** | 实时趋势图 + 历史数据回溯 + 前后期趋势对比 + 异常检测 (±2σ) + 周报导出 |
| 🏪 **应用商店** | 一键安装 Nginx / MySQL / Redis / Node.js 等 20+ 常用软件 |
| 📁 **文件管理** | 双栏浏览器 + 在线代码编辑 + 上传下载 + 权限管理 |
| 🔥 **防火墙** | ufw / firewalld / Windows Firewall 统一管理 |
| ⏰ **计划任务** | crontab / Task Scheduler 可视化配置 |
| 🗄️ **数据库管理** | MySQL / PostgreSQL Web 管理界面 |
| 🔴 **告警中心** | 自定义规则引擎 + 浏览器通知 + 飞书 Webhook |
| 🐳 **容器监控** | Docker 容器列表 / 状态 / 资源使用 / 日志流 / Compose 编排 |
| 💻 **Web 终端** | 浏览器内 SSH 终端，直接操作远程主机 |
| 🔐 **安全加固** | JWT 认证 + TLS 1.2/1.3 + 速率限制 + CORS 白名单 + 安全头 |
| ⚖️ **负载均衡** | Nginx 反向代理 + 主备故障转移 + WebSocket 支持 |

---

## 🖼️ 功能预览

### 仪表盘 - 实时监控
实时展示所有节点的 CPU、内存、磁盘、网络、TCP 连接等关键指标，5 秒自动刷新，异常高亮提示。

### 趋势分析 - 数据洞察
- **系统指标**：CPU / 内存 / 磁盘 I/O 速率 / 网络流量趋势图
- **趋势对比**：当前周期 vs 前期数据对比，自动计算变化趋势和幅度
- **异常检测**：基于统计学 ±2σ 方法自动检测异常数据点，标注正常范围

### 主机管理 - 多节点管控
添加/删除被管主机，查看 Agent 连接状态，支持远程执行命令和详细主机信息查看。

---

## 🏗️ 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端浏览器                          │
│              Vue 3 + Naive UI + ECharts + Pinia             │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS / WSS
┌──────────────────────────▼──────────────────────────────────┐
│                     Nginx 反向代理                           │
│         TLS 1.2/1.3 终止 + 负载均衡 + 静态文件              │
│                                                              │
│   upstream: devdash:9090 (主)  ←→  devdash2:9090 (备)       │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   Go 后端 (Gin Framework)                    │
│                                                              │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐   │
│  │ REST API │ │ WebSocket│ │ 数据采集  │ │ 告警引擎      │   │
│  │ Handler  │ │ Terminal │ │ Collector │ │ Alert Engine  │   │
│  └────┬────┘ └────┬─────┘ └────┬─────┘ └───────┬───────┘   │
│       │           │            │                │            │
│  ┌────▼───────────▼────────────▼────────────────▼───────┐   │
│  │              SQLite / PostgreSQL 存储                  │   │
│  └──────────────────────────┬───────────────────────────┘   │
└─────────────────────────────┼───────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ 本地节点  │   │ Agent 节点│   │ Agent 节点│
        │ (gopsutil)│   │ (远程主机)│   │ (远程主机)│
        └──────────┘   └──────────┘   └──────────┘
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

### 方式一：一键 Docker 部署（推荐）

无需克隆代码，直接拉取预构建镜像运行，**30 秒内启动**：

```bash
# 1. 生成随机密钥并启动（Linux / macOS）
export JWT_SECRET=$(openssl rand -hex 32)
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
  -e JWT_SECRET=$JWT_SECRET \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_DEV=/host/dev \
  -e HOST_ETC=/host/etc \
  -e GIN_MODE=release \
  -e TZ=Asia/Shanghai \
  ghcr.io/gxfdev/devdash:latest
```

```powershell
# Windows PowerShell
$jwtSecret = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
docker run -d `
  --name devdash `
  --restart unless-stopped `
  -p 9090:9090 `
  -v devdash-data:/data `
  -e "JWT_SECRET=$jwtSecret" `
  -e GIN_MODE=release `
  -e TZ=Asia/Shanghai `
  ghcr.io/gxfdev/devdash:latest
```

访问：**http://localhost:9090**  
默认账号：`admin` / `admin123`（首次登录请立即修改密码）

> 💡 **提示**：Windows 上不需要挂载 `/proc`、`/sys` 等目录，Docker 管理功能也不可用（无需挂载 `docker.sock`）。

#### 使用 Docker Compose 一键部署（更推荐）

```bash
# 下载编排文件
curl -O https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.ghcr.yml

# 生成密钥并启动
export JWT_SECRET=$(openssl rand -hex 32)
docker compose -f docker-compose.ghcr.yml up -d
```

```powershell
# Windows PowerShell
Invoke-WebRequest -Uri https://raw.githubusercontent.com/gxfdev/DevDash/main/docker-compose.ghcr.yml -OutFile docker-compose.ghcr.yml
$env:JWT_SECRET = -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
docker compose -f docker-compose.ghcr.yml up -d
```

#### 指定版本

```bash
# 使用特定版本
docker run -d ... ghcr.io/gxfdev/devdash:v1.0.0

# 查看所有可用版本
# https://github.com/gxfdev/DevDash/pkgs/container/devdash
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

### 3. 🖥️ 主机管理 Hosts
- 添加/删除被管主机
- 查看 Agent 连接状态
- 远程执行命令
- 详细主机信息（OS、内核、CPU、内存、磁盘、网络）

### 4. 🏪 应用商店 Store
内置常用软件目录，支持一键安装/卸载/启停服务。

支持的软件类别：
- **Web 服务**: Nginx, Apache, Caddy, Tomcat
- **数据库**: MySQL, PostgreSQL, MongoDB, Redis, SQLite
- **运行时**: Node.js, Python, JDK, Go, .NET
- **工具**: Git, Vim, htop, curl, wget

### 5. 🔴 告警中心 Alerts
- 内置默认告警规则（CPU > 80%, 内存 > 85%, 磁盘 > 90%）
- 支持自定义告警规则（指标、阈值、级别）
- 多渠道通知：浏览器弹窗、飞书 Webhook
- 告警历史记录与静默功能

### 6. 📁 文件管理 Files
- 双栏文件浏览器
- 在线编辑代码文件（语法高亮）
- 批量上传/下载
- 权限管理

### 7. 🔥 防火墙 Firewall
统一管理 Linux (ufw/firewalld) 和 Windows 防火墙规则。

### 8. ⏰ 计划任务 CronJobs
可视化创建和管理定时任务，支持 crontab 和 Windows Task Scheduler。

### 9. 💻 Web 终端 Terminal
基于 xterm.js 的 Web SSH 终端，直接在浏览器中操作远程主机。

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

## 🛠️ 部署工具

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

## 📄 许可证

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
