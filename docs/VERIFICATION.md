# 验证报告

环境：Windows 11 + WSL2 Ubuntu 22.04 · Docker 29.9.3（overlay2） · Go 1.26.3 · Node 24 · 16 核 / 15GB RAM。

> 本轮已使用真实 Docker 与真实模型完成验证，不再只依赖 mock。以下同时保留 mock 引擎测试，
> 用于证明无 Docker 环境下 API、部署引擎与网关逻辑正确。

## 1. 真实模型部署验证（WSL2 + Docker 29.9.3）

| 项目 | 结果 |
| --- | --- |
| 模型下载 | 魔搭 `Qwen/Qwen2.5-0.5B-Instruct-GGUF`，文件 `qwen2.5-0.5b-instruct-q4_k_m.gguf`，491,400,032 字节 |
| 下载实现 | 独立下载容器（python:3.12-slim + modelscope CLI），挂载 Docker 卷 `aidash-models:/models`，日志实时回传 tqdm 进度 |
| 产物识别 | 下载完成后扫描目录，自动识别 GGUF 并写入 `gguf=/models/mc0c239c473/...q4_k_m.gguf` |
| 推理部署 | llama.cpp server 容器，4C/8G，端口 20000→8080，容器日志 `model loaded`、`listening on http://0.0.0.0:8080` |
| 真实调用 | `/v1/chat/completions` 返回模型实际输出；usage `prompt_tokens=42, completion_tokens=56, total_tokens=98` |
| 流式调用 | SSE 实时返回 `+` `1` `等于` `2` `。` 等 delta，来自 llama.cpp 的真实 token 流 |
| UI | 模型仓库、模型服务概览、模型体验、网关页均实机截图，Playwright 检查 0 console error |

真实回复节选：

> 容器是一种虚拟的运行环境，它包含了一组预先安装好的软件和硬件资源，以及一个操作系统和一组配置文件……
> 2+3的结果是5。

备注：

- WSL 环境 Docker Hub / ghcr 直连受限，llama.cpp 镜像通过 `ghcr.m.daocloud.io` 镜像拉取后重打标签；
  AIDash 检测到本地镜像会跳过 pull（离线部署同样适用）。
- 该环境无 NVIDIA GPU，vLLM/GPU 路径通过单元测试断言 DeviceRequest、镜像与显存参数，未做实机推理。

## 2. 后端单元 / 集成测试

```bash
cd server && go test ./...
```

| 包 | 结果 | 覆盖内容 |
| --- | --- | --- |
| `internal/engine` | ok | ping/version/info、镜像拉取、创建/启动/inspect、日志分帧解复用、stats 计算、exec hijack、网络、删除 |
| `internal/orchestrator` | ok | 一键部署 Ollama 模型、事件流、日志、命令执行、启停删、宿主机权限参数、端口唯一分配 |
| `internal/api` | ok | 未授权 401、登录、viewer 写操作 403、部署/停止/删除、审计、设置、网关 Key、容器/镜像/网络/卷 API、Copilot 降级 |
| `internal/gateway` | ok | Copilot 工具调用循环（host_overview 结果回填并完成回答）、未开启执行开关时命令工具被拒绝 |

同时验证：

- 部署 `qwen2.5-7b`：创建请求包含 4C/8G、11434/tcp 端口绑定、`aios.service_id` 标签，事件包含「拉取镜像」「模型下载」「服务已就绪」
- 部署 `ai-ops-agent`：断言 `Privileged=true`、`PidMode=host`、`/:/host:rw`、`/var/run/docker.sock`、`/proc` 只读挂载、`SYS_ADMIN/SYS_PTRACE`、`seccomp=unconfined`
- 网关：真实 HTTP 代理到假 OpenAI 上游，非流式 usage=18 落库；无效 Key 401；未知模型 404；配额超限 429
- 部署两个模型时宿主机端口分别为 20000 / 20001（唯一分配）

## 3. 前端类型检查与构建

```bash
cd web && npx vue-tsc --noEmit && npx vite build
```

结果：类型检查 0 错误；生产构建成功（vendor 拆分：vue / naive / charts）。

## 4. Playwright UI 验证

```bash
cd web && node scripts/shots.mjs   # 全页面截图
cd web && node scripts/e2e.mjs     # 关键流程
```

`shots.mjs`：登录、驾驶舱、模型广场、模型服务、AI 应用、容器、可观测、AI 网关、全局管理、
部署抽屉、服务详情、Copilot。结果：

- consoleErrors: 0
- pageErrors: 0
- failedRequests: 0

`e2e.mjs`：登录 → 模型广场打开部署抽屉 → 修改服务名 → 一键部署 → 跳到服务详情并等待「运行中」→
打开容器终端（xterm 挂载成功）→ 清理测试服务 → 打开 Copilot 提问并收到回复。结果：

```json
{
  "steps": [
    "login ok",
    "deploy drawer opened: e2e-coder-156956 下载模型",
    "deployed service reached running state",
    "container terminal mounted: true",
    "copilot replied",
    "cleaned up 0 e2e service(s)"
  ],
  "consoleErrors": [],
  "pageErrors": [],
  "failedRequests": [],
  "ok": true
}
```

## 5. 演示模式

```bash
cd server && go run ./cmd/demo
```

启动后数据：

- 5 个服务：qwen-7b / deepseek-r1（运行）、open-webui（运行）、ops-sandbox（运行，宿主机权限）、vector-db（停止）
- 24 小时主机指标、24 小时 Token 用量、2 个 API Key、3 个账号（admin / ops01 / guest）
- 网关真实代理到内置假模型，可验证流式 SSE、Token 计量与 Copilot 对话

## 6. 未覆盖与已知限制

- 未在真实 Docker Engine 上执行 GPU 部署（验证机无 NVIDIA GPU）；GPU 参数通过 `DeviceRequest` 生成并被测试断言
- Windows 宿主上的宿主机权限挂载退化为 npipe + 盘符，README 已注明生产建议 Linux
- mock 的交互式 shell 是回显模拟，真实容器终端由 Docker exec hijack 提供
- 浏览器终端在 mock 下 2 分钟自动断开（模拟 TTY 超时）
