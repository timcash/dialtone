[CmdletBinding(PositionalBinding = $false)]
param(
  [Parameter(Position = 0)]
  [ValidateSet("ensure", "send", "read", "clear", "interrupt", "list")]
  [string]$Action = "send",
  [string]$Session = "codex",
  [string]$Distro,
  [string]$Cwd = "/home/user/dialtone",
  [int]$Lines = 120,
  [int]$Width = 120,
  [int]$Height = 40,
  [Parameter(Position = 1, ValueFromRemainingArguments = $true)]
  [string[]]$CommandArgs
)

$ErrorActionPreference = "Stop"

function Invoke-WslBash {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Script
  )

  $wslArgs = @()
  if ($Distro) {
    $wslArgs += "-d"
    $wslArgs += $Distro
  }
  $wslArgs += "bash"
  $wslArgs += "-s"

  $Script | & wsl.exe @wslArgs
  if ($LASTEXITCODE -ne 0) {
    throw "wsl command failed with exit code $LASTEXITCODE"
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
  "ensure" {
    Ensure-TmuxSession
  }
  "list" {
    Invoke-WslBash "tmux list-sessions"
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
sleep 0.2
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
  "interrupt" {
    Ensure-TmuxSession
    $safeSession = ConvertTo-BashSingleQuoted $Session
    Invoke-WslBash @"
tmux send-keys -t '$safeSession' C-c
sleep 0.2
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
cmd_text=\$(printf '%s' "\$cmd_b64" | base64 -d)
tmux send-keys -t '$safeSession' C-c
tmux send-keys -l -t '$safeSession' "\$cmd_text"
tmux send-keys -t '$safeSession' Enter
sleep 0.5
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
}
