#Requires -Version 5.1
param(
    [Parameter(Position=0)]
    [ValidateSet('check', 'deploy', 'rollback', 'status')]
    [string]$Command = 'deploy',

    [string]$DeployHost = $env:DEPLOY_HOST,
    [string]$DeployUser = if ($env:DEPLOY_USER) { $env:DEPLOY_USER } else { 'root' },
    [int]$DeployPort = if ($env:DEPLOY_PORT) { [int]$env:DEPLOY_PORT } else { 22 },
    [string]$DeployPath = if ($env:DEPLOY_PATH) { $env:DEPLOY_PATH } else { '/opt/devdash' },
    [string]$DeployEnv = if ($env:DEPLOY_ENV) { $env:DEPLOY_ENV } else { 'production' }
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

function Write-Info($msg)  { Write-Host "[INFO] $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Write-Err($msg)   { Write-Host "[ERROR] $msg" -ForegroundColor Red }
function Write-Step($msg)  { Write-Host "`n==> $msg" -ForegroundColor Cyan }

function Invoke-Remote($script) {
    $tempFile = [System.IO.Path]::GetTempFileName()
    try {
        $script | Set-Content $tempFile -Encoding UTF8
        & scp -P $DeployPort $tempFile "${DeployUser}@${DeployHost}:/tmp/devdash_remote.sh" 2>$null
        & ssh -p $DeployPort "${DeployUser}@${DeployHost}" "bash /tmp/devdash_remote.sh; rm -f /tmp/devdash_remote.sh"
    } finally {
        Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
    }
}

function Test-SshConnection {
    if (-not $DeployHost) {
        Write-Err "DEPLOY_HOST not set. Usage: .\deploy.ps1 -DeployHost 1.2.3.4 -Command deploy"
        exit 1
    }
    Write-Info "Target: ${DeployUser}@${DeployHost}:${DeployPort} (env: $DeployEnv)"
    try {
        & ssh -o ConnectTimeout=10 -o BatchMode=yes -p $DeployPort "${DeployUser}@${DeployHost}" "echo ok" 2>$null | Out-Null
        Write-Info "SSH connection OK"
    } catch {
        Write-Err "Cannot connect to ${DeployUser}@${DeployHost}:${DeployPort}"
        exit 1
    }
}

function Build-Locally {
    Write-Step "Building frontend..."
    Push-Location (Join-Path $ScriptDir 'web')
    if (-not (Test-Path 'node_modules')) {
        & npm ci --prefer-offline
    }
    & npm run build
    Pop-Location
    Write-Info "Frontend built successfully"

    Write-Step "Building backend binary..."
    Push-Location (Join-Path $ScriptDir 'server')
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    & go build -ldflags="-s -w" -o (Join-Path $ScriptDir 'dist' 'devdash-linux-amd64') ./cmd/server
    Pop-Location
    Write-Info "Backend binary built successfully"
}

function New-Backup {
    Write-Step "Creating backup on server..."
    Invoke-Remote @"
        set -e
        mkdir -p ${DeployPath}/backups
        TIMESTAMP=`$(date +%Y%m%d_%H%M%S)
        BACKUP_NAME="devdash_`${TIMESTAMP}.tar.gz"
        if [ -d "${DeployPath}" ]; then
            tar -czf "${DeployPath}/backups/`${BACKUP_NAME}" -C "${DeployPath}" \
                --exclude='backups' --exclude='node_modules' --exclude='.git' . 2>/dev/null || true
            echo "Backup created: `${BACKUP_NAME}"
            cd "${DeployPath}/backups"
            ls -t devdash_*.tar.gz 2>/dev/null | tail -n +6 | xargs -r rm -f
        else
            echo "No existing deployment to backup"
        fi
"@
    Write-Info "Backup completed"
}

function Send-Files {
    Write-Step "Transferring files to server..."
    & ssh -p $DeployPort "${DeployUser}@${DeployHost}" "mkdir -p ${DeployPath}/dist ${DeployPath}/docker" 2>$null

    & scp -P $DeployPort (Join-Path $ScriptDir 'dist' 'devdash-linux-amd64') "${DeployUser}@${DeployHost}:${DeployPath}/dist/devdash"
    & scp -P $DeployPort (Join-Path $ScriptDir 'docker-compose.yml') "${DeployUser}@${DeployHost}:${DeployPath}/docker-compose.yml"
    & scp -P $DeployPort (Join-Path $ScriptDir 'docker' 'nginx.conf') "${DeployUser}@${DeployHost}:${DeployPath}/docker/nginx.conf"
    & scp -P $DeployPort (Join-Path $ScriptDir 'docker' 'Dockerfile.server') "${DeployUser}@${DeployHost}:${DeployPath}/docker/Dockerfile.server"

    Write-Info "Files transferred successfully"
}

function Deploy-Services {
    Write-Step "Deploying services on server..."
    Invoke-Remote @"
        set -e
        cd ${DeployPath}
        docker compose up -d --build --remove-orphans
        echo "Waiting for services to be healthy..."
        for i in `$(seq 1 30); do
            if curl -sf http://localhost:9090/api/v1/health > /dev/null 2>&1; then
                echo "Services are healthy!"
                exit 0
            fi
            sleep 2
        done
        echo "WARNING: Services did not become healthy within 60s"
        docker compose ps
        docker compose logs --tail=50
"@
    Write-Info "Deployment completed"
}

function Invoke-Rollback {
    Write-Step "Rolling back to previous deployment..."
    Invoke-Remote @"
        set -e
        cd ${DeployPath}/backups
        LATEST_BACKUP=`$(ls -t devdash_*.tar.gz 2>/dev/null | head -1)
        if [ -z "`$LATEST_BACKUP" ]; then
            echo "ERROR: No backup found for rollback"
            exit 1
        fi
        echo "Rolling back with: `$LATEST_BACKUP"
        cd ${DeployPath}
        docker compose down 2>/dev/null || true
        tar -xzf "${DeployPath}/backups/`$LATEST_BACKUP"
        docker compose up -d --remove-orphans
        echo "Rollback completed"
"@
    Write-Info "Rollback completed"
}

switch ($Command) {
    'check' {
        Test-SshConnection
        Write-Step "Checking server prerequisites..."
        Invoke-Remote @"
            echo '=== OS Info ==='
            cat /etc/os-release | head -3
            echo '=== Docker ==='
            docker --version 2>/dev/null || echo 'NOT INSTALLED'
            echo '=== Docker Compose ==='
            docker compose version 2>/dev/null || docker-compose --version 2>/dev/null || echo 'NOT INSTALLED'
            echo '=== Disk Space ==='
            df -h / | tail -1
            echo '=== Memory ==='
            free -h | head -2
            echo '=== Ports ==='
            ss -tlnp | grep -E ':(80|443|9090)\b' || echo 'Ports 80/443/9090 available'
"@
    }
    'deploy' {
        Test-SshConnection
        Build-Locally
        New-Backup
        Send-Files
        Deploy-Services
        Write-Info "Deployment complete!"
    }
    'rollback' {
        Test-SshConnection
        Invoke-Rollback
    }
    'status' {
        Test-SshConnection
        Invoke-Remote @"
            cd ${DeployPath} 2>/dev/null || { echo 'Not deployed'; exit 1; }
            docker compose ps
            echo ''
            curl -sf http://localhost:9090/api/v1/health && echo ' - Health: OK' || echo ' - Health: FAIL'
"@
    }
}
