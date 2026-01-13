# build_and_deploy.ps1

Write-Host "Starting Build and Deploy Process for Dialtone..." -ForegroundColor Cyan

# 1. Build Web UI
Write-Host "Building Web UI..." -ForegroundColor Yellow
Set-Location web
npm install
npm run build
Set-Location ..
if ($LASTEXITCODE -ne 0) {
    Write-Host "Web UI build failed" -ForegroundColor Red
    exit $LASTEXITCODE
}

# Copy web files to src/web for embedding
Remove-Item -Recurse -Force src/web -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path src/web -Force
Copy-Item -Path web/dist/* -Destination src/web -Recurse

# 1b. Build Dialtone for ARM64 using Podman
Write-Host "Building Dialtone for ARM64 using Podman..." -ForegroundColor Yellow
# Ensure ssh_tools is compiled first as bin\ssh_tools.exe -podman-build uses it
go build -o bin/ssh_tools.exe src/ssh_tools.go
bin\ssh_tools.exe -podman-build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Podman build failed" -ForegroundColor Red
    exit $LASTEXITCODE
}

# 2. Execute Deployment
Write-Host "Deploying to Raspberry Pi..." -ForegroundColor Yellow
bin\ssh_tools.exe -host tim@192.168.4.36 -pass password -deploy
if ($LASTEXITCODE -ne 0) {
    Write-Host "Deployment failed" -ForegroundColor Red
    exit $LASTEXITCODE
}

# 3. Run Verification Tests
Write-Host "Running Verification Tests..." -ForegroundColor Yellow
go test -v ./src/...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "Build and Deploy Successful!" -ForegroundColor Green
