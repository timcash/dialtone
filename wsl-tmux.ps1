[CmdletBinding(PositionalBinding = $false)]
param(
  [Parameter(Position = 0)]
  [string]$Action,
  [string]$Session = "windows",
  [string]$Distro,
  [string]$Cwd = "",
  [int]$Lines = 120,
  [int]$Width = 120,
  [int]$Height = 40,
  [int]$WaitMs = 500,
  [Parameter(Position = 1, ValueFromRemainingArguments = $true)]
  [string[]]$CommandArgs
)

$ErrorActionPreference = "Stop"
$knownActions = @("help", "ensure", "send", "read", "clear", "interrupt", "list", "status", "clean-state")

function Get-DefaultWslCwd {
  $configPath = Join-Path $PSScriptRoot "env/dialtone.json"
  if (Test-Path -LiteralPath $configPath) {
    try {
      $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
      $repoRoot = [string]$config.DIALTONE_REPO_ROOT
      if (-not [string]::IsNullOrWhiteSpace($repoRoot)) {
        return $repoRoot
      }
    }
    catch {
    }
  }
  return "/home/user/dialtone"
}

if ([string]::IsNullOrWhiteSpace($Cwd)) {
  $Cwd = Get-DefaultWslCwd
}

function Show-Usage {
  @"
Usage:
  wsl-tmux [command...]
  wsl-tmux help
  wsl-tmux read
  wsl-tmux clear
  wsl-tmux clean-state
  wsl-tmux interrupt
  wsl-tmux ensure
  wsl-tmux list
  wsl-tmux status

Defaults:
  Session: $Session
  Cwd:     $Cwd

Behavior:
  - No arguments reads the current pane.
  - An unknown first argument is treated as a command to send.
  - send clears the current shell input line before typing the command.
  - interrupt sends Ctrl-C without killing the tmux session.

Examples:
  wsl-tmux help
  wsl-tmux pwd
  wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test"
  wsl-tmux read
  wsl-tmux status
  wsl-tmux interrupt
  wsl-tmux clean-state

Options:
  -Session <name>   tmux session name
  -Distro <name>    WSL distro override
  -Cwd <path>       tmux session working directory when created
  -Lines <n>        lines to capture from the pane
  -Width <n>        session width when created
  -Height <n>       session height when created
  -WaitMs <n>       milliseconds to wait after send/clear/interrupt
"@ | Write-Output
}

if (-not $Action) {
  if ($CommandArgs -and $CommandArgs.Count -gt 0) {
    $Action = "send"
  }
  else {
    $Action = "read"
  }
}
elseif ($knownActions -notcontains $Action) {
  $CommandArgs = @($Action) + $CommandArgs
  $Action = "send"
}

function Invoke-WslBash {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Script
  )

  $tempFile = Join-Path ([System.IO.Path]::GetTempPath()) ("wsl-tmux-" + [guid]::NewGuid().ToString("N") + ".sh")
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($tempFile, $Script, $utf8NoBom)

  $root = [System.IO.Path]::GetPathRoot($tempFile).TrimEnd('\').TrimEnd(':').ToLowerInvariant()
  $relative = $tempFile.Substring(3).Replace('\', '/')
  $linuxPath = "/mnt/$root/$relative"

  $wslArgs = @()
  if ($Distro) {
    $wslArgs += "-d"
    $wslArgs += $Distro
  }
  $wslArgs += "bash"
  $wslArgs += $linuxPath

  try {
    & wsl.exe @wslArgs
    if ($LASTEXITCODE -ne 0) {
      throw "wsl command failed with exit code $LASTEXITCODE"
    }
  }
  finally {
    Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
  }
}

function ConvertTo-BashSingleQuoted {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Value
  )

  return $Value.Replace("'", "'""'""'")
}

function Ensure-TmuxSession {
  $safeSession = ConvertTo-BashSingleQuoted $Session
  $safeCwd = ConvertTo-BashSingleQuoted $Cwd
  Invoke-WslBash @"
if ! tmux has-session -t '$safeSession' 2>/dev/null; then
  tmux new-session -d -s '$safeSession' -c '$safeCwd' -x $Width -y $Height
fi
"@
}

switch ($Action) {
  "help" {
    Show-Usage
  }
  "ensure" {
    Ensure-TmuxSession
  }
  "list" {
    Invoke-WslBash "tmux list-sessions"
  }
  "status" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash @"
tmux has-session -t '$safeSession' 2>/dev/null
tmux display-message -p -t '$safeSession' 'session=#{session_name} window=#{window_index}:#{window_name} pane=#{pane_index} cwd=#{pane_current_path} pid=#{pane_pid}'
tmux capture-pane -pt '$safeSession' -S -20
"@
  }
  "read" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash "tmux capture-pane -pt '$safeSession' -S -$Lines"
  }
  "clear" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash @"
tmux send-keys -t '$safeSession' C-l
sleep $(('{0:N3}' -f ($WaitMs / 1000)).Replace(',', ''))
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
  "clean-state" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash @"
tmux send-keys -t '$safeSession' C-c
tmux send-keys -t '$safeSession' C-u
tmux send-keys -t '$safeSession' C-l
tmux clear-history -t '$safeSession'
sleep $(('{0:N3}' -f ($WaitMs / 1000)).Replace(',', ''))
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
  "interrupt" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash @"
tmux send-keys -t '$safeSession' C-c
sleep $(('{0:N3}' -f ($WaitMs / 1000)).Replace(',', ''))
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
  "send" {
    if (-not $CommandArgs -or $CommandArgs.Count -eq 0) {
      throw "send requires a command string"
    }

    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    $commandText = ($CommandArgs -join " ").Trim()
    $encodedCommand = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($commandText))
    Invoke-WslBash @"
cmd_b64='$encodedCommand'
cmd_text=`$(printf '%s' "`$cmd_b64" | base64 -d)
tmux send-keys -t '$safeSession' C-u
tmux send-keys -l -t '$safeSession' "`$cmd_text"
tmux send-keys -t '$safeSession' Enter
sleep $(('{0:N3}' -f ($WaitMs / 1000)).Replace(',', ''))
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
}
