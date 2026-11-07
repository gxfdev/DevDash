#!/usr/bin/env bash
# AIDash 一键安装脚本（Linux / macOS）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

info()  { printf '\033[1;36m[aidash]\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m[aidash]\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m[aidash]\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "未找到 docker，请先安装 Docker Engine 20.10+"

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

info "构建并启动 AIDash（首次构建约 2-5 分钟）"
$COMPOSE up -d --build

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
       $COMPOSE logs -f aidash     # 查看日志
       $COMPOSE restart aidash     # 重启
       $COMPOSE down               # 停止
       $COMPOSE up -d --build      # 升级（保留数据卷）

EOF
