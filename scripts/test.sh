#!/usr/bin/env bash
# DevDash 测试脚本
# 用法: ./scripts/test.sh [unit|integration|all|coverage]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

MODE="${1:-all}"

run_unit_tests() {
    log_info "运行单元测试..."
    cd "$PROJECT_DIR/server"
    go test -v -count=1 -timeout 60s ./internal/... 2>&1 | tee /tmp/devdash_unit_test.log
    local result=${PIPESTATUS[0]}
    if [ $result -eq 0 ]; then
        log_info "单元测试通过"
    else
        log_error "单元测试失败"
        return 1
    fi
}

run_integration_tests() {
    log_info "运行集成测试..."
    cd "$PROJECT_DIR/server"
    go test -v -count=1 -timeout 120s -tags integration ./tests/... 2>&1 | tee /tmp/devdash_integration_test.log
    local result=${PIPESTATUS[0]}
    if [ $result -eq 0 ]; then
        log_info "集成测试通过"
    else
        log_error "集成测试失败"
        return 1
    fi
}

run_coverage() {
    log_info "生成测试覆盖率报告..."
    cd "$PROJECT_DIR/server"
    go test -coverprofile=coverage.out -timeout 120s ./internal/... 2>&1
    go tool cover -html=coverage.out -o coverage.html
    local total
    total=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    log_info "总覆盖率: ${total}"
    log_info "HTML报告: server/coverage.html"
}

run_frontend_tests() {
    log_info "运行前端测试..."
    cd "$PROJECT_DIR/web"
    if [ -f package.json ] && grep -q '"test"' package.json; then
        npm test 2>&1 | tee /tmp/devdash_frontend_test.log
    else
        log_info "前端未配置测试，跳过"
    fi
}

run_lint() {
    log_info "运行代码检查..."
    cd "$PROJECT_DIR/server"
    if command -v golangci-lint > /dev/null 2>&1; then
        golangci-lint run ./...
    else
        go vet ./...
    fi
    cd "$PROJECT_DIR/web"
    if [ -f package.json ]; then
        npx vue-tsc --noEmit 2>&1 || true
    fi
}

case "$MODE" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    coverage)
        run_coverage
        ;;
    lint)
        run_lint
        ;;
    frontend)
        run_frontend_tests
        ;;
    all)
        echo -e "${CYAN}=== DevDash 完整测试 ===${NC}"
        echo ""
        run_lint
        echo ""
        run_unit_tests
        echo ""
        run_frontend_tests
        echo ""
        echo -e "${GREEN}=== 所有测试完成 ===${NC}"
        ;;
    *)
        echo "用法: $0 [unit|integration|coverage|lint|frontend|all]"
        echo ""
        echo "  unit         单元测试"
        echo "  integration  集成测试"
        echo "  coverage     覆盖率报告"
        echo "  lint         代码检查"
        echo "  frontend     前端测试"
        echo "  all          全部测试（默认）"
        exit 1
        ;;
esac
