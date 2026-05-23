$ErrorActionPreference = "Stop"

$InstallDir = "$env:PROGRAMFILES\DevDash"
$DataDir = "$env:PROGRAMDATA\DevDash"
$LogDir = "$env:PROGRAMDATA\DevDash\logs"

function Write-Step($msg) {
    Write-Host "`n[STEP] $msg" -ForegroundColor Blue
}

function Write-Success($msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

function Write-Warn($msg) {
    Write-Host "[WARN] $msg" -ForegroundColor Yellow
}

function Write-Err($msg) {
    Write-Host "[ERROR] $msg" -ForegroundColor Red
}

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  DevDash Windows 生产环境部署脚本" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Write-Step "检查管理员权限"
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Err "请以管理员身份运行此脚本"
    exit 1
}
Write-Success "管理员权限验证通过"

Write-Step "检查依赖"
$missing = @()
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $missing += "Go" }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { $missing += "Node.js" }

if ($missing.Count -gt 0) {
    Write-Warn "缺少依赖: $($missing -join ', ')"
    Write-Host "请先安装缺少的依赖后重试"
    exit 1
}
Write-Success "依赖检查通过"

Write-Step "创建目录"
@($InstallDir, $DataDir, $LogDir) | ForEach-Object {
    if (-not (Test-Path $_)) {
        New-Item -ItemType Directory -Path $_ -Force | Out-Null
    }
}
Write-Success "目录创建完成"

Write-Step "编译后端"
$serverDir = Join-Path $PSScriptRoot "..\server"
if (-not (Test-Path $serverDir)) {
    $serverDir = Join-Path $PSScriptRoot "server"
}

Push-Location $serverDir
$env:CGO_ENABLED = "0"
go build -o "$InstallDir\devdash-server.exe" ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Err "后端编译失败"
    Pop-Location
    exit 1
}
Pop-Location
Write-Success "后端编译完成"

Write-Step "构建前端"
$webDir = Join-Path $PSScriptRoot "..\web"
if (-not (Test-Path $webDir)) {
    $webDir = Join-Path $PSScriptRoot "web"
}

Push-Location $webDir
npm install --production
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Err "前端构建失败"
    Pop-Location
    exit 1
}

$distDir = Join-Location $webDir "dist"
$staticDir = "$DataDir\www"
if (Test-Path $distDir) {
    Copy-Item -Path "$distDir\*" -Destination $staticDir -Recurse -Force
}
Pop-Location
Write-Success "前端构建完成"

Write-Step "生成环境配置"
$jwtSecret = -join ((1..64) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
$envFile = "$DataDir\.env"

$envContent = @"
JWT_SECRET=$jwtSecret
PORT=9090
TZ=Asia/Shanghai
GIN_MODE=release
DB_PATH=$DataDir\devdash.db
LOG_DIR=$LogDir
STATIC_DIR=$DataDir\www
CORS_ORIGINS=http://localhost:9090,http://localhost:3000
"@

Set-Content -Path $envFile -Value $envContent -Encoding UTF8
$acl = Get-Acl $envFile
$acl.SetAccessRuleProtection($true, $false)
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule("Administrators", "FullControl", "Allow")
$acl.AddAccessRule($rule)
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule("SYSTEM", "FullControl", "Allow")
$acl.AddAccessRule($rule)
Set-Acl -Path $envFile -AclObject $acl
Write-Success "环境配置已生成 (JWT_SECRET: $($jwtSecret.Substring(0,16))...)"

Write-Step "注册 Windows 服务"
$svcName = "DevDash"
$svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue

if ($svc) {
    Write-Warn "服务已存在，正在更新..."
    Stop-Service -Name $svcName -Force -ErrorAction SilentlyContinue
    sc.exe delete $svcName | Out-Null
    Start-Sleep -Seconds 2
}

$nssmPath = "$InstallDir\nssm.exe"
if (-not (Test-Path $nssmPath)) {
    Write-Host "下载 NSSM 服务管理器..."
    Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "$env:TEMP\nssm.zip" -UseBasicParsing
    Expand-Archive -Path "$env:TEMP\nssm.zip" -DestinationPath "$env:TEMP\nssm" -Force
    Copy-Item "$env:TEMP\nssm\nssm-2.24\win64\nssm.exe" $nssmPath -Force
}

& $nssmPath install $svcName "$InstallDir\devdash-server.exe"
& $nssmPath set $svcName AppDirectory $DataDir
& $nssmPath set $svcName DisplayName "DevDash Operations Dashboard"
& $nssmPath set $svcName Description "DevDash - Enterprise Operations Monitoring Dashboard"
& $nssmPath set $svcName Start SERVICE_AUTO_START
& $nssmPath set $svcName AppStdout "$LogDir\devdash-stdout.log"
& $nssmPath set $svcName AppStderr "$LogDir\devdash-stderr.log"
& $nssmPath set $svcName AppRotateFiles 1
& $nssmPath set $svcName AppRotateBytes 10485760

& $nssmPath start $svcName
Start-Sleep -Seconds 5

$svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Write-Success "Windows 服务已启动"
} else {
    Write-Warn "服务可能未正常启动，请检查日志: $LogDir"
}

Write-Step "健康检查"
$maxAttempts = 10
$attempt = 1
$healthy = $false

while ($attempt -le $maxAttempts) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:9090/api/v1/health" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
            $healthy = $true
            break
        }
    } catch {
        Write-Warn "等待服务就绪... (尝试 $attempt/$maxAttempts)"
    }
    Start-Sleep -Seconds 2
    $attempt++
}

if ($healthy) {
    Write-Success "健康检查通过"
} else {
    Write-Warn "健康检查超时，请手动验证: http://localhost:9090/api/v1/health"
}

Write-Step "配置防火墙"
$rule = Get-NetFirewallRule -DisplayName "DevDash" -ErrorAction SilentlyContinue
if (-not $rule) {
    New-NetFirewallRule -DisplayName "DevDash" -Direction Inbound -Protocol TCP -LocalPort 9090 -Action Allow | Out-Null
    Write-Success "防火墙规则已添加 (端口 9090)"
} else {
    Write-Success "防火墙规则已存在"
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  DevDash 部署完成！" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  访问地址: http://localhost:9090"
Write-Host "  默认账号: admin / admin123"
Write-Host "  ⚠️  请立即修改默认密码！"
Write-Host ""
Write-Host "  服务管理:"
Write-Host "    启动: Start-Service DevDash"
Write-Host "    停止: Stop-Service DevDash"
Write-Host "    重启: Restart-Service DevDash"
Write-Host "    状态: Get-Service DevDash"
Write-Host ""
Write-Host "  日志目录: $LogDir"
Write-Host "  数据目录: $DataDir"
Write-Host "  配置文件: $DataDir\.env"
Write-Host ""
