#!/usr/bin/env pwsh
# install.ps1 — Install ttab on Windows
# Usage: irm https://raw.githubusercontent.com/dpkay-io/ttab/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "dpkay-io/ttab"
$installDir = Join-Path $HOME ".terminal_tagger\bin"
$binaryName = "ttab.exe"

Write-Host ""
Write-Host "  ttab installer" -ForegroundColor Cyan
Write-Host "  ─────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

# ── Create install directory ──────────────────────────────────────────────────
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Write-Host "  Created $installDir" -ForegroundColor DarkGray
}

# ── Detect architecture ──────────────────────────────────────────────────────
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "ARM64" { "arm64" }
    default { "amd64" }
}
Write-Host "  Architecture: $arch" -ForegroundColor DarkGray

# ── Fetch latest release ─────────────────────────────────────────────────────
Write-Host "  Fetching latest release..." -ForegroundColor DarkGray
$releaseUrl = "https://api.github.com/repos/$repo/releases/latest"

try {
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{
        "User-Agent" = "ttab-installer"
        "Accept"     = "application/vnd.github+json"
    }
} catch {
    Write-Host "  Error: Could not reach GitHub API." -ForegroundColor Red
    Write-Host "  $_" -ForegroundColor DarkRed
    exit 1
}

$assetName = "ttab-windows-$arch.exe"
$asset = $release.assets | Where-Object { $_.name -eq $assetName }

if (-not $asset) {
    Write-Host "  Error: Asset '$assetName' not found in the latest release." -ForegroundColor Red
    Write-Host "  Available assets:" -ForegroundColor Yellow
    $release.assets | ForEach-Object { Write-Host "    - $($_.name)" }
    exit 1
}

# ── Download binary ───────────────────────────────────────────────────────────
$downloadUrl = $asset.browser_download_url
$targetPath = Join-Path $installDir $binaryName

Write-Host "  Downloading $assetName..." -ForegroundColor DarkGray
Invoke-WebRequest -Uri $downloadUrl -OutFile $targetPath -UseBasicParsing

if (-not (Test-Path $targetPath)) {
    Write-Host "  Error: Download failed." -ForegroundColor Red
    exit 1
}

Write-Host "  Saved to $targetPath" -ForegroundColor DarkGray

# ── Add to user PATH ─────────────────────────────────────────────────────────
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
    $env:PATH = "$env:PATH;$installDir"
    Write-Host "  Added to PATH" -ForegroundColor DarkGray
} else {
    Write-Host "  Already in PATH" -ForegroundColor DarkGray
}

# ── Install shell hook ───────────────────────────────────────────────────────
Write-Host "  Installing shell hook..." -ForegroundColor DarkGray
& $targetPath install --profile $PROFILE

Write-Host ""
Write-Host "  ✓ ttab installed successfully!" -ForegroundColor Green
Write-Host "  Restart your terminal to activate." -ForegroundColor Yellow
Write-Host ""
