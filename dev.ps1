#Requires -Version 5.1
param(
    [Parameter(Position=0)]
    [ValidateSet('start', 'stop', 'restart', 'status', 'logs')]
    [string]$Command = 'start',

    [string]$LogService = 'all'
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

$PidDir = Join-Path $ScriptDir '.pids'
$LogDir = Join-Path $ScriptDir '.logs'
New-Item -ItemType Directory -Force -Path $PidDir, $LogDir | Out-Null

function Write-Info($msg)  { Write-Host "[INFO] $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Write-Err($msg)   { Write-Host "[ERROR] $msg" -ForegroundColor Red }
function Write-Step($msg)  { Write-Host "`n==> $msg" -ForegroundColor Cyan }

function Load-Env {
    $envFile = Join-Path $ScriptDir '.env.local'
    if (-not (Test-Path $envFile)) {
        $envFile = Join-Path $ScriptDir '.env.example'
    }
    if (Test-Path $envFile) {
        Get-Content $envFile | ForEach-Object {
            if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
                $key = $matches[1].Trim()
                $val = $matches[2].Trim()
                if (-not (Get-ChildItem env: | Where-Object { $_.Name -eq $key })) {
                    Set-Item "env:$key" $val
                }
            }
        }
        Write-Info "Loaded env from $envFile"
    }
}

function Start-Backend {
    Write-Step "Starting backend server..."
    $pidFile = Join-Path $PidDir 'backend.pid'

    if (Test-Path $pidFile) {
        $oldPid = Get-Content $pidFile -Raw
        try {
            $proc = Get-Process -Id $oldPid.Trim() -ErrorAction Stop
            if ($proc) {
                Write-Warn "Backend already running (PID: $($proc.Id))"
                return
            }
        } catch {}
        Remove-Item $pidFile -Force
    }

    $env:PORT = if ($env:PORT) { $env:PORT } else { '9090' }
    $env:JWT_SECRET = if ($env:JWT_SECRET) { $env:JWT_SECRET } else { 'devdash-dev-secret-key-min-32ch!!' }
    $env:ENCRYPTION_KEY = if ($env:ENCRYPTION_KEY) { $env:ENCRYPTION_KEY } else { 'devdash-encryption-key-32ch!!' }
    $env:DB_PATH = if ($env:DB_PATH) { $env:DB_PATH } else { './devdash.db' }
    $env:GIN_MODE = if ($env:GIN_MODE) { $env:GIN_MODE } else { 'debug' }
    $env:CORS_ORIGINS = if ($env:CORS_ORIGINS) { $env:CORS_ORIGINS } else { 'http://localhost:5173,http://localhost:9090' }

    $serverDir = Join-Path $ScriptDir 'server'
    $logFile = Join-Path $LogDir 'backend.log'

    $proc = Start-Process -FilePath 'go' -ArgumentList 'run', './cmd/server' `
        -WorkingDirectory $serverDir -RedirectStandardOutput $logFile `
        -RedirectStandardError (Join-Path $LogDir 'backend_err.log') `
        -NoNewWindow -PassThru

    $proc.Id | Set-Content $pidFile
    Write-Info "Backend started (PID: $($proc.Id), Port: $env:PORT)"
    Write-Info "Backend log: $logFile"
}

function Start-Frontend {
    Write-Step "Starting frontend dev server..."
    $pidFile = Join-Path $PidDir 'frontend.pid'

    if (Test-Path $pidFile) {
        $oldPid = Get-Content $pidFile -Raw
        try {
            $proc = Get-Process -Id $oldPid.Trim() -ErrorAction Stop
            if ($proc) {
                Write-Warn "Frontend already running (PID: $($proc.Id))"
                return
            }
        } catch {}
        Remove-Item $pidFile -Force
    }

    $webDir = Join-Path $ScriptDir 'web'

    if (-not (Test-Path (Join-Path $webDir 'node_modules'))) {
        Write-Info "Installing frontend dependencies..."
        Push-Location $webDir
        npm ci --prefer-offline 2>&1 | Select-Object -Last 3
        Pop-Location
    }

    $env:VITE_BACKEND_URL = if ($env:VITE_BACKEND_URL) { $env:VITE_BACKEND_URL } else { 'http://localhost:9090' }
    $logFile = Join-Path $LogDir 'frontend.log'

    $proc = Start-Process -FilePath 'npm' -ArgumentList 'run', 'dev' `
        -WorkingDirectory $webDir -RedirectStandardOutput $logFile `
        -RedirectStandardError (Join-Path $LogDir 'frontend_err.log') `
        -NoNewWindow -PassThru

    $proc.Id | Set-Content $pidFile
    Write-Info "Frontend started (PID: $($proc.Id), Port: 5173)"
    Write-Info "Frontend log: $logFile"
}

function Stop-Backend {
    Write-Step "Stopping backend server..."
    $pidFile = Join-Path $PidDir 'backend.pid'
    if (Test-Path $pidFile) {
        $pid = (Get-Content $pidFile -Raw).Trim()
        try {
            $proc = Get-Process -Id $pid -ErrorAction Stop
            Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            Write-Info "Backend stopped (PID: $pid)"
        } catch {
            Write-Warn "Backend process not found"
        }
        Remove-Item $pidFile -Force
    } else {
        Write-Warn "Backend not running"
    }
}

function Stop-Frontend {
    Write-Step "Stopping frontend dev server..."
    $pidFile = Join-Path $PidDir 'frontend.pid'
    if (Test-Path $pidFile) {
        $pid = (Get-Content $pidFile -Raw).Trim()
        try {
            $proc = Get-Process -Id $pid -ErrorAction Stop
            Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
            Write-Info "Frontend stopped (PID: $pid)"
        } catch {
            Write-Warn "Frontend process not found"
        }
        Remove-Item $pidFile -Force
    } else {
        Write-Warn "Frontend not running"
    }
}

function Stop-All {
    Stop-Frontend
    Stop-Backend
    Write-Info "All services stopped"
}

function Show-Status {
    Write-Host "`n=== DevDash Service Status ===" -ForegroundColor Cyan
    foreach ($svc in @('backend', 'frontend')) {
        $pidFile = Join-Path $PidDir "$svc.pid"
        if (Test-Path $pidFile) {
            $pid = (Get-Content $pidFile -Raw).Trim()
            try {
                $proc = Get-Process -Id $pid -ErrorAction Stop
                Write-Host "  ${svc}: " -NoNewline; Write-Host "running" -ForegroundColor Green -NoNewline; Write-Host " (PID: $pid)"
            } catch {
                Write-Host "  ${svc}: " -NoNewline; Write-Host "stopped" -ForegroundColor Red -NoNewline; Write-Host " (stale PID)"
            }
        } else {
            Write-Host "  ${svc}: " -NoNewline; Write-Host "not started" -ForegroundColor Yellow
        }
    }
    Write-Host ""
}

function Show-Logs($svc) {
    if ($svc -eq 'backend' -or $svc -eq 'all') {
        Write-Host "`n=== Backend Logs (last 30 lines) ===" -ForegroundColor Cyan
        $logFile = Join-Path $LogDir 'backend.log'
        if (Test-Path $logFile) { Get-Content $logFile -Tail 30 } else { Write-Host "(no logs)" }
    }
    if ($svc -eq 'frontend' -or $svc -eq 'all') {
        Write-Host "`n=== Frontend Logs (last 30 lines) ===" -ForegroundColor Cyan
        $logFile = Join-Path $LogDir 'frontend.log'
        if (Test-Path $logFile) { Get-Content $logFile -Tail 30 } else { Write-Host "(no logs)" }
    }
}

function Wait-ForBackend {
    $port = if ($env:PORT) { $env:PORT } else { '9090' }
    $maxAttempts = 30
    Write-Info "Waiting for backend on port $port..."
    for ($i = 1; $i -le $maxAttempts; $i++) {
        try {
            $resp = Invoke-WebRequest -Uri "http://localhost:${port}/api/v1/health" -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
            Write-Info "Backend is ready! (attempt $i/$maxAttempts)"
            return
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    Write-Warn "Backend did not become ready within ${maxAttempts}s"
}

Load-Env

switch ($Command) {
    'start' {
        Write-Step "Starting DevDash development environment"
        if ($env:SKIP_BACKEND -ne '1') {
            Start-Backend
            Wait-ForBackend
        }
        if ($env:SKIP_FRONTEND -ne '1') {
            Start-Frontend
        }
        Write-Host ""
        Write-Info "DevDash is running!"
        if ($env:SKIP_BACKEND -ne '1') {
            Write-Host "  Backend:  " -NoNewline; Write-Host "http://localhost:$($env:PORT ?? '9090')" -ForegroundColor Green
        }
        if ($env:SKIP_FRONTEND -ne '1') {
            Write-Host "  Frontend: " -NoNewline; Write-Host "http://localhost:5173" -ForegroundColor Green
        }
        Write-Host ""
    }
    'stop'    { Stop-All }
    'restart' { Stop-All; Start-Sleep -Seconds 2; & $MyInvocation.MyCommand.Path 'start' }
    'status'  { Show-Status }
    'logs'    { Show-Logs $LogService }
}
