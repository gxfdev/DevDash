#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

PID_DIR="$SCRIPT_DIR/.pids"
LOG_DIR="$SCRIPT_DIR/.logs"
mkdir -p "$PID_DIR" "$LOG_DIR"

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "\n${BLUE}==>${NC} $1"; }

load_env() {
    local env_file="$SCRIPT_DIR/.env.local"
    if [ ! -f "$env_file" ]; then
        env_file="$SCRIPT_DIR/.env.example"
    fi
    if [ -f "$env_file" ]; then
        set -a
        source "$env_file"
        set +a
        log_info "Loaded env from $env_file"
    fi
}

check_prereqs() {
    local missing=()
    if [ "${SKIP_BACKEND:-}" != "1" ] && ! command -v go &> /dev/null; then
        missing+=("go")
    fi
    if [ "${SKIP_FRONTEND:-}" != "1" ] && ! command -v node &> /dev/null; then
        missing+=("node")
    fi
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing prerequisites: ${missing[*]}"
        exit 1
    fi
}

start_backend() {
    log_step "Starting backend server..."
    local pid_file="$PID_DIR/backend.pid"

    if [ -f "$pid_file" ]; then
        local old_pid
        old_pid=$(cat "$pid_file")
        if kill -0 "$old_pid" 2>/dev/null; then
            log_warn "Backend already running (PID: $old_pid)"
            return 0
        fi
        rm -f "$pid_file"
    fi

    cd "$SCRIPT_DIR/server"

    export PORT="${PORT:-9090}"
    export JWT_SECRET="${JWT_SECRET:-devdash-dev-secret-key-min-32ch!!}"
    export ENCRYPTION_KEY="${ENCRYPTION_KEY:-devdash-encryption-key-32ch!!}"
    export DB_PATH="${DB_PATH:-./devdash.db}"
    export GIN_MODE="${GIN_MODE:-debug}"
    export CORS_ORIGINS="${CORS_ORIGINS:-http://localhost:5173,http://localhost:9090}"

    if [ "${HOT_RELOAD:-1}" = "1" ] && command -v air &> /dev/null; then
        log_info "Starting with hot-reload (air)"
        air -c .air.toml > "$LOG_DIR/backend.log" 2>&1 &
    else
        log_info "Starting with go run"
        go run ./cmd/server > "$LOG_DIR/backend.log" 2>&1 &
    fi

    local pid=$!
    echo "$pid" > "$pid_file"
    log_info "Backend started (PID: $pid, Port: $PORT)"
    log_info "Backend log: $LOG_DIR/backend.log"

    cd "$SCRIPT_DIR"
}

start_frontend() {
    log_step "Starting frontend dev server..."
    local pid_file="$PID_DIR/frontend.pid"

    if [ -f "$pid_file" ]; then
        local old_pid
        old_pid=$(cat "$pid_file")
        if kill -0 "$old_pid" 2>/dev/null; then
            log_warn "Frontend already running (PID: $old_pid)"
            return 0
        fi
        rm -f "$pid_file"
    fi

    cd "$SCRIPT_DIR/web"

    if [ ! -d "node_modules" ]; then
        log_info "Installing frontend dependencies..."
        npm ci --prefer-offline 2>&1 | tail -3
    fi

    export VITE_BACKEND_URL="${VITE_BACKEND_URL:-http://localhost:9090}"

    npm run dev > "$LOG_DIR/frontend.log" 2>&1 &
    local pid=$!
    echo "$pid" > "$pid_file"
    log_info "Frontend started (PID: $pid, Port: 5173)"
    log_info "Frontend log: $LOG_DIR/frontend.log"

    cd "$SCRIPT_DIR"
}

stop_backend() {
    log_step "Stopping backend server..."
    local pid_file="$PID_DIR/backend.pid"
    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            sleep 1
            kill -9 "$pid" 2>/dev/null || true
            log_info "Backend stopped (PID: $pid)"
        fi
        rm -f "$pid_file"
    else
        log_warn "Backend not running"
    fi
}

stop_frontend() {
    log_step "Stopping frontend dev server..."
    local pid_file="$PID_DIR/frontend.pid"
    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            sleep 1
            kill -9 "$pid" 2>/dev/null || true
            log_info "Frontend stopped (PID: $pid)"
        fi
        rm -f "$pid_file"
    else
        log_warn "Frontend not running"
    fi
}

stop_all() {
    stop_frontend
    stop_backend
    log_info "All services stopped"
}

show_status() {
    echo -e "\n${CYAN}=== DevDash Service Status ===${NC}"
    for svc in backend frontend; do
        local pid_file="$PID_DIR/${svc}.pid"
        if [ -f "$pid_file" ]; then
            local pid
            pid=$(cat "$pid_file")
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "  ${svc}: ${GREEN}running${NC} (PID: $pid)"
            else
                echo -e "  ${svc}: ${RED}stopped${NC} (stale PID)"
            fi
        else
            echo -e "  ${svc}: ${YELLOW}not started${NC}"
        fi
    done
    echo ""
}

show_logs() {
    local svc="${1:-all}"
    if [ "$svc" = "backend" ] || [ "$svc" = "all" ]; then
        echo -e "\n${CYAN}=== Backend Logs (last 30 lines) ===${NC}"
        tail -30 "$LOG_DIR/backend.log" 2>/dev/null || echo "(no logs)"
    fi
    if [ "$svc" = "frontend" ] || [ "$svc" = "all" ]; then
        echo -e "\n${CYAN}=== Frontend Logs (last 30 lines) ===${NC}"
        tail -30 "$LOG_DIR/frontend.log" 2>/dev/null || echo "(no logs)"
    fi
}

wait_for_backend() {
    local port="${PORT:-9090}"
    local max_attempts=30
    local attempt=1
    log_info "Waiting for backend on port $port..."
    while [ $attempt -le $max_attempts ]; do
        if curl -sf "http://localhost:${port}/api/v1/health" > /dev/null 2>&1; then
            log_info "Backend is ready! (attempt ${attempt}/${max_attempts})"
            return 0
        fi
        sleep 1
        attempt=$((attempt + 1))
    done
    log_warn "Backend did not become ready within ${max_attempts}s"
    return 1
}

usage() {
    echo -e "${CYAN}DevDash Development Script${NC}"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  start       Start all services (default)"
    echo "  stop        Stop all services"
    echo "  restart     Restart all services"
    echo "  status      Show service status"
    echo "  logs [svc]  Show logs (backend|frontend|all)"
    echo ""
    echo "Options (environment variables):"
    echo "  SKIP_BACKEND=1     Skip backend startup"
    echo "  SKIP_FRONTEND=1    Skip frontend startup"
    echo "  HOT_RELOAD=0       Disable hot-reload (air)"
    echo "  PORT=9090          Backend port"
    echo ""
    echo "Examples:"
    echo "  $0 start                    # Start both services"
    echo "  $0 start SKIP_FRONTEND=1    # Start backend only"
    echo "  $0 logs backend             # Show backend logs"
    echo "  $0 stop                     # Stop all services"
}

main() {
    local cmd="${1:-start}"
    shift 2>/dev/null || true

    load_env
    check_prereqs

    case "$cmd" in
        start)
            log_step "Starting DevDash development environment"
            [ "${SKIP_BACKEND:-}" != "1" ] && start_backend
            [ "${SKIP_BACKEND:-}" != "1" ] && wait_for_backend
            [ "${SKIP_FRONTEND:-}" != "1" ] && start_frontend
            echo ""
            log_info "DevDash is running!"
            [ "${SKIP_BACKEND:-}" != "1" ] && echo -e "  Backend:  ${GREEN}http://localhost:${PORT:-9090}${NC}"
            [ "${SKIP_FRONTEND:-}" != "1" ] && echo -e "  Frontend: ${GREEN}http://localhost:5173${NC}"
            echo ""
            ;;
        stop)
            stop_all
            ;;
        restart)
            stop_all
            sleep 2
            main start
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "${1:-all}"
            ;;
        *)
            usage
            ;;
    esac
}

main "$@"
