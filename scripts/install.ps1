param(
    [switch]$AtomGit,
    [string]$Mirror = "",
    [string]$Home = "",
    [string]$Data = "",
    [string]$Version = "latest",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$AFLARE_NAME = "aflare"
$GITHUB_REPO = "alib8b8/aflare"
$ATOMGIT_REPO = "aflare/aflare"

if ($Home -eq "") { $Home = "$env:USERPROFILE\aflare" }
if ($Data -eq "") { $Data = "$env:USERPROFILE\.aflare" }
if ($Mirror -ne "") { $env:AFLARE_MIRROR = $Mirror }
if ($Version -ne "latest") { $env:AFLARE_TAG = $Version }

Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║           aflare Installer               ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

function Detect-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { return "unknown" }
    }
}

function Get-LatestTag($repoUrl) {
    try {
        $response = Invoke-RestMethod -Uri $repoUrl -UseBasicParsing -TimeoutSec 10
        if ($response.tag_name) { return $response.tag_name }
    } catch {}
    return ""
}

function Download-File($url, $output) {
    Write-Host "[INFO] Downloading: $url" -ForegroundColor Blue
    try {
        Invoke-WebRequest -Uri $url -OutFile $output -UseBasicParsing -ProgressAction SilentlyContinue
        return $true
    } catch {
        Write-Host "[WARN] Download failed: $_" -ForegroundColor Yellow
        return $false
    }
}

function Install-FromSource {
    Write-Host "[INFO] Building from source (Go required)..." -ForegroundColor Blue

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "[ERROR] Go not found. Please install Go 1.21+ from https://go.dev/dl/" -ForegroundColor Red
        exit 1
    }

    $buildDir = Join-Path $env:TEMP "aflare-build-$(Get-Random)"
    New-Item -ItemType Directory -Path $buildDir -Force | Out-Null

    $cloneUrl = if ($AtomGit) {
        "https://atomgit.com/$ATOMGIT_REPO.git"
    } else {
        "https://github.com/$GITHUB_REPO.git"
    }

    Write-Host "[INFO] Cloning source..." -ForegroundColor Blue
    git clone --depth 1 $cloneUrl $buildDir 2>$null

    if (-not (Test-Path $buildDir)) {
        Write-Host "[ERROR] Failed to clone repository" -ForegroundColor Red
        exit 1
    }

    Set-Location $buildDir
    Write-Host "[INFO] Building aflare..." -ForegroundColor Blue
    go build -o "$Home\bin\aflare.exe" ./cmd/aflare

    if (Test-Path "templates") { Copy-Item -Recurse "templates" "$Home\" -Force }
    if (Test-Path "README*") { Copy-Item "README*" "$Home\" -Force }
    if (Test-Path "LICENSE") { Copy-Item "LICENSE" "$Home\" -Force }

    Set-Location $env:USERPROFILE
    Remove-Item -Recurse -Force $buildDir -ErrorAction SilentlyContinue
}

$arch = Detect-Arch
if ($arch -eq "unknown") {
    Write-Host "[ERROR] Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" -ForegroundColor Red
    exit 1
}

Write-Host "[INFO] Detected: windows / $arch" -ForegroundColor Blue
Write-Host "[INFO] Program dir: $Home" -ForegroundColor Blue
Write-Host "[INFO] Data dir: $Data" -ForegroundColor Blue
if ($AtomGit) { Write-Host "[INFO] Mirror: AtomGit (China optimized)" -ForegroundColor Blue }

if ((Test-Path "$Home\bin\aflare.exe") -and -not $Force) {
    Write-Host "[WARN] aflare already installed at $Home" -ForegroundColor Yellow
    Write-Host "[WARN] Use -Force to reinstall" -ForegroundColor Yellow
    exit 0
}

New-Item -ItemType Directory -Path "$Home\bin" -Force | Out-Null
New-Item -ItemType Directory -Path $Data -Force | Out-Null

$tag = if ($Version -ne "latest") { $Version } else { "" }
if ($tag -eq "") {
    $apiUrl = if ($AtomGit) {
        "https://atomgit.com/api/v1/repos/$ATOMGIT_REPO/releases/latest"
    } else {
        "https://api.github.com/repos/$GITHUB_REPO/releases/latest"
    }
    $tag = Get-LatestTag $apiUrl
    if ($tag -eq "") { $tag = "main" }
}

$assetName = "${AFLARE_NAME}_${tag}_windows_${arch}.tar.gz"
$downloadUrl = if ($Mirror -ne "") {
    "$Mirror/$assetName"
} elseif ($AtomGit) {
    "https://atomgit.com/$ATOMGIT_REPO/-/archive/$tag/$assetName"
} else {
    "https://github.com/$GITHUB_REPO/releases/download/$tag/$assetName"
}

$tmpFile = Join-Path $env:TEMP "aflare-$(Get-Random).tar.gz"
$downloaded = Download-File $downloadUrl $tmpFile

if ($downloaded -and (Test-Path $tmpFile)) {
    Write-Host "[OK] Download complete" -ForegroundColor Green
    Write-Host "[INFO] Extracting..." -ForegroundColor Blue

    if (Get-Command tar -ErrorAction SilentlyContinue) {
        tar -xzf $tmpFile -C $Home --strip-components=1 2>$null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[WARN] Extract failed, building from source..." -ForegroundColor Yellow
            Install-FromSource
        }
    } else {
        Write-Host "[WARN] tar not available, building from source..." -ForegroundColor Yellow
        Install-FromSource
    }
    Remove-Item $tmpFile -Force -ErrorAction SilentlyContinue
} else {
    Write-Host "[WARN] Download failed, building from source..." -ForegroundColor Yellow
    Install-FromSource
}

if (-not (Test-Path "$Home\bin\aflare.exe")) {
    Write-Host "[ERROR] Installation failed: aflare.exe not found" -ForegroundColor Red
    exit 1
}

$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$Home\bin*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$Home\bin", "User")
    Write-Host "[OK] Added $Home\bin to user PATH" -ForegroundColor Green
}

[Environment]::SetEnvironmentVariable("AFLARE_HOME", $Home, "User")
[Environment]::SetEnvironmentVariable("AFLARE_DATA", $Data, "User")

Write-Host ""
Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║      aflare installed successfully!      ║" -ForegroundColor Green
Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""
Write-Host "[OK] Program: $Home" -ForegroundColor Green
Write-Host "[OK] Data:    $Data" -ForegroundColor Green
Write-Host ""
Write-Host "[INFO] Quick start (restart your terminal first):" -ForegroundColor Blue
Write-Host "  aflare skills list"
Write-Host "  aflare run <template>"
Write-Host "  aflare init --mcp all"
Write-Host ""
