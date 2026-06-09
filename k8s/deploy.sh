#!/usr/bin/env bash
# DevDash Kubernetes 部署脚本
# 用法: ./k8s/deploy.sh [命令]
# 命令: deploy | destroy | status | upgrade | logs | scale

set -euo pipefail

NS="devdash"
DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

check_kubectl() {
  if ! command -v kubectl &>/dev/null; then
    error "kubectl 未安装。请先安装: https://kubernetes.io/docs/tasks/tools/"
  fi
  if ! kubectl cluster-info &>/dev/null; then
    error "无法连接 Kubernetes 集群。请检查 kubeconfig。"
  fi
}

do_deploy() {
  check_kubectl
  info "部署 DevDash 到 Kubernetes..."
  kubectl apply -f "${DIR}/deployment.yml"

  info "等待 Pod 就绪..."
  kubectl -n "${NS}" rollout status deployment/devdash --timeout=120s

  info "获取访问信息..."
  local ip
  ip=$(kubectl -n "${NS}" get svc devdash -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "pending")
  info "ClusterIP: ${ip}:9090"
  info ""
  info "端口转发测试: kubectl -n ${NS} port-forward svc/devdash 9090:9090"
  info "然后访问: http://localhost:9090"
  info "默认账号: admin / admin123"
}

do_destroy() {
  check_kubectl
  warn "将删除 DevDash 所有资源（包括数据）..."
  read -rp "确认删除？(y/N) " confirm
  [[ "${confirm,,}" == "y" ]] || { info "已取消"; return; }
  kubectl delete -f "${DIR}/deployment.yml"
  info "已删除"
}

do_status() {
  check_kubectl
  kubectl -n "${NS}" get all -l app=devdash
}

do_upgrade() {
  check_kubectl
  local image="${1:-ghcr.io/gxfdev/devdash:latest}"
  info "更新镜像为 ${image}..."
  kubectl -n "${NS}" set image deployment/devdash devdash="${image}"
  kubectl -n "${NS}" rollout status deployment/devdash --timeout=120s
  info "更新完成"
}

do_logs() {
  check_kubectl
  kubectl -n "${NS}" logs -f -l app=devdash --tail=100
}

do_scale() {
  check_kubectl
  local replicas="${1:-1}"
  info "扩缩容到 ${replicas} 副本..."
  kubectl -n "${NS}" scale deployment/devdash --replicas="${replicas}"
}

case "${1:-deploy}" in
  deploy)  do_deploy  ;;
  destroy) do_destroy ;;
  status)  do_status  ;;
  upgrade) do_upgrade "${2:-}" ;;
  logs)    do_logs    ;;
  scale)   do_scale "${2:-1}" ;;
  *)
    echo "DevDash Kubernetes 部署工具"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  deploy          部署到集群（默认）"
    echo "  destroy         删除所有资源"
    echo "  status          查看资源状态"
    echo "  upgrade [镜像]  更新镜像版本"
    echo "  logs            查看日志"
    echo "  scale [N]       扩缩容（默认 1）"
    ;;
esac
