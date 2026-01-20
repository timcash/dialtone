if (-not $env:DIALTONE_ENV) {
    $env:DIALTONE_ENV = "$HOME\.dialtone_env"
}

$GO_VERSION = "1.25.5"
$INSTALL_DIR = "$HOME\.local\go"
$GO_BIN = "$INSTALL_DIR\bin\go.exe"

# Function to check if go is available
function Test-GoCommand {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        return $true
    }
    if (Test-Path $GO_BIN) {
        $env:PATH = "$INSTALL_DIR\bin;$env:PATH"
        return $true
    }
    return $false
}

if (-not (Test-GoCommand)) {
    Write-Host "Go not found. Installing Go $GO_VERSION to $INSTALL_DIR..."
    
    if (-not (Test-Path "$HOME\.local")) {
        New-Item -Path "$HOME\.local" -ItemType Directory | Out-Null
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
    Expand-Archive -Path $TAR_FILE -DestinationPath "$HOME\.local" -Force
    
    Remove-Item $TAR_FILE
    
    $env:PATH = "$INSTALL_DIR\bin;$env:PATH"
}

# Run the dialtone-dev tool
# We pass all arguments to the go run command
# Note: In PowerShell, parsing $args and passing them correctly can be tricky, 
# but simply listing them usually works for simple cases.
go run dialtone-dev.go $args
