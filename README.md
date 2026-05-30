<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat-square&logo=vue.js" alt="Vue" />
  <img src="https://img.shields.io/badge/TypeScript-5.x-3178C6?style=flat-square&logo=typescript" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker" alt="Docker" />
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
- [配置说明](#-配置说明)
- [功能模块](#-功能模块)
- [项目结构](#-项目结构)
- [安全特性](#-安全特性)
- [API 文档](#-api-文档)
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

### 方式一：Docker Compose 部署（推荐生产环境）

```bash
# 1. 克隆项目
git clone https://github.com/your-username/devdash.git
cd devdash

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，设置 JWT_SECRET、DB_PASSWORD 等

# 3. 启动服务（含 TLS 负载均衡）
docker-compose -f docker-compose.prod.yml up -d --build
```

访问：`https://your-domain.com`

默认账号：`admin` / `admin123`（首次登录请立即修改密码）

### 方式二：Docker 单实例部署

```bash
git clone https://github.com/your-username/devdash.git
cd devdash
docker-compose up -d --build
```

访问：`http://localhost:9090`

### 方式三：本地开发（一键脚本）

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

# 示例：仅启动前端
SKIP_BACKEND=1 bash dev.sh start

# 查看服务状态
bash dev.sh status

# 查看日志
bash dev.sh logs backend
bash dev.sh logs frontend

# 停止所有服务
bash dev.sh stop

# 重启所有服务
bash dev.sh restart
```

### 方式三（备选）：手动启动

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
wget https://github.com/your-username/devdash/releases/latest/download/devdash-linux-amd64.tar.gz
tar -xzf devdash-linux-amd64.tar.gz
chmod +x devdash
./devdash

# Windows
# 下载 devdash-windows-amd64.zip，解压后运行 devdash.exe
```

---

## 🔧 配置说明

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `9090` | 服务监听端口 |
| `JWT_SECRET` | `change-this-random-secret` | JWT 签名密钥（**生产环境必须修改**） |
| `DB_TYPE` | `sqlite` | 数据库类型：`sqlite` 或 `postgres` |
| `DB_PATH` | `devdash.db` | SQLite 数据库路径（仅 SQLite 模式） |
| `DB_HOST` | `localhost` | PostgreSQL 主机（仅 PostgreSQL 模式） |
| `DB_PORT` | `5432` | PostgreSQL 端口 |
| `DB_USER` | `devdash` | PostgreSQL 用户名 |
| `DB_PASSWORD` | - | PostgreSQL 密码（**必填**） |
| `DB_NAME` | `devdash` | PostgreSQL 数据库名 |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL 模式 |
| `INTERVAL` | `5` | 数据采集间隔（秒） |
| `TZ` | `Asia/Shanghai` | 时区 |
| `GIN_MODE` | `debug` | Gin 运行模式：`debug` 或 `release` |
| `CORS_ORIGINS` | `*` | CORS 允许的来源（**生产环境建议限制**） |
| `STATIC_DIR` | - | 前端静态文件目录 |
| `AGENT_TOKEN` | - | Agent 认证令牌 |

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
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /dev:/host/dev:ro
      - /etc/hostname:/etc/hostname:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - PORT=9090
      - JWT_SECRET=your-random-secret-here
      - TZ=Asia/Shanghai
      - LANG=C.UTF-8
      - GIN_MODE=release
      - HOST_PROC=/host/proc
      - HOST_SYS=/host/sys
      - HOST_DEV=/host/dev
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
│   │   │   ├── handler.go           # 核心 API（仪表盘、趋势、历史）
│   │   │   ├── monitor_handler.go   # 监控相关 API
│   │   │   ├── agent_handler.go     # Agent 通信 API
│   │   │   ├── docker_handler.go    # Docker 管理 API
│   │   │   └── container_monitor_handler.go  # 容器监控 API
│   │   ├── auth/                    # 认证与安全
│   │   │   ├── jwt.go               # JWT 令牌管理
│   │   │   ├── middleware.go        # 认证中间件
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
│   ├── Dockerfile.server            # 后端多阶段构建
│   ├── Dockerfile.agent             # Agent 构建
│   ├── nginx.conf                   # Nginx 开发配置
│   ├── nginx.ssl.conf               # Nginx TLS + 负载均衡配置
│   ├── init-db/01-init.sql          # PostgreSQL 初始化
│   └── build-multiarch.sh           # 多架构构建脚本
├── deploy/                          # 部署脚本
│   ├── deploy.sh                    # Linux 部署脚本
│   ├── deploy-windows.ps1           # Windows 部署脚本
│   ├── backup.sh                    # 数据备份脚本
│   └── devdash.service              # systemd 服务文件
├── scripts/                         # 工具脚本
│   ├── build.sh                     # 构建脚本
│   ├── install.sh                   # Linux 安装脚本
│   ├── install.ps1                  # Windows 安装脚本
│   ├── deploy-secure.sh             # 安全部署脚本
│   └── test-production.sh           # 生产测试脚本
├── .github/workflows/               # GitHub Actions
│   ├── ci.yml                       # CI 流水线
│   └── release.yml                  # 发布流水线
├── docker-compose.yml               # 开发环境编排
├── docker-compose.prod.yml          # 生产环境编排（含负载均衡）
├── docker-compose.monitoring.yml    # 监控栈编排
├── Makefile                         # 构建命令
├── SECURITY.md                      # 安全指南
└── LICENSE                          # MIT 许可证
```

## � 部署工具

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

**部署配置文件：**

| 文件 | 用途 |
|------|------|
| `.env.prod` | 生产环境配置模板 |
| `.env.example` | 本地开发环境配置模板 |

### Docker Compose 部署

```bash
# 生产环境（含 Nginx 反向代理）
docker compose up -d --build

# 开发环境（含热重载）
docker compose -f docker-compose.dev.yml up -d --build
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
| `make test-unit` | 仅运行单元测试 |
| `make test-integration` | 仅运行集成测试 |
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

### 主要端点

| 分类 | 端点 | 方法 | 说明 |
|------|------|------|------|
| 认证 | `/auth/login` | POST | 用户登录 |
| 认证 | `/auth/refresh` | POST | 刷新Token |
| 指标 | `/snapshot` | GET | 获取系统快照 |
| 指标 | `/latest` | GET | 获取最新指标 |
| 指标 | `/history` | GET | 获取历史指标 |
| 节点 | `/nodes` | GET | 节点列表 |
| 节点 | `/node/:id/metrics` | GET | 节点指标 |
| 文件 | `/node/:id/fs/list` | GET | 目录列表 |
| 文件 | `/node/:id/fs/upload` | POST | 上传文件 |
| 告警 | `/alerts` | GET | 告警列表 |
| 告警 | `/alert-rules` | POST | 创建告警规则 |
| 设置 | `/settings` | GET/PUT | 系统设置 |
| 备份 | `/backup` | POST | 创建备份 |

---

## �🔒 安全特性

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

## 📡 API 文档

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 用户登录，返回 JWT |
| POST | `/api/auth/change-password` | 修改密码 |

### 监控数据

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/health` | 健康检查 |
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
| GET | `/api/v1/docker/containers` | 容器列表 |
| POST | `/api/v1/docker/containers/:id/start` | 启动容器 |
| POST | `/api/v1/docker/containers/:id/stop` | 停止容器 |
| GET | `/api/v1/docker/containers/:id/logs` | 容器日志 |

> 所有 API（除登录外）均需在请求头中携带 `Authorization: Bearer <token>`

---

## ❓ 常见问题

### Q: 控制台出现 429 错误？
A: API 请求频率超过限制（600次/分钟）。请检查前端轮询间隔是否过短，或调整后端 `handler.go` 中的速率限制值。

### Q: 控制台出现网络错误？
A: 请确保：
- 后端服务正常运行（检查 `http://localhost:9090/api/v1/health`）
- 前端 API 地址正确（`.env.development` 或环境变量）
- 浏览器控制台无网络错误

### Q: 中文显示乱码？
A: Docker 部署会自动设置 `LANG=C.UTF-8`、`TZ=Asia/Shanghai`。如仍有问题，请确认终端编码为 UTF-8。

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
A: 设置环境变量 `DB_TYPE=postgres`，并配置 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`。使用 `docker-compose.prod.yml` 会自动配置 PostgreSQL。

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
