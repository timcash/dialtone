[CmdletBinding(PositionalBinding = $false)]
param(
  [string]$Session = "dialtone",
  [string]$Distro = "",
  [string]$Cwd = "",
  [int]$Lines = 120,
  [int]$WaitMs = 500,
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ForwardArgs
)

$ErrorActionPreference = "Stop"

$dialtone = Join-Path $PSScriptRoot "dialtone.ps1"
$envFile = Join-Path $PSScriptRoot "env/dialtone.json"
if (-not (Test-Path -LiteralPath $dialtone)) {
  throw "dialtone.ps1 not found at $dialtone"
}

function Convert-ToWslPath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $resolved = if (Test-Path -LiteralPath $Path) {
    (Resolve-Path -LiteralPath $Path).Path
  } else {
    $Path
  }
  if ($resolved -match '^([A-Za-z]):\\(.*)$') {
    $drive = $matches[1].ToLowerInvariant()
    $rest = $matches[2] -replace '\\', '/'
    return "/mnt/$drive/$rest"
  }
  return ($resolved -replace '\\', '/')
}

function ConvertTo-BashArg {
  param([Parameter(Mandatory = $true)][string]$Value)

  return "'" + $Value.Replace("'", "'""'""'") + "'"
}

function Resolve-RepoCwd {
  if (-not [string]::IsNullOrWhiteSpace($Cwd)) {
    return $Cwd.Trim()
  }
  if (-not [string]::IsNullOrWhiteSpace($env:DIALTONE_REPO_ROOT) -and $env:DIALTONE_REPO_ROOT.StartsWith("/")) {
    return $env:DIALTONE_REPO_ROOT.Trim()
  }
  if (Test-Path -LiteralPath $envFile) {
    try {
      $config = Get-Content -LiteralPath $envFile -Raw | ConvertFrom-Json
      $repoRoot = [string]$config.DIALTONE_REPO_ROOT
      if (-not [string]::IsNullOrWhiteSpace($repoRoot)) {
        return $repoRoot.Trim()
      }
    } catch {
    }
  }
  return Convert-ToWslPath -Path $PSScriptRoot
}

function Resolve-Distro {
  if (-not [string]::IsNullOrWhiteSpace($Distro)) {
    return $Distro.Trim()
  }
  foreach ($candidate in @(
    $env:DIALTONE_WSL_MOD_DISTRO,
    $env:DIALTONE_WSL_TERMINAL_DISTRO,
    $env:DIALTONE_WSL_DISTRO
  )) {
    if (-not [string]::IsNullOrWhiteSpace($candidate)) {
      return $candidate.Trim()
    }
  }
  if (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
    return ""
  }

  $raw = (& wsl.exe -l -q 2>$null | Out-String) -replace "`0", ""
  $distros = @(
    $raw -split "\r?\n" |
      ForEach-Object { $_.Trim() } |
      Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
  )
  foreach ($preferred in @("Ubuntu-24.04", "Ubuntu", "Ubuntu-22.04")) {
    if ($distros -contains $preferred) {
      return $preferred
    }
  }
  $ubuntu = $distros | Where-Object { $_ -like "Ubuntu*" } | Select-Object -First 1
  if ($ubuntu) {
    return $ubuntu
  }
  return ""
}

function Build-DialtoneModCommand {
  param([Parameter(Mandatory = $true)][string[]]$Args)

  $quotedArgs = @($Args | ForEach-Object { ConvertTo-BashArg -Value $_ })
  if ($quotedArgs.Count -eq 0) {
    return "./dialtone_mod"
  }
  return "./dialtone_mod " + ($quotedArgs -join " ")
}

function Show-Usage {
  $defaultCwd = Resolve-RepoCwd
  $defaultDistro = Resolve-Distro
  @"
Usage:
  .\dialtone_mod.ps1 <mod> <version> <command> [args]
  .\dialtone_mod.ps1 status
  .\dialtone_mod.ps1 read
  .\dialtone_mod.ps1 clear
  .\dialtone_mod.ps1 interrupt

Examples:
  .\dialtone_mod.ps1 db v1 test
  .\dialtone_mod.ps1 db v1 run --benchmark
  .\dialtone_mod.ps1 mod v1 list
  .\dialtone_mod.ps1 status
  .\dialtone_mod.ps1 read

Defaults:
  Session: $Session
  Distro:  $defaultDistro
  Cwd:     $defaultCwd

Behavior:
  - Mod commands are sent into the visible WSL tmux session through dialtone.ps1.
  - The wrapper types ./dialtone_mod ... into tmux so you can watch the exact command run.
  - Control actions (status, read, clear, interrupt, list, ensure, clean-state) target the same session directly.
"@ | Write-Output
}

if ($ForwardArgs.Count -eq 0) {
  Show-Usage
  exit 0
}

$first = $ForwardArgs[0].Trim().ToLowerInvariant()
if ($first -in @("help", "-h", "--help")) {
  Show-Usage
  exit 0
}

$resolvedDistro = Resolve-Distro
$resolvedCwd = Resolve-RepoCwd
$tmuxArgs = New-Object System.Collections.Generic.List[string]
$tmuxArgs.Add("tmux")

foreach ($pair in @(
  @("-Session", $Session),
  @("-Distro", $resolvedDistro),
  @("-Cwd", $resolvedCwd),
  @("-Lines", [string]$Lines),
  @("-WaitMs", [string]$WaitMs)
)) {
  if (-not [string]::IsNullOrWhiteSpace($pair[1])) {
    $tmuxArgs.Add($pair[0])
    $tmuxArgs.Add($pair[1])
  }
}

$tmuxControlActions = @("status", "read", "clear", "interrupt", "list", "ensure", "clean-state")
if ($ForwardArgs.Count -eq 1 -and $tmuxControlActions -contains $first) {
  $tmuxArgs.Add($first)
} else {
  $tmuxArgs.Add("send")
  $tmuxArgs.Add("--")
  $tmuxArgs.Add((Build-DialtoneModCommand -Args $ForwardArgs))
}

& $dialtone @tmuxArgs
exit $LASTEXITCODE
