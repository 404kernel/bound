# Bound Windows Installer (PowerShell)
# Usage: irm https://raw.githubusercontent.com/404kernel/bound/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "404kernel/bound"
$installDir = "$HOME\.bound\bin"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  BOUND // Programmable Spend Vault for Autonomous AI Agents" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

Write-Host "Fetching latest release tag..."
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "PowerShell" }
    $tag = $release.tag_name
} catch {
    $tag = "v0.2.1"
}

$binaryName = "bound-windows-amd64.exe"
$downloadUrl = "https://github.com/$repo/releases/download/$tag/$binaryName"

if (!(Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$destPath = Join-Path $installDir "bound.exe"
Write-Host "Downloading Bound $tag to $destPath..." -ForegroundColor Green
Invoke-WebRequest -Uri $downloadUrl -OutFile $destPath -UseBasicParsing

# Add to User PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
if ($userPath -notlike "*$installDir*") {
    Write-Host "Adding $installDir to User PATH..." -ForegroundColor Yellow
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", [EnvironmentVariableTarget]::User)
    $env:Path += ";$installDir"
}

Write-Host "`nBound $tag installed successfully!" -ForegroundColor Green
Write-Host "Open a new terminal and run 'bound help' or 'bound proxy'." -ForegroundColor Cyan
