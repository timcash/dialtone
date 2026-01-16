if (-not $env:DIALTONE_ENV) {
    $env:DIALTONE_ENV = "$HOME\.dialtone_env"
    Write-Host "DIALTONE_ENV was not set, defaulting to $env:DIALTONE_ENV"
}

$GO_VERSION = "1.25.5"
$INSTALL_DIR = "$HOME\.local\go"

Write-Host "Installing Go $GO_VERSION to $INSTALL_DIR..."

if (-not (Test-Path "$HOME\.local")) {
    New-Item -Path "$HOME\.local" -ItemType Directory
}

if (Test-Path $INSTALL_DIR) {
    Write-Host "Removing existing Go installation..."
    Remove-Item -Path $INSTALL_DIR -Recurse -Force
}

$TAR_FILE = "go$GO_VERSION.windows-amd64.zip"
$DOWNLOAD_URL = "https://go.dev/dl/$TAR_FILE"

Write-Host "Downloading $DOWNLOAD_URL..."
Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $TAR_FILE

Write-Host "Extracting..."
Expand-Archive -Path $TAR_FILE -DestinationPath "$HOME\.local"

Remove-Item $TAR_FILE

$env:PATH = "$INSTALL_DIR\bin;$env:PATH"

Write-Host "Building Dialtone dev CLI binary..."
if (-not (Test-Path "bin")) {
    New-Item -Path "bin" -ItemType Directory
}

go build -o bin\dialtone-dev.exe dialtone-dev.go

Write-Host "Go $GO_VERSION and dialtone-dev CLI installed successfully."
Write-Host "Please add the following to your environment variables:"
Write-Host "PATH = $INSTALL_DIR\bin;$env:PATH"
Write-Host ""
Write-Host "You can now run:"
Write-Host ".\bin\dialtone-dev.exe --help"
