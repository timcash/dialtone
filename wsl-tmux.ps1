[CmdletBinding(PositionalBinding = $false)]
param(
  [Parameter(Position = 0)]
  [string]$Action,
  [string]$Session = "windows",
  [string]$Distro,
  [string]$Cwd = "/home/user/dialtone",
  [int]$Lines = 120,
  [int]$Width = 120,
  [int]$Height = 40,
  [Parameter(Position = 1, ValueFromRemainingArguments = $true)]
  [string[]]$CommandArgs
)

$ErrorActionPreference = "Stop"
$knownActions = @("ensure", "send", "read", "clear", "interrupt", "list")

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
cmd_text=`$(printf '%s' "`$cmd_b64" | base64 -d)
tmux send-keys -t '$safeSession' C-c
tmux send-keys -l -t '$safeSession' "`$cmd_text"
tmux send-keys -t '$safeSession' Enter
sleep 0.5
tmux capture-pane -pt '$safeSession' -S -$Lines
"@
  }
}
