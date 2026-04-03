[CmdletBinding(PositionalBinding = $false)]
param(
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ForwardArgs
)

$ErrorActionPreference = "Stop"
$dialtone = Join-Path $PSScriptRoot "dialtone.ps1"
if (-not (Test-Path -LiteralPath $dialtone)) {
  throw "dialtone.ps1 not found at $dialtone"
}

& $dialtone tmux @ForwardArgs
exit $LASTEXITCODE
