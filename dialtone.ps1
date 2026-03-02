# dialtone.ps1: Simplified orchestrator wrapper for PowerShell.
# Ported from dialtone.sh.

$ErrorActionPreference = "Stop"
$ScriptArgs = @($args)
$env:DIALTONE_USE_NIX = if ([string]::IsNullOrWhiteSpace($env:DIALTONE_USE_NIX)) { "1" } else { $env:DIALTONE_USE_NIX }

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$env:DIALTONE_REPO_ROOT = $ScriptDir
$env:DIALTONE_SRC_ROOT = Join-Path $ScriptDir "src"

function Write-EnvFile {
    param(
        [Parameter(Mandatory = $true)][string]$EnvFilePath,
        [Parameter(Mandatory = $true)][string]$DialtoneEnv
    )
    $envDir = Split-Path -Parent $EnvFilePath
    New-Item -ItemType Directory -Path $envDir -Force | Out-Null
    @(
        "DIALTONE_ENV=$DialtoneEnv"
        "DIALTONE_USE_NIX=1"
    ) | Set-Content -Path $EnvFilePath -Encoding UTF8
}

function Convert-ToWslPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    $p = (Resolve-Path -LiteralPath $Path).Path
    if ($p -match '^([A-Za-z]):\\(.*)$') {
        $drive = $matches[1].ToLower()
        $rest = $matches[2] -replace '\\', '/'
        return "/mnt/$drive/$rest"
    }
    return ($p -replace '\\', '/')
}

function Escape-BashArg {
    param([Parameter(Mandatory = $true)][string]$Value)
    return "'" + ($Value -replace "'", "'\"'\"'") + "'"
}

function Enter-NixShellIfNeeded {
    if ($env:DIALTONE_USE_NIX -in @("0","false","False","no","off")) { return }
    if (![string]::IsNullOrWhiteSpace($env:IN_NIX_SHELL)) { return }
    if ($env:DIALTONE_NIX_SHELL_BOOTSTRAPPED -eq "1") { return }

    $flakePath = Join-Path $ScriptDir "flake.nix"
    if (!(Test-Path $flakePath)) { return }

    if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
        throw "WSL is required for Nix-first workflow on Windows. Install WSL and rerun."
    }

    $wslRepo = Convert-ToWslPath -Path $ScriptDir
    $escapedArgs = @()
    foreach ($a in $ScriptArgs) { $escapedArgs += (Escape-BashArg -Value $a) }
    $argTail = [string]::Join(" ", $escapedArgs)
    $cmd = "cd $(Escape-BashArg -Value $wslRepo) && export DIALTONE_NIX_SHELL_BOOTSTRAPPED=1 && ./dialtone.sh $argTail"

    Write-Host "DIALTONE> Entering WSL + Nix shell..."
    & wsl.exe -e bash -lc $cmd
    exit $LASTEXITCODE
}

function Bootstrap-CloneRepoInPlace {
    param(
        [Parameter(Mandatory = $true)][string]$RepoUrl,
        [Parameter(Mandatory = $true)][string]$Branch
    )
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        throw "Git is required to bootstrap the repository."
    }

    $ps1Path = Join-Path $ScriptDir "dialtone.ps1"
    if (Test-Path $ps1Path) {
        Copy-Item $ps1Path "$ps1Path.back" -Force
        Write-Host "DIALTONE> Backed up launcher: $ps1Path.back"
    }

    if (-not (Test-Path (Join-Path $ScriptDir ".git"))) {
        & git -C $ScriptDir init | Out-Null
        & git -C $ScriptDir remote add origin $RepoUrl
    } else {
        & git -C $ScriptDir remote get-url origin *> $null
        if ($LASTEXITCODE -ne 0) {
            & git -C $ScriptDir remote add origin $RepoUrl
        }
    }

    & git -C $ScriptDir fetch --depth 1 origin $Branch
    if ($LASTEXITCODE -ne 0) { throw "git fetch failed" }
    & git -C $ScriptDir checkout -f -B $Branch FETCH_HEAD
    if ($LASTEXITCODE -ne 0) { throw "git checkout failed" }
}

function Run-BootstrapRepl {
    param(
        [Parameter(Mandatory = $true)][string]$EnvFilePath
    )
    $defaultEnv = Join-Path $ScriptDir ".dialtone_env"
    $defaultRepo = "https://github.com/timcash/dialtone.git"
    $defaultBranch = "main"

    Write-Host "DIALTONE> Bootstrap REPL started."
    Write-Host "DIALTONE> This will configure env/.env and bootstrap the dialtone repo."
    Write-Host "DIALTONE> Runtime/tooling install happens in WSL + Nix shell after bootstrap."

    $inputEnv = Read-Host "DIALTONE> Environment directory [default: $defaultEnv]"
    if ([string]::IsNullOrWhiteSpace($inputEnv)) { $inputEnv = $defaultEnv }
    if ($inputEnv.StartsWith("~")) {
        $inputEnv = Join-Path $env:USERPROFILE $inputEnv.Substring(1).TrimStart("\/")
    }
    New-Item -ItemType Directory -Path $inputEnv -Force | Out-Null
    $env:DIALTONE_ENV = $inputEnv

    Write-EnvFile -EnvFilePath $EnvFilePath -DialtoneEnv $env:DIALTONE_ENV
    Write-Host "DIALTONE> Wrote $EnvFilePath"

    $repoInput = Read-Host "DIALTONE> Git repo to bootstrap [default: $defaultRepo]"
    if ([string]::IsNullOrWhiteSpace($repoInput)) { $repoInput = $defaultRepo }
    $branchInput = Read-Host "DIALTONE> Branch [default: $defaultBranch]"
    if ([string]::IsNullOrWhiteSpace($branchInput)) { $branchInput = $defaultBranch }

    Write-Host "DIALTONE> Bootstrapping repo in $ScriptDir ..."
    Bootstrap-CloneRepoInPlace -RepoUrl $repoInput -Branch $branchInput
    Write-Host "DIALTONE> Repo bootstrap complete."
    Write-Host "DIALTONE> Launching new dialtone runtime..."

    $env:DIALTONE_BOOTSTRAP_DONE = "1"
    & (Join-Path $ScriptDir "dialtone.ps1") @Script:ScriptArgs
    exit $LASTEXITCODE
}

# 1. Load Environment
$EnvFile = Join-Path $ScriptDir "env/.env"
if ([string]::IsNullOrWhiteSpace($env:DIALTONE_ENV_FILE)) {
    $env:DIALTONE_ENV_FILE = $EnvFile
}
if (!(Test-Path $EnvFile) -and [string]::IsNullOrWhiteSpace($env:DIALTONE_BOOTSTRAP_DONE)) {
    Run-BootstrapRepl -EnvFilePath $EnvFile
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

Enter-NixShellIfNeeded

# Default DIALTONE_ENV if not set
if ([string]::IsNullOrWhiteSpace($env:DIALTONE_ENV)) {
    $env:DIALTONE_ENV = Join-Path $ScriptDir ".dialtone_env"
}

# Expand ~ in DIALTONE_ENV
if ($env:DIALTONE_ENV.StartsWith("~")) {
    $env:DIALTONE_ENV = Join-Path $env:USERPROFILE $env:DIALTONE_ENV.Substring(1).TrimStart("\/")
}

$env:GOROOT = Join-Path $env:DIALTONE_ENV "go"
$GoBin = Join-Path $env:GOROOT "bin/go.exe"
$BunBin = Join-Path $env:DIALTONE_ENV "bun/bin/bun.exe"

# Optional global log mirror: pass --stdout anywhere to mirror logs to stdout
$PassThruArgs = New-Object System.Collections.Generic.List[string]
foreach ($arg in $args) {
    if ($arg -eq "--stdout") {
        $env:DIALTONE_LOG_STDOUT = "1"
        continue
    }
    $PassThruArgs.Add($arg)
}

# 2. Resolve Go
if (!(Test-Path $GoBin)) {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
        $GoBin = $goCmd.Source
    } else {
        Write-Host "DIALTONE> Go runtime missing and 'go' is not on PATH."
        Write-Host "DIALTONE> Enable Nix mode (DIALTONE_USE_NIX=1) or install managed Go into DIALTONE_ENV."
        exit 1
    }
}

# 3. Setup PATH and GOROOT
if (Test-Path $BunBin) {
    $env:PATH = "$(Join-Path $env:DIALTONE_ENV 'go/bin');$(Join-Path $env:DIALTONE_ENV 'bun/bin');$env:PATH"
    $env:DIALTONE_BUN_BIN = $BunBin
} else {
    $managedGoBin = Join-Path $env:DIALTONE_ENV "go/bin"
    if (Test-Path $managedGoBin) {
        $env:PATH = "$managedGoBin;$env:PATH"
    }
}
$env:DIALTONE_GO_BIN = $GoBin

# 4. Hand over to Go-based orchestrator
Push-Location -Path $env:DIALTONE_SRC_ROOT
try {
    & "$GoBin" run dev.go $PassThruArgs
    $code = $LASTEXITCODE
}
finally {
    Pop-Location
}
exit $code
