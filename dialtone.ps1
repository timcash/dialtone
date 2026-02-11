<#
.SYNOPSIS
    Dialtone CLI Wrapper (PowerShell)
.DESCRIPTION
    Replicates dialtone.sh functionality for Windows environments.
    Handles environment variables, Go installation, and command dispatching.
#>

[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)]
    [string]$Command = "",
    
    [Parameter()]
    [string]$EnvFile = "env/.env",
    
    [Parameter()]
    [switch]$Clean,
    
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ExtraArgs
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$CurrentDir = (Get-Location).Path
if ($CurrentDir -ne $ScriptDir) {
    Write-Host "Error: .\dialtone.ps1 must be run from the repository root." -ForegroundColor Red
    Write-Host "Expected: $ScriptDir"
    Write-Host "Current:  $CurrentDir"
    Write-Host "Run: cd `"$ScriptDir`"; .\dialtone.ps1 <command>"
    exit 1
}

# --- HELP MENU ---
function Show-Help {
    Write-Host "Usage: .\dialtone.ps1 <command> [options]"
    Write-Host ""
    Write-Host "Commands:"
    Write-Host "  start         Start the NATS and Web server"
    Write-Host "  install       Install local Go toolchain"
    Write-Host "  build         Build web UI and binary"
    Write-Host "  task          Task management (create, validate)"
    Write-Host "  ...           (See dialtone.sh for full list)"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -EnvFile <path>    Path to .env file (default: env/.env)"
    Write-Host "  -Clean             Clean dependencies directory"
}

if ($Command -eq "help" -or $Command -eq "-h" -or $Command -eq "--help") {
    Show-Help
    exit 0
}

# --- CONFIGURATION ---
# Ensure dist directory exists
if (!(Test-Path "src/core/web/dist")) {
    New-Item -ItemType Directory -Force -Path "src/core/web/dist" | Out-Null
}

# --- ENV LOADING ---
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and !$line.StartsWith("#")) {
            $parts = $line.Split("=", 2)
            if ($parts.Length -eq 2) {
                [System.Environment]::SetEnvironmentVariable($parts[0], $parts[1], "Process")
            }
        }
    }
}

# Clean dependencies if requested
if ($Clean) {
    if ($env:DIALTONE_ENV -and (Test-Path $env:DIALTONE_ENV)) {
        Write-Host "Cleaning dependencies: $env:DIALTONE_ENV"
        Remove-Item -Recurse -Force $env:DIALTONE_ENV
    }
}

# Default DIALTONE_ENV if not set
if (-not $env:DIALTONE_ENV) {
    # Default to a local 'env' folder if not specified
    # dialtone.sh allows passing it, here we assume sourced from .env or default
    # If not in .env, maybe fail or default? dialtone.sh fails.
    # We'll make it optional for now or default to ./env/deps if needed.
    # For now, let's warn if missing but verify Go availability.
}

# --- GO CONFIGURATION ---
$GoBin = "go"
# If DIALTONE_ENV is set, prefer local go
if ($env:DIALTONE_ENV) {
    # Resolve to absolute path to bypass Windows security restrictions on relative PATH entries
    $env:DIALTONE_ENV = (Resolve-Path $env:DIALTONE_ENV).Path
    $LocalGo = Join-Path $env:DIALTONE_ENV "go/bin/go.exe"
    if (Test-Path $LocalGo) {
        $GoBin = $LocalGo
        $env:GOROOT = Join-Path $env:DIALTONE_ENV "go"
        $env:PATH = "$(Join-Path $env:DIALTONE_ENV 'go/bin');$env:PATH"
    }
}

# --- INSTALL COMMAND ---
if ($Command -eq "install") {
    if (-not $env:DIALTONE_ENV) {
        Write-Host "Error: DIALTONE_ENV not set in $EnvFile" -ForegroundColor Red
        exit 1
    }
    
    if (!(Test-Path $env:DIALTONE_ENV)) {
        New-Item -ItemType Directory -Force -Path $env:DIALTONE_ENV | Out-Null
    }

    $GoDir = Join-Path $env:DIALTONE_ENV "go"
    if (!(Test-Path $GoDir)) {
        Write-Host "Installing Go to $GoDir..."
        # Extract version from go.mod
        $GoVersion = (Select-String -Path "go.mod" -Pattern "^go ").Line.Split(" ")[1]
        $ZipFile = "go$GoVersion.windows-amd64.zip"
        $Url = "https://go.dev/dl/$ZipFile"
        $Dest = Join-Path $env:DIALTONE_ENV $ZipFile
        
        Invoke-WebRequest -Uri $Url -OutFile $Dest
        Expand-Archive -Path $Dest -DestinationPath $env:DIALTONE_ENV -Force
        Remove-Item $Dest
    }
    else {
        Write-Host "Go already installed in $GoDir"
    }
    exit 0
}

# --- RUN COMMAND ---
if (-not $Command) {
    Show-Help
    exit 0
}

# CGO Support (check for gcc)
if (Get-Command "gcc" -ErrorAction SilentlyContinue) {
    $env:CGO_ENABLED = "1"
}
else {
    $env:CGO_ENABLED = "0"
}

# Execute
# We use & operator for the command
$GoArgs = @("run", "src/cmd/dev/main.go", $Command) + $ExtraArgs

# Check if Go is available
if (!(Get-Command $GoBin -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go binary not found. Run '.\dialtone.ps1 install' or install Go globally." -ForegroundColor Red
    exit 1
}

Write-Host "Running: $GoBin $GoArgs" -ForegroundColor DarkGray
& $GoBin $GoArgs
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
