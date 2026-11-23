#!/usr/bin/env pwsh
# dialtone.ps1: Windows PowerShell wrapper for the dev tasks in xtask/ (.\dialtone.ps1 help).
$ErrorActionPreference = "Stop"

$cargoHome = if ($env:CARGO_HOME) { $env:CARGO_HOME } else { Join-Path $HOME ".cargo" }
$cargoBin = Join-Path $cargoHome "bin"
if (Test-Path $cargoBin) {
    $env:PATH = "$cargoBin;$env:PATH"
}

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $root
try {
    & cargo run -q -p xtask -- @args
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
