# build.ps1 - Dialtone Build Script

Write-Host "Starting Build Process..." -ForegroundColor Cyan

# 1. Build Web UI
Write-Host "Building Web UI..." -ForegroundColor Yellow
$webDir = Join-Path "src" "web"
Push-Location $webDir
npm install
npm run build
Pop-Location

# 2. Sync web assets
Write-Host "Syncing web assets to src/web_build..." -ForegroundColor Yellow
$webBuildDir = Join-Path "src" "web_build"
$distDir = Join-Path $webDir "dist"

if (Test-Path $webBuildDir) {
    Remove-Item -Recurse -Force $webBuildDir
}
New-Item -ItemType Directory -Path $webBuildDir -Force | Out-Null
Copy-Item -Path "$distDir\*" -Destination $webBuildDir -Recurse

# 3. Build Dialtone binary
Write-Host "Building Dialtone binary..." -ForegroundColor Yellow
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" -Force | Out-Null
}
go build -o bin/dialtone.exe .

Write-Host "Build successful!" -ForegroundColor Green
