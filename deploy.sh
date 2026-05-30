#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "\n${BLUE}==>${NC} $1"; }

DEPLOY_ENV="${DEPLOY_ENV:-production}"
DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_USER="${DEPLOY_USER:-root}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
DEPLOY_PATH="${DEPLOY_PATH:-/opt/devdash}"
BACKUP_PATH="${BACKUP_PATH:-/opt/devdash/backups}"
MAX_BACKUPS="${MAX_BACKUPS:-5}"

ENV_FILES=()
ENV_FILES[production]=".env.prod"
ENV_FILES[staging]=".env.staging"
ENV_FILES[development]=".env.dev"

load_deploy_env() {
    local env_file="${ENV_FILES[$DEPLOY_ENV]:-.env.prod}"
    if [ -f "$SCRIPT_DIR/$env_file" ]; then
        set -a
        source "$SCRIPT_DIR/$env_file"
        set +a
        log_info "Loaded $DEPLOY_ENV config from $env_file"
    fi
}

check_ssh() {
    if [ -z "$DEPLOY_HOST" ]; then
        log_error "DEPLOY_HOST not set. Usage: DEPLOY_HOST=1.2.3.4 $0 deploy"
        exit 1
    fi
    log_info "Target: $DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PORT (env: $DEPLOY_ENV)"
    ssh -o ConnectTimeout=10 -o BatchMode=yes -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" "echo ok" > /dev/null 2>&1 || {
        log_error "Cannot connect to $DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PORT"
        exit 1
    }
    log_info "SSH connection OK"
}

check_server() {
    log_step "Checking server prerequisites..."
    ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" bash -s <<'CHECK'
        echo "=== OS Info ==="
        cat /etc/os-release | head -3
        echo "=== Docker ==="
        docker --version 2>/dev/null || echo "NOT INSTALLED"
        echo "=== Docker Compose ==="
        docker compose version 2>/dev/null || docker-compose --version 2>/dev/null || echo "NOT INSTALLED"
        echo "=== Disk Space ==="
        df -h / | tail -1
        echo "=== Memory ==="
        free -h | head -2
        echo "=== Ports ==="
        ss -tlnp | grep -E ':(80|443|9090)\b' || echo "Ports 80/443/9090 available"
CHECK
}

build_locally() {
    log_step "Building frontend..."
    cd "$SCRIPT_DIR/web"
    if [ ! -d "node_modules" ]; then
        npm ci --prefer-offline
    fi
    npm run build
    log_info "Frontend built successfully"

    log_step "Building backend binary..."
    cd "$SCRIPT_DIR/server"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$SCRIPT_DIR/dist/devdash-linux-amd64" ./cmd/server
    log_info "Backend binary built successfully"
    cd "$SCRIPT_DIR"
}

create_backup() {
    log_step "Creating backup on server..."
    ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" bash -s <<REMOTE
        set -e
        mkdir -p "$BACKUP_PATH"
        TIMESTAMP=\$(date +%Y%m%d_%H%M%S)
        BACKUP_NAME="devdash_\${TIMESTAMP}.tar.gz"

        if [ -d "$DEPLOY_PATH" ]; then
            tar -czf "$BACKUP_PATH/\${BACKUP_NAME}" -C "$DEPLOY_PATH" \
                --exclude='backups' \
                --exclude='node_modules' \
                --exclude='.git' \
                . 2>/dev/null || true
            echo "Backup created: \${BACKUP_NAME}"

            cd "$BACKUP_PATH"
            ls -t devdash_*.tar.gz 2>/dev/null | tail -n +\$(($MAX_BACKUPS + 1)) | xargs -r rm -f
            echo "Old backups cleaned (keeping last $MAX_BACKUPS)"
        else
            echo "No existing deployment to backup"
        fi
REMOTE
    log_info "Backup completed"
}

transfer_files() {
    log_step "Transferring files to server..."
    ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" "mkdir -p $DEPLOY_PATH/dist"

    scp -P "$DEPLOY_PORT" "$SCRIPT_DIR/dist/devdash-linux-amd64" \
        "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PATH/dist/devdash"

    scp -P "$DEPLOY_PORT" "$SCRIPT_DIR/docker-compose.yml" \
        "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PATH/docker-compose.yml"

    scp -P "$DEPLOY_PORT" "$SCRIPT_DIR/docker/nginx.conf" \
        "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PATH/docker/nginx.conf"

    scp -P "$DEPLOY_PORT" "$SCRIPT_DIR/docker/Dockerfile.server" \
        "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PATH/docker/Dockerfile.server"

    local env_file="${ENV_FILES[$DEPLOY_ENV]:-.env.prod}"
    if [ -f "$SCRIPT_DIR/$env_file" ]; then
        scp -P "$DEPLOY_PORT" "$SCRIPT_DIR/$env_file" \
            "$DEPLOY_USER@$DEPLOY_HOST:$DEPLOY_PATH/.env"
    fi

    log_info "Files transferred successfully"
}

deploy_services() {
    log_step "Deploying services on server..."
    ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" bash -s <<REMOTE
        set -e
        cd "$DEPLOY_PATH"

        echo "Building and starting services..."
        docker compose up -d --build --remove-orphans

        echo "Waiting for services to be healthy..."
        for i in \$(seq 1 30); do
            if curl -sf http://localhost:9090/api/v1/health > /dev/null 2>&1; then
                echo "Services are healthy!"
                exit 0
            fi
            sleep 2
        done
        echo "WARNING: Services did not become healthy within 60s"
        docker compose ps
        docker compose logs --tail=50
REMOTE
    log_info "Deployment completed"
}

rollback() {
    log_step "Rolling back to previous deployment..."
    ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" bash -s <<REMOTE
        set -e
        cd "$BACKUP_PATH"
        LATEST_BACKUP=\$(ls -t devdash_*.tar.gz 2>/dev/null | head -1)
        if [ -z "\$LATEST_BACKUP" ]; then
            echo "ERROR: No backup found for rollback"
            exit 1
        fi

        echo "Rolling back with: \$LATEST_BACKUP"
        cd "$DEPLOY_PATH"
        docker compose down 2>/dev/null || true
        tar -xzf "$BACKUP_PATH/\$LATEST_BACKUP"
        docker compose up -d --remove-orphans
        echo "Rollback completed"
REMOTE
    log_info "Rollback completed"
}

verify_deployment() {
    log_step "Verifying deployment..."
    local health_url="http://$DEPLOY_HOST/api/v1/health"
    for i in $(seq 1 15); do
        if curl -sf "$health_url" > /dev/null 2>&1; then
            log_info "Deployment verified! Health check passed."
            return 0
        fi
        sleep 2
    done
    log_error "Deployment verification failed! Consider running: $0 rollback"
    return 1
}

usage() {
    echo -e "${BLUE}DevDash Deployment Script${NC}"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  check      Check server prerequisites"
    echo "  deploy     Full deployment (build + backup + transfer + deploy)"
    echo "  rollback   Rollback to previous deployment"
    echo "  status     Check deployment status"
    echo ""
    echo "Environment variables:"
    echo "  DEPLOY_HOST    Server IP/hostname (required)"
    echo "  DEPLOY_USER    SSH user (default: root)"
    echo "  DEPLOY_PORT    SSH port (default: 22)"
    echo "  DEPLOY_PATH    Install path (default: /opt/devdash)"
    echo "  DEPLOY_ENV     Environment: production|staging|development"
    echo "  MAX_BACKUPS    Max backup count (default: 5)"
    echo ""
    echo "Examples:"
    echo "  DEPLOY_HOST=1.2.3.4 $0 deploy"
    echo "  DEPLOY_HOST=1.2.3.4 DEPLOY_ENV=staging $0 deploy"
    echo "  DEPLOY_HOST=1.2.3.4 $0 rollback"
}

main() {
    local cmd="${1:-}"
    shift 2>/dev/null || true

    load_deploy_env

    case "$cmd" in
        check)
            check_ssh
            check_server
            ;;
        deploy)
            check_ssh
            build_locally
            create_backup
            transfer_files
            deploy_services
            verify_deployment
            ;;
        rollback)
            check_ssh
            rollback
            ;;
        status)
            check_ssh
            ssh -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" bash -s <<'REMOTE'
                cd "$DEPLOY_PATH" 2>/dev/null || { echo "Not deployed"; exit 1; }
                docker compose ps
                echo ""
                curl -sf http://localhost:9090/api/v1/health && echo " - Health: OK" || echo " - Health: FAIL"
REMOTE
            ;;
        *)
            usage
            ;;
    esac
}

main "$@"
