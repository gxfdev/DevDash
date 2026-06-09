#!/usr/bin/env bash
# DevDash 一键 Docker 部署脚本
# 用法: ./deploy.sh [命令]
# 命令: start | stop | restart | status | logs | update | backup | uninstall

set -euo pipefail

# ─── 配置 ──────────────────────────────────────────────
IMAGE="ghcr.io/gxfdev/devdash"
TAG="${VERSION:-latest}"
CONTAINER="devdash"
PORT="${PORT:-9090}"
DATA_DIR="${DATA_DIR:-devdash-data}"
TZ="${TZ:-Asia/Shanghai}"

# ─── 颜色 ──────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ─── 检查 Docker ───────────────────────────────────────
check_docker() {
  if ! command -v docker &>/dev/null; then
    error "Docker 未安装。请先安装 Docker: https://docs.docker.com/get-docker/"
  fi
  if ! docker info &>/dev/null; then
    error "Docker 未运行。请启动 Docker 服务。"
  fi
}

# ─── 生成密钥 ──────────────────────────────────────────
gen_secret() {
  if command -v openssl &>/dev/null; then
    openssl rand -hex 32
  else
    cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 64 | head -n 1
  fi
}

# ─── 启动 ──────────────────────────────────────────────
do_start() {
  check_docker

  if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    warn "容器 ${CONTAINER} 已存在，正在重启..."
    docker restart "${CONTAINER}"
    info "容器已重启"
    return
  fi

  JWT_SECRET="${JWT_SECRET:-$(gen_secret)}"
  info "JWT_SECRET: ${JWT_SECRET}"
  info "正在拉取镜像 ${IMAGE}:${TAG}..."
  docker pull "${IMAGE}:${TAG}"

  # 检测操作系统
  OS_TYPE="$(uname -s 2>/dev/null || echo Unknown)"

  if [ "$OS_TYPE" = "Linux" ]; then
    info "检测到 Linux 系统，挂载宿主机监控目录..."
    docker run -d \
      --name "${CONTAINER}" \
      --restart unless-stopped \
      -p "${PORT}:9090" \
      -v "${DATA_DIR}:/data" \
      -v /proc:/host/proc:ro \
      -v /sys:/host/sys:ro \
      -v /dev:/host/dev:ro \
      -v /etc/hostname:/etc/hostname:ro \
      -v /var/run/docker.sock:/var/run/docker.sock:ro \
      -e "JWT_SECRET=${JWT_SECRET}" \
      -e "ENCRYPTION_KEY=${ENCRYPTION_KEY:-}" \
      -e HOST_PROC=/host/proc \
      -e HOST_SYS=/host/sys \
      -e HOST_DEV=/host/dev \
      -e HOST_ETC=/host/etc \
      -e GIN_MODE=release \
      -e "TZ=${TZ}" \
      -e "INTERVAL=${INTERVAL:-5}" \
      --memory=512m \
      --cpus=2.0 \
      "${IMAGE}:${TAG}"
  else
    info "检测到非 Linux 系统，跳过宿主机目录挂载..."
    docker run -d \
      --name "${CONTAINER}" \
      --restart unless-stopped \
      -p "${PORT}:9090" \
      -v "${DATA_DIR}:/data" \
      -e "JWT_SECRET=${JWT_SECRET}" \
      -e "ENCRYPTION_KEY=${ENCRYPTION_KEY:-}" \
      -e GIN_MODE=release \
      -e "TZ=${TZ}" \
      -e "INTERVAL=${INTERVAL:-5}" \
      --memory=512m \
      --cpus=2.0 \
      "${IMAGE}:${TAG}"
  fi

  info "等待服务启动..."
  sleep 5

  if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    info "DevDash 启动成功！"
    info "访问地址: http://localhost:${PORT}"
    info "默认账号: admin / admin123"
    info "请尽快修改默认密码！"
  else
    error "启动失败，查看日志: docker logs ${CONTAINER}"
  fi
}

# ─── 停止 ──────────────────────────────────────────────
do_stop() {
  docker stop "${CONTAINER}" 2>/dev/null && info "已停止" || warn "容器未运行"
}

# ─── 重启 ──────────────────────────────────────────────
do_restart() {
  docker restart "${CONTAINER}" 2>/dev/null && info "已重启" || do_start
}

# ─── 状态 ──────────────────────────────────────────────
do_status() {
  if docker ps --format '{{.Names}}\t{{.Status}}' | grep "^${CONTAINER}"; then
    info "服务运行中"
  elif docker ps -a --format '{{.Names}}\t{{.Status}}' | grep "^${CONTAINER}"; then
    warn "服务已停止"
  else
    warn "容器不存在"
  fi
}

# ─── 日志 ──────────────────────────────────────────────
do_logs() {
  docker logs -f --tail 100 "${CONTAINER}" 2>/dev/null || warn "容器不存在"
}

# ─── 更新 ──────────────────────────────────────────────
do_update() {
  info "正在拉取最新镜像..."
  docker pull "${IMAGE}:${TAG}"
  info "正在重建容器..."
  docker stop "${CONTAINER}" 2>/dev/null
  docker rm "${CONTAINER}" 2>/dev/null
  do_start
}

# ─── 备份 ──────────────────────────────────────────────
do_backup() {
  BACKUP_FILE="devdash-backup-$(date +%Y%m%d-%H%M%S).db"
  docker cp "${CONTAINER}:/data/devdash.db" "./${BACKUP_FILE}" 2>/dev/null \
    && info "备份已保存: ${BACKUP_FILE}" \
    || error "备份失败，请确认容器正在运行"
}

# ─── 卸载 ──────────────────────────────────────────────
do_uninstall() {
  docker stop "${CONTAINER}" 2>/dev/null
  docker rm "${CONTAINER}" 2>/dev/null
  docker rmi "${IMAGE}:${TAG}" 2>/dev/null
  info "已卸载（数据卷 ${DATA_DIR} 保留，如需删除: docker volume rm ${DATA_DIR}）"
}

# ─── 主入口 ────────────────────────────────────────────
case "${1:-start}" in
  start)     do_start     ;;
  stop)      do_stop      ;;
  restart)   do_restart   ;;
  status)    do_status    ;;
  logs)      do_logs      ;;
  update)    do_update    ;;
  backup)    do_backup    ;;
  uninstall) do_uninstall ;;
  *)
    echo "DevDash Docker 部署工具"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  start      启动服务（默认）"
    echo "  stop       停止服务"
    echo "  restart    重启服务"
    echo "  status     查看状态"
    echo "  logs       查看日志"
    echo "  update     更新到最新版本"
    echo "  backup     备份数据库"
    echo "  uninstall  卸载服务"
    echo ""
    echo "环境变量:"
    echo "  VERSION        镜像版本 (默认: latest)"
    echo "  PORT           服务端口 (默认: 9090)"
    echo "  JWT_SECRET     JWT 密钥 (自动生成)"
    echo "  TZ             时区 (默认: Asia/Shanghai)"
    echo "  INTERVAL       采集间隔秒数 (默认: 5)"
    echo "  DATA_DIR       数据目录 (默认: devdash-data)"
    ;;
esac
