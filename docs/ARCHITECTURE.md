# 架构设计

## 1. 设计目标

| 目标 | 约束 |
| --- | --- |
| 单机交付 | 不依赖 Kubernetes、etcd、CNI、CSI；只需要 Docker Engine |
| 与 d.run 对齐的模块边界 | 模型服务 / AI 应用 / 容器底座 / 可观测 / 运营 / 全局管理 |
| 一键部署 AI | 从目录点击到模型可调用，全程可观测、可中断、可重试 |
| 容器化但可运维 | 工作负载运行在容器内，可按需获得宿主机级权限 |
| 安全可审计 | RBAC、API Key、Token 计量、全量操作审计 |

非目标：多机调度、跨节点迁移、服务网格、租户级网络隔离。这些交给 Kubernetes 版本。

## 2. 运行时拓扑

```
Browser ──HTTP/JWT──▶ Gin API ──▶ Store(SQLite)
                          │
                          ├──▶ Orchestrator ──Docker API──▶ container
                          ├──▶ Gateway ──HTTP──▶ model container /v1
                          ├──▶ Collector ──gopsutil/nvidia-smi──▶ host
                          └──▶ Terminal ──PTY/nsenter/exec──▶ shell & containers
```

面板自身可以以容器方式运行（推荐），此时：

- `docker.sock` 提供引擎访问；
- `/:/host` 提供宿主机文件系统；
- `pid: host` + `nsenter` 让 Web 终端进入宿主机命名空间；
- `/proc`、`/sys`、`/dev` 提供指标与 GPU。

## 3. 数据模型（SQLite）

| 表 | 用途 |
| --- | --- |
| `users` | 账号、bcrypt 密码、角色、强制改密 |
| `api_keys` | 网关 Key（前缀索引 + SHA-256）、角色、配额 |
| `services` | 服务记录：spec JSON、状态、容器 ID、端点、健康 |
| `service_events` | 部署/操作事件流（拉取进度、错误、就绪） |
| `token_usage` | 每次调用的 token/耗时/状态/模型/Key |
| `host_metrics` | 主机指标时间序列（CPU/内存/磁盘/网络/GPU） |
| `audit_logs` | 用户操作审计 |
| `settings` | 平台设置键值 |

SQLite 使用 WAL、单写连接，适合单机面板的并发量。

## 4. 部署引擎状态机

```
pending ──▶ pulling ──▶ starting ──▶ installing ──▶ running
   │            │           │             │            │
   └────────────┴───────────┴─────────────┴──▶ error    │
                                          stopped ◀──────┘
```

关键实现（`internal/orchestrator`）：

1. **BuildSpec**：目录默认值 + 用户覆盖；校验名称；从已有服务与操作系统端口表分配唯一宿主机端口。
2. **Pull**：`POST /images/create` 流式解析，事件节流写入 `service_events`。
3. **Create/Start**：组装 `HostConfig`（CPU/内存/shm、端口绑定、命名卷、资源限制、日志轮转、GPU DeviceRequest）。
4. **模型安装**：`runtime=ollama` 时等待服务就绪并执行 `ollama pull <model>`，输出实时写事件。
5. **健康检查**：优先容器网络 IP + 容器端口访问 `/v1/models`；失败不阻塞部署，标记 health。
6. **Monitor**：每 10s 对账容器状态，容器被外部删除/退出时更新服务状态。
7. **生命周期**：start/stop/restart/upgrade/remove，带每服务互斥锁，避免并发操作。

端口分配：优先声明的 `host_port`；已被占用或已分配给其他服务时从 20000–29999 顺序探测空闲端口。

## 5. 模型资产流水线（modelhub）

```
控制台/API: POST /model-assets
  source= modelscope | huggingface | url | local | ollama
        │
        ├── modelscope/huggingface：python:3.12-slim 下载容器
        │     pip install (清华源) → modelscope download / huggingface snapshot_download
        │     输出目录 /models/<assetId>（Docker 卷 aidash-models）
        ├── url：alpine + wget，tar.gz 自动解压
        ├── local：alpine 只读挂载宿主机目录，扫描大小/GGUF
        └── ollama：不在本地存文件，部署时由 Ollama 从官方仓库拉取
        │
        ▼
扫描目录：du -sb / find / *.gguf → 记录 size、file_count、gguf、runtime
        │
        ▼
部署：DeployRequest.model_asset_id + runtime
  llama.cpp：挂载 aidash-models:/models，llama-server -m <gguf> --alias <name>
  vLLM：挂载后 --model /models/<id> --served-model-name <name>（GPU）
  Ollama：ollama/ollama + 受管流程 ollama pull <repo>
        │
        ▼
健康检查 /v1/models → 注入网关模型路由
```

要点：

- **真实文件**：下载发生在真实 Docker 容器中，产物落在 Docker 命名卷；推理容器挂载同一卷。
- **断点恢复**：面板重启后 `Hub.Recover()` 检查下载容器状态，完成扫描或标记失败。
- **离线**：模型可先通过本地目录/直链导入；镜像可预拉取，部署时跳过 pull。
- **隔离**：下载容器只有 2C/2G、仅挂载模型卷，不接触 docker.sock。
## 5. 网关

```
POST /v1/chat/completions
  │  API Key 校验（前缀查表 + SHA-256 比对 + 配额）
  ▼
resolveModel(model) ──▶ running service (kind=model) ──▶ inspect ──▶ http://containerIP:port
  │
  ├── 非流式：读取上游 JSON，记录 usage，原样返回
  └── 流式：SSE 透传，按输出字符估算 completion token，结束后落库
```

- 模型别名来自目录 / 部署面板的 `model_name`，未指定时回退服务名；
- 网关缓存容器地址 20–30s，避免每个请求都 inspect；
- Token 配额按 Key 近 30 天累计，超限返回 429；
- Copilot 复用同一个 `resolveModel`，但走工具调用循环（最多 4 轮）。

## 6. 运维模式（宿主机权限）

`HostAccess` → `engine.HostConfig` 映射：

| 字段 | Docker 参数 |
| --- | --- |
| `privileged` | `Privileged: true` |
| `host_pid` | `PidMode: "host"` |
| `mount_host_fs` | `Binds: "/:/host:rw"`、`/proc`、`/sys`、`/dev` |
| `mount_docker_socket` | `Binds: "/var/run/docker.sock"` |
| 额外能力 | `CapAdd: SYS_ADMIN, SYS_PTRACE, NET_ADMIN, SYS_CHROOT` |
| 安全配置 | `SecurityOpt: seccomp=unconfined, apparmor=unconfined` |
| 环境变量 | `HOST_ROOT=/host`、`AIDASH_HOST_ACCESS=true`、`DOCKER_HOST=...` |

容器仍保持独立文件系统与网络命名空间（除非显式 host 网络），因此删除容器即可回收权限；
宿主机上的实际影响面由挂载与 Capability 决定，审计日志记录授权动作。

## 7. 前端

- `AppShell` 提供左侧模块导航 + 顶部状态条 + 全局 Copilot 抽屉；
- 页面通过 `api/index.ts` 访问 REST，统一解包 `{data}` 与错误提示；
- `TrendChart` 封装 ECharts，自适应尺寸，统一暗色主题；
- `DeployDrawer` / `ServiceDrawer` / `CopilotDrawer` 是主要交互容器；
- `TerminalPane` 通过 WebSocket 连接宿主机或容器终端，xterm.js + FitAddon；
- 设计语言：深色玻璃面板、紫色到青色渐变、等宽字体展示地址与日志。

## 8. 扩展点

| 需求 | 做法 |
| --- | --- |
| 新增模型 | 在 `catalog/catalog.json` 的 `models` 添加条目（镜像、端口、runtime、model_ref） |
| 新增应用 | 同上，`apps` 添加条目；`default_host_access` 控制默认权限 |
| 接入其他推理引擎 | 在 orchestrator 增加 runtime 分支（等待就绪 / 拉模型） |
| 多机 | 当前设计为单机；多机需要引入节点代理与调度层，建议使用 Kubernetes 版本 |
| 外部 SSO | 在 `auth` 层扩展 OIDC，保留 JWT 签发逻辑 |
