Write-Host "DevDash Windows 安装脚本" -ForegroundColor Green
Write-Host "======================="

$Version = "1.0.0"
$DownloadUrl = "https://github.com/devdash-dev/devdash/releases/download/v${Version}/devdash-windows-amd64.exe"
$InstallPath = "$env:USERPROFILE\devdash\devdash.exe"

Write-Host "正在下载 DevDash..."
New-Item -ItemType Directory -Force -Path (Split-Path $InstallPath -Parent) | Out-Null
Invoke-WebRequest -Uri $DownloadUrl -OutFile $InstallPath -UseBasicParsing

Write-Host "安装完成！"
Write-Host "路径: $InstallPath"
Write-Host ""
Write-Host "默认端口: 9090"
Write-Host "默认账号: admin / admin123"
Write-Host ""
Write-Host "启动: $InstallPath"
Write-Host "或使用 Docker: docker-compose up -d"