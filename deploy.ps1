# DevDash Docker 一键部署脚本 (PowerShell)
# 用法: .\deploy.ps1 [命令]
# 命令: start | stop | restart | status | logs | update | backup | uninstall

param(
    [string]$Command = "start"
)

$ErrorActionPreference = "Stop"
$Image = "ghcr.io/gxfdev/devdash"
$Tag = if ($env:VERSION) { $env:VERSION } else { "latest" }
$Container = "devdash"
$Port = if ($env:PORT) { $env:PORT } else { "9090" }
$TZ = if ($env:TZ) { $env:TZ } else { "Asia/Shanghai" }
$Interval = if ($env:INTERVAL) { $env:INTERVAL } else { "5" }

function Write-Info($msg)  { Write-Host "[INFO] $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Write-Err($msg)   { Write-Host "[ERROR] $msg" -ForegroundColor Red; exit 1 }

function Check-Docker {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-Err "Docker 未安装。请先安装 Docker Desktop: https://docs.docker.com/desktop/install/windows-install/"
    }
    try { docker info | Out-Null } catch { Write-Err "Docker 未运行。请启动 Docker Desktop。" }
}

function Gen-Secret {
    -join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
}

function Do-Start {
    Check-Docker

    $existing = docker ps -a --format '{{.Names}}' 2>$null
    if ($existing -contains $Container) {
        Write-Warn "容器 $Container 已存在，正在重启..."
        docker restart $Container
        Write-Info "容器已重启"
        return
    }

    $jwtSecret = if ($env:JWT_SECRET) { $env:JWT_SECRET } else { Gen-Secret }
    Write-Info "JWT_SECRET: $jwtSecret"
    Write-Info "正在拉取镜像 ${Image}:${Tag}..."
    docker pull "${Image}:${Tag}"

    Write-Info "检测到 Windows 系统，跳过宿主机目录挂载..."
    docker run -d `
        --name $Container `
        --restart unless-stopped `
        -p "${Port}:9090" `
        -v "devdash-data:/data" `
        -e "JWT_SECRET=$jwtSecret" `
        -e "ENCRYPTION_KEY=$($env:ENCRYPTION_KEY)" `
        -e GIN_MODE=release `
        -e "TZ=$TZ" `
        -e "INTERVAL=$Interval" `
        --memory=512m `
        --cpus=2.0 `
        "${Image}:${Tag}"

    Write-Info "等待服务启动..."
    Start-Sleep -Seconds 5

    $running = docker ps --format '{{.Names}}' 2>$null
    if ($running -contains $Container) {
        Write-Info "DevDash 启动成功！"
        Write-Info "访问地址: http://localhost:${Port}"
        Write-Info "默认账号: admin / admin123"
        Write-Info "请尽快修改默认密码！"
    } else {
        Write-Err "启动失败，查看日志: docker logs $Container"
    }
}

function Do-Stop     { docker stop $Container 2>$null; Write-Info "已停止" }
function Do-Restart  { docker restart $Container 2>$null; Write-Info "已重启" }
function Do-Status   { docker ps -a --format 'table {{.Names}}`t{{.Status}}' | Select-String $Container }
function Do-Logs     { docker logs -f --tail 100 $Container 2>$null }
function Do-Update   { docker pull "${Image}:${Tag}"; docker stop $Container 2>$null; docker rm $Container 2>$null; Do-Start }
function Do-Backup   { $f = "devdash-backup-$(Get-Date -Format 'yyyyMMdd-HHmmss').db"; docker cp "${Container}:/data/devdash.db" "./$f"; Write-Info "备份已保存: $f" }
function Do-Uninstall { docker stop $Container 2>$null; docker rm $Container 2>$null; docker rmi "${Image}:${Tag}" 2>$null; Write-Info "已卸载" }

switch ($Command) {
    "start"     { Do-Start }
    "stop"      { Do-Stop }
    "restart"   { Do-Restart }
    "status"    { Do-Status }
    "logs"      { Do-Logs }
    "update"    { Do-Update }
    "backup"    { Do-Backup }
    "uninstall" { Do-Uninstall }
    default {
        Write-Host "DevDash Docker 部署工具 (PowerShell)"
        Write-Host ""
        Write-Host "用法: .\deploy.ps1 [命令]"
        Write-Host ""
        Write-Host "命令: start | stop | restart | status | logs | update | backup | uninstall"
        Write-Host ""
        Write-Host "环境变量: VERSION, PORT, JWT_SECRET, TZ, INTERVAL"
    }
}
