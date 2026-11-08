#!/usr/bin/env bash
# AIDash 一键安装脚本（Linux / macOS）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

USE_IMAGE=0
IMAGE_REF="${AIDASH_IMAGE:-ghcr.io/gxfdev/devdash:latest}"
for arg in "$@"; do
  case "$arg" in
    --image|--prebuilt) USE_IMAGE=1 ;;
    -h|--help)
      echo "用法: ./install.sh [--image]"
      echo "  --image  使用 GitHub Actions 发布的预构建镜像（默认 ghcr.io/gxfdev/devdash:latest）"
      exit 0 ;;
  esac
done

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

info()  { printf '\033[1;36m[aidash]\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m[aidash]\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m[aidash]\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "未找到 docker，请先安装 Docker Engine 20.10+"

# ---------- 部署前检查 ----------
mem_gb=$(awk '/MemTotal/{printf "%d", $2/1024/1024}' /proc/meminfo 2>/dev/null || echo 0)
if [ "${mem_gb:-0}" -lt 8 ]; then
  warn "内存约 ${mem_gb}GB，建议 8GB 以上；7B q4 模型建议 16GB"
fi
disk_gb=$(df -Pk /var/lib/docker 2>/dev/null | awk 'NR==2{printf "%d", $4/1024/1024}')
if [ -n "${disk_gb:-}" ] && [ "$disk_gb" -lt 30 ]; then
  warn "Docker 数据盘可用约 ${disk_gb}GB，建议 30GB 以上（模型 + 镜像）"
fi
if command -v ss >/dev/null 2>&1 && ss -ltn 2>/dev/null | grep -q ":${AIDASH_PORT:-9090} "; then
  warn "端口 ${AIDASH_PORT:-9090} 已被占用，请先在 .env 中修改 AIDASH_PORT"
fi
if ! docker info >/dev/null 2>&1; then
  fail "无法连接 Docker daemon（确认 docker 服务已启动，当前用户有权限）"
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  fail "未找到 docker compose 插件"
fi

if [ ! -f .env ]; then
  info "生成默认 .env（含随机 JWT_SECRET）"
  if command -v openssl >/dev/null 2>&1; then
    SECRET="$(openssl rand -hex 32)"
  else
    SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  fi
  cat > .env <<EOF
JWT_SECRET=${SECRET}
ADMIN_USER=admin
ADMIN_PASS=admin123
AIDASH_PORT=9090
TZ=Asia/Shanghai
PUBLIC_URL=
CORS_ORIGINS=
EOF
  chmod 600 .env
else
  info "使用已有 .env"
fi

COMPOSE_FILES=""
if [ "$USE_IMAGE" = "1" ]; then
  info "拉取预构建镜像 ${IMAGE_REF}（多架构，无需本地构建）"
  if ! docker pull "${IMAGE_REF}"; then
    fail "镜像拉取失败。若镜像是私有包，请先 docker login ghcr.io；也可以去掉 --image 使用本地构建"
  fi
  export AIDASH_IMAGE="${IMAGE_REF}"
  COMPOSE_FILES="-f docker-compose.ghcr.yml"
  UPGRADE_CMD="$COMPOSE $COMPOSE_FILES pull && $COMPOSE $COMPOSE_FILES up -d"
  info "启动 AIDash"
  $COMPOSE $COMPOSE_FILES up -d
else
  UPGRADE_CMD="$COMPOSE up -d --build"
  info "构建并启动 AIDash（首次构建约 2-5 分钟）"
  $COMPOSE up -d --build
fi

info "等待服务就绪…"
for i in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${AIDASH_PORT:-9090}/api/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if ! curl -fsS "http://127.0.0.1:${AIDASH_PORT:-9090}/api/health" >/dev/null 2>&1; then
  warn "健康检查未通过，查看日志：$COMPOSE logs -f aidash"
  exit 1
fi

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -z "${IP:-}" ] && IP="127.0.0.1"

cat <<EOF

  ✅ AIDash 已启动

     控制台:  http://${IP}:${AIDASH_PORT:-9090}
     默认账号: admin / admin123 （登录后请立即修改密码）

     常用命令:
       $COMPOSE $COMPOSE_FILES logs -f aidash     # 查看日志
       $COMPOSE $COMPOSE_FILES restart aidash     # 重启
       $COMPOSE $COMPOSE_FILES down               # 停止
       $UPGRADE_CMD                               # 升级（保留数据卷）

EOF
