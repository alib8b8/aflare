$ErrorActionPreference = "Stop"

$BINARY_NAME = "llm-box"
$REPO = "alib8b8/llm-box"
$GITCODE_REPO = "llm-box/llm-box"

function Write-Info($msg)    { Write-Host "[INFO] " -ForegroundColor Cyan -NoNewline; Write-Host $msg }
function Write-Success($msg) { Write-Host "[OK] "   -ForegroundColor Green -NoNewline; Write-Host $msg }
function Write-Warn($msg)    { Write-Host "[WARN] " -ForegroundColor Yellow -NoNewline; Write-Host $msg }
function Write-Error($msg)   { Write-Host "[ERROR] " -ForegroundColor Red -NoNewline; Write-Host $msg }

function Detect-Platform {
    $os = "windows"
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    if ([Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE") -eq "ARM64") { $arch = "arm64" }
    return @{ OS = $os; Arch = $arch }
}

function Get-LatestRelease {
    param([string]$Region)
    
    if ($Region -eq "cn") {
        try {
            $api = "https://gitcode.com/api/v5/repos/$GITCODE_REPO/releases/latest"
            $resp = Invoke-RestMethod -Uri $api -TimeoutSec 5 -ErrorAction Stop
            if ($resp.tag_name -match '^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9]+)?$') { return $resp.tag_name }
        } catch {
            Write-Warn "GitCode API failed, trying GitHub..."
        }
    }
    
    $mirrors = @(
        "https://api.github.com/repos/$REPO/releases/latest",
        "https://ghproxy.com/https://api.github.com/repos/$REPO/releases/latest"
    )
    
    foreach ($url in $mirrors) {
        try {
            $resp = Invoke-RestMethod -Uri $url -TimeoutSec 10 -ErrorAction Stop
            if ($resp.tag_name -match '^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9]+)?$') { return $resp.tag_name }
        } catch {
            continue
        }
    }
    return $null
}

function Download-File {
    param([string]$Url, [string]$Output, [string]$Region)
    
    $mirrors = if ($Region -eq "cn") {
        "https://ghproxy.com/", "https://gh.api.99988866.xyz/", ""
    } else { "" }
    
    foreach ($mirror in $mirrors) {
        $finalUrl = $mirror + $Url
        Write-Info "Trying: $($finalUrl.Substring(0, [Math]::Min(80, $finalUrl.Length)))..."
        
        try {
            Invoke-WebRequest -Uri $finalUrl -OutFile $Output -TimeoutSec 120 -UseBasicParsing -ErrorAction Stop
            $size = (Get-Item $Output).Length
            if ($size -gt 1MB) {
                Write-Success "Downloaded ([math]::Round($size/1MB, 1) MB)"
                return $true
            } else {
                Write-Warn "File too small ($size bytes), trying next mirror..."
                Remove-Item $Output -Force -ErrorAction SilentlyContinue
            }
        } catch {
            continue
        }
    }
    return $false
}

function Detect-Region {
    try {
        $resp = Invoke-RestMethod -Uri "https://ipapi.co/country/" -TimeoutSec 3 -ErrorAction Stop
        if ($resp -eq "CN") { return "cn" }
    } catch {}
    return "global"
}

Write-Host ""
Write-Host "╔══════════════════════════════════════════╗"
Write-Host "║         llm-box 安装向导                 ║"
Write-Host "║   AI Workflow Engine Installer          ║"
Write-Host "╚══════════════════════════════════════════╝"
Write-Host ""

$plat = Detect-Platform
Write-Info "检测系统: $($plat.OS) / $($plat.Arch)"

Write-Info "检测网络环境..."
$region = Detect-Region
if ($region -eq "cn") {
    Write-Success "检测到国内网络，将使用镜像加速下载"
} else {
    Write-Info "检测到国际网络环境"
}

Write-Info "获取最新版本..."
$version = Get-LatestRelease -Region $region
if (-not $version) {
    Write-Error "无法获取最新版本信息"
    Write-Host ""
    Write-Host "手动下载地址："
    Write-Host "  GitHub:  https://github.com/$REPO/releases"
    Write-Host "  GitCode: https://gitcode.com/$GITCODE_REPO/-/releases"
    exit 1
}
Write-Success "最新版本: $version"

$archiveName = "$BINARY_NAME-$($plat.OS)-$($plat.Arch).zip"
$downloadUrl = "https://github.com/$REPO/releases/download/$version/$archiveName"
$checksumsUrl = "https://github.com/$REPO/releases/download/$version/checksums.txt"

$tmpDir = Join-Path $env:TEMP "llm-box-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

$archivePath = Join-Path $tmpDir $archiveName
Write-Info "下载 $archiveName..."
if (-not (Download-File -Url $downloadUrl -Output $archivePath -Region $region)) {
    Write-Error "下载失败"
    Write-Host ""
    Write-Host "请尝试手动下载："
    Write-Host "  GitHub:  $downloadUrl"
    Write-Host "  镜像加速: https://ghproxy.com/$downloadUrl"
    Remove-Item $tmpDir -Recurse -Force
    exit 1
}

Write-Info "解压..."
Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

$exePath = Join-Path $tmpDir "$BINARY_NAME.exe"
if (-not (Test-Path $exePath)) {
    Write-Error "解压后未找到 $BINARY_NAME.exe"
    Remove-Item $tmpDir -Recurse -Force
    exit 1
}

$installDir = Join-Path $env:LOCALAPPDATA "Programs\llm-box"
Write-Info "安装到 $installDir..."
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

Copy-Item $exePath (Join-Path $installDir "$BINARY_NAME.exe") -Force

$envPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($envPath -notlike "*$installDir*") {
    Write-Info "添加到用户 PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$envPath;$installDir", "User")
    $env:Path += ";$installDir"
}

Remove-Item $tmpDir -Recurse -Force

Write-Host ""
Write-Host "╔══════════════════════════════════════════╗"
Write-Host "║           安装完成！🎉                   ║"
Write-Host "╚══════════════════════════════════════════╝"
Write-Host ""
Write-Host "快速开始："
Write-Host "  llm-box --help"
Write-Host "  llm-box create `"Summarize today's AI news`""
Write-Host "  llm-box run your-workflow.yaml"
Write-Host ""
Write-Host "更多文档：https://gitcode.com/$GITCODE_REPO"
Write-Host ""
Write-Host "提示：请重新打开终端窗口使 PATH 生效"
Write-Host ""