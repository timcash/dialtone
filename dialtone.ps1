# dialtone.ps1: Simplified orchestrator wrapper for PowerShell.
# Ported from dialtone.sh.

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$env:DIALTONE_REPO_ROOT = $ScriptDir
$env:DIALTONE_SRC_ROOT = Join-Path $ScriptDir "src"

# 1. Load Environment
$EnvFile = Join-Path $ScriptDir "env/.env"
if ([string]::IsNullOrWhiteSpace($env:DIALTONE_ENV_FILE)) {
    $env:DIALTONE_ENV_FILE = $EnvFile
}
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        $line = $_.Trim()
        if (!$line -or $line.StartsWith("#")) { return }
        $parts = $line.Split("=", 2)
        if ($parts.Length -eq 2) {
            $key = $parts[0].Trim()
            $value = $parts[1].Trim()
            # Basic env expansion (optional but helpful)
            [System.Environment]::SetEnvironmentVariable($key, $value, "Process")
        }
    }
}

# Default DIALTONE_ENV if not set
if ([string]::IsNullOrWhiteSpace($env:DIALTONE_ENV)) {
    $env:DIALTONE_ENV = Join-Path $env:USERPROFILE ".dialtone_env"
}

# Expand ~ in DIALTONE_ENV
if ($env:DIALTONE_ENV.StartsWith("~")) {
    $env:DIALTONE_ENV = Join-Path $env:USERPROFILE $env:DIALTONE_ENV.Substring(1).TrimStart("\/")
}

$env:GOROOT = Join-Path $env:DIALTONE_ENV "go"
$GoBin = Join-Path $env:GOROOT "bin/go.exe"
$BunBin = Join-Path $env:DIALTONE_ENV "bun/bin/bun.exe"

# 2. Check for Go
if (!(Test-Path $GoBin)) {
    Write-Host "DIALTONE> Go runtime missing at $env:GOROOT"
    $confirm = Read-Host "DIALTONE> Would you like to install it? [y/N]"
    if ($confirm -match "^[Yy]$") {
        Write-Host "DIALTONE> Installing Go..."
        $installScript = Join-Path $ScriptDir "src/plugins/go/install.sh"
        if (!(Test-Path $installScript)) {
            Write-Host "Error: Installer not found: $installScript" -ForegroundColor Red
            exit 1
        }
        # Run via bash if available (often Git Bash is in PATH on Windows dev boxes)
        bash "$installScript" "$env:DIALTONE_ENV"
    } else {
        Write-Host "DIALTONE> Go is required. Exiting."
        exit 1
    }
}

# 3. Setup PATH and GOROOT
if (Test-Path $BunBin) {
    $env:PATH = "$(Join-Path $env:DIALTONE_ENV 'go/bin');$(Join-Path $env:DIALTONE_ENV 'bun/bin');$env:PATH"
    $env:DIALTONE_BUN_BIN = $BunBin
} else {
    $env:PATH = "$(Join-Path $env:DIALTONE_ENV 'go/bin');$env:PATH"
}
$env:DIALTONE_GO_BIN = $GoBin

# 4. Hand over to Go-based orchestrator
Set-Location -Path $env:DIALTONE_SRC_ROOT
& "$GoBin" run dev.go $args
exit $LASTEXITCODE
