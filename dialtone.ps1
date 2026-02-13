<#
.SYNOPSIS
  Dialtone CLI Wrapper (PowerShell)
.DESCRIPTION
  Windows/PowerShell wrapper aligned with dialtone.sh:
  - env loading/verification
  - explicit process management (ps/proc/kill)
  - Go-only install handoff
  - forwards all other commands to src/cmd/dev/main.go using managed Go
#>

[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)]
    [string]$Command = "",

    [Parameter()]
    [string]$EnvFile = "env/.env",

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

$RuntimeDir = Join-Path $ScriptDir ".dialtone/run"
$GracefulTimeout = 5
if ($env:GRACEFUL_TIMEOUT -and ($env:GRACEFUL_TIMEOUT -as [int])) {
    $GracefulTimeout = [int]$env:GRACEFUL_TIMEOUT
}

$depth = 0
if ($env:DIALTONE_WRAPPER_DEPTH -and ($env:DIALTONE_WRAPPER_DEPTH -as [int])) {
    $depth = [int]$env:DIALTONE_WRAPPER_DEPTH
}
$depth += 1
$env:DIALTONE_WRAPPER_DEPTH = "$depth"

function Show-Help {
    $helpPath = Join-Path $ScriptDir "help.txt"
    if (Test-Path $helpPath) {
        Get-Content $helpPath | Write-Host
        return
    }
    Write-Host "Usage: .\dialtone.ps1 <command> [options]"
    Write-Host "Run '.\dialtone.ps1 install' to install managed Go."
}

function Show-PsHelp {
    @"
Usage: .\dialtone.ps1 ps <option>

Options:
  all                 Show all running dialtone wrapper processes (default)
  tracked             Show tracked processes from .dialtone/run
  tree                Show process tree-style rows
  help                Show this help
"@ | Write-Host
}

function Show-ProcHelp {
    @"
Usage: .\dialtone.ps1 proc <subcommand>

Subcommands:
  ps                  List tracked processes
  stop <key>          Stop tracked process by key
  logs <key>          Tail tracked log for key
"@ | Write-Host
}

function Show-KillHelp {
    @"
Usage: .\dialtone.ps1 kill <pid|all>

Commands:
  kill <pid>          Kill PID and descendants
  kill all            Kill all dialtone wrapper process trees
  kill help           Show this help
"@ | Write-Host
}

function Load-EnvFile([string]$Path) {
    if (!(Test-Path $Path)) { return }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if (!$line -or $line.StartsWith("#")) { return }
        $parts = $line.Split("=", 2)
        if ($parts.Length -eq 2) {
            [System.Environment]::SetEnvironmentVariable($parts[0], $parts[1], "Process")
        }
    }
}

function Ensure-Env {
    if (-not $env:DIALTONE_ENV) {
        Write-Host "Error: DIALTONE_ENV is not set." -ForegroundColor Red
        Write-Host "Set it in $EnvFile or pass -EnvFile <path>."
        exit 1
    }
}

function Get-ProcessKey([string]$Raw) {
    $key = $Raw -replace '[\\/]', '_' -replace '\s+', '_' -replace '[^A-Za-z0-9_.-]', ''
    $key = $key.Trim('_')
    if ([string]::IsNullOrWhiteSpace($key)) { $key = "unnamed" }
    if ([int]$env:DIALTONE_WRAPPER_DEPTH -gt 1) {
        $key = "${key}__pid_$PID"
    }
    return $key
}

function Ensure-RuntimeDir {
    if (!(Test-Path $RuntimeDir)) {
        New-Item -ItemType Directory -Force -Path $RuntimeDir | Out-Null
    }
}

function Get-ProcessDescendants([int]$ParentPid) {
    $all = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId
    $children = @()
    $queue = New-Object System.Collections.Generic.Queue[int]
    $queue.Enqueue($ParentPid)
    while ($queue.Count -gt 0) {
        $p = $queue.Dequeue()
        foreach ($row in $all) {
            if ($row.ParentProcessId -eq $p) {
                $children += [int]$row.ProcessId
                $queue.Enqueue([int]$row.ProcessId)
            }
        }
    }
    return $children
}

function Stop-Tree([int]$Pid) {
    $targets = @($Pid) + (Get-ProcessDescendants -ParentPid $Pid)
    foreach ($t in $targets) {
        try { Stop-Process -Id $t -ErrorAction SilentlyContinue } catch {}
    }
    Start-Sleep -Seconds $GracefulTimeout
    foreach ($t in $targets) {
        try { Stop-Process -Id $t -Force -ErrorAction SilentlyContinue } catch {}
    }
}

function Get-Tracked {
    Ensure-RuntimeDir
    $files = Get-ChildItem -Path $RuntimeDir -Filter *.pid -ErrorAction SilentlyContinue
    $rows = @()
    foreach ($f in $files) {
        $key = [IO.Path]::GetFileNameWithoutExtension($f.Name)
        $pid = (Get-Content $f.FullName -ErrorAction SilentlyContinue | Select-Object -First 1)
        $meta = Join-Path $RuntimeDir "$key.meta"
        $cmd = "(unknown)"
        if (Test-Path $meta) {
            $cmdLine = Get-Content $meta | Where-Object { $_ -like 'CMD=*' } | Select-Object -First 1
            if ($cmdLine) { $cmd = $cmdLine.Substring(4) }
        }
        $running = $false
        if ($pid -and ($pid -as [int])) {
            $running = [bool](Get-Process -Id ([int]$pid) -ErrorAction SilentlyContinue)
        }
        if (-not $running) {
            Remove-Item -Force $f.FullName -ErrorAction SilentlyContinue
            Remove-Item -Force $meta -ErrorAction SilentlyContinue
            continue
        }
        $rows += [PSCustomObject]@{ Key = $key; PID = [int]$pid; Status = "running"; Cmd = $cmd }
    }
    return $rows
}

function Cmd-ProcPs {
    $rows = Get-Tracked
    if ($rows.Count -eq 0) {
        Write-Host "No tracked processes."
        return
    }
    $rows | Format-Table -AutoSize Key, PID, Status, Cmd
}

function Cmd-ProcStop([string]$Key) {
    if (-not $Key) {
        Write-Host "Usage: .\dialtone.ps1 proc stop <key>" -ForegroundColor Red
        exit 1
    }
    $pidFile = Join-Path $RuntimeDir "$Key.pid"
    $metaFile = Join-Path $RuntimeDir "$Key.meta"
    if (!(Test-Path $pidFile)) {
        Write-Host "No tracked process for key: $Key" -ForegroundColor Yellow
        return
    }
    $pid = (Get-Content $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1)
    if ($pid -and ($pid -as [int])) {
        Stop-Tree -Pid ([int]$pid)
    }
    Remove-Item -Force $pidFile -ErrorAction SilentlyContinue
    Remove-Item -Force $metaFile -ErrorAction SilentlyContinue
}

function Cmd-ProcLogs([string]$Key) {
    if (-not $Key) {
        Write-Host "Usage: .\dialtone.ps1 proc logs <key>" -ForegroundColor Red
        exit 1
    }
    $log = Join-Path $RuntimeDir "$Key.log"
    if (!(Test-Path $log)) {
        Write-Host "No log file found for key: $Key" -ForegroundColor Yellow
        exit 1
    }
    Get-Content -Path $log -Tail 50 -Wait
}

function Cmd-PsAll {
    Get-CimInstance Win32_Process |
        Where-Object { $_.CommandLine -match 'dialtone\.ps1|dialtone\.cmd' } |
        Select-Object ProcessId, ParentProcessId, Name, CommandLine |
        Format-Table -AutoSize
}

function Cmd-PsTree {
    Cmd-PsAll
}

function Cmd-KillPid([string]$PidText) {
    if (-not ($PidText -as [int])) {
        Write-Host "Invalid PID: $PidText" -ForegroundColor Red
        exit 1
    }
    Stop-Tree -Pid ([int]$PidText)
}

function Cmd-KillAll {
    $self = $PID
    $rows = Get-CimInstance Win32_Process |
        Where-Object { $_.CommandLine -match 'dialtone\.ps1|dialtone\.cmd' -and $_.ProcessId -ne $self }
    foreach ($row in $rows) {
        Stop-Tree -Pid ([int]$row.ProcessId)
    }
}

function Forward-Go([string]$Cmd, [string[]]$ForwardArgs) {
    Ensure-Env
    $goBin = Join-Path $env:DIALTONE_ENV "go/bin/go.exe"
    if (!(Test-Path $goBin)) {
        Write-Host "Error: Go not found in $($env:DIALTONE_ENV)/go." -ForegroundColor Red
        Write-Host "Run .\dialtone.ps1 install first."
        exit 1
    }

    Ensure-RuntimeDir
    $key = Get-ProcessKey "$Cmd $($ForwardArgs -join ' ')"
    $pidFile = Join-Path $RuntimeDir "$key.pid"
    $metaFile = Join-Path $RuntimeDir "$key.meta"
    $logFile = Join-Path $RuntimeDir "$key.log"

    $goArgs = @("run", "src/cmd/dev/main.go", $Cmd) + $ForwardArgs

    @(
        "CMD=.\dialtone.ps1 $Cmd $($ForwardArgs -join ' ')"
        "LOG=$logFile"
        "STARTED_AT=$([DateTime]::UtcNow.ToString('s'))Z"
    ) | Set-Content -Path $metaFile

    # Use redirected files, then stream appended content to terminal while process runs.
    $stdoutFile = "$logFile.stdout"
    $stderrFile = "$logFile.stderr"
    Remove-Item -Force $stdoutFile, $stderrFile -ErrorAction SilentlyContinue

    $proc = Start-Process -FilePath $goBin -ArgumentList $goArgs -WorkingDirectory $ScriptDir -PassThru -NoNewWindow -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile
    "$($proc.Id)" | Set-Content -Path $pidFile

    $outOffset = 0L
    $errOffset = 0L

    try {
        while (-not $proc.HasExited) {
            foreach ($pair in @(@($stdoutFile, [ref]$outOffset), @($stderrFile, [ref]$errOffset))) {
                $file = $pair[0]
                $offRef = $pair[1]
                if (Test-Path $file) {
                    $fs = [System.IO.File]::Open($file, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
                    try {
                        if ($fs.Length -gt $offRef.Value) {
                            $fs.Seek($offRef.Value, [System.IO.SeekOrigin]::Begin) | Out-Null
                            $sr = New-Object System.IO.StreamReader($fs)
                            $chunk = $sr.ReadToEnd()
                            $offRef.Value = $fs.Position
                            if ($chunk) {
                                $chunk | Add-Content -Path $logFile
                                Write-Host -NoNewline $chunk
                            }
                        }
                    }
                    finally {
                        $fs.Close()
                    }
                }
            }
            Start-Sleep -Milliseconds 200
        }

        # Final drain
        foreach ($pair in @(@($stdoutFile, [ref]$outOffset), @($stderrFile, [ref]$errOffset))) {
            $file = $pair[0]
            $offRef = $pair[1]
            if (Test-Path $file) {
                $fs = [System.IO.File]::Open($file, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
                try {
                    if ($fs.Length -gt $offRef.Value) {
                        $fs.Seek($offRef.Value, [System.IO.SeekOrigin]::Begin) | Out-Null
                        $sr = New-Object System.IO.StreamReader($fs)
                        $chunk = $sr.ReadToEnd()
                        $offRef.Value = $fs.Position
                        if ($chunk) {
                            $chunk | Add-Content -Path $logFile
                            Write-Host -NoNewline $chunk
                        }
                    }
                }
                finally {
                    $fs.Close()
                }
            }
        }
    }
    finally {
        Remove-Item -Force $pidFile, $metaFile -ErrorAction SilentlyContinue
        Remove-Item -Force $stdoutFile, $stderrFile -ErrorAction SilentlyContinue
    }

    exit $proc.ExitCode
}

# Ensure dist directory exists for embed expectations.
if (!(Test-Path "src/core/web/dist")) {
    New-Item -ItemType Directory -Force -Path "src/core/web/dist" | Out-Null
}

# Load env early.
Load-EnvFile -Path $EnvFile

if (-not $Command -or $Command -in @("help", "-h", "--help")) {
    Show-Help
    exit 0
}

switch ($Command) {
    "ps" {
        $opt = if ($ExtraArgs.Count -gt 0) { $ExtraArgs[0] } else { "all" }
        switch ($opt) {
            "all" { Cmd-PsAll; exit 0 }
            "tracked" { Cmd-ProcPs; exit 0 }
            "tree" { Cmd-PsTree; exit 0 }
            "help" { Show-PsHelp; exit 0 }
            default {
                Write-Host "Unknown ps option: $opt" -ForegroundColor Red
                Show-PsHelp
                exit 1
            }
        }
    }
    "proc" {
        $sub = if ($ExtraArgs.Count -gt 0) { $ExtraArgs[0] } else { "help" }
        switch ($sub) {
            "ps" { Cmd-ProcPs; exit 0 }
            "stop" { Cmd-ProcStop -Key ($ExtraArgs | Select-Object -Skip 1 -First 1); exit 0 }
            "logs" { Cmd-ProcLogs -Key ($ExtraArgs | Select-Object -Skip 1 -First 1); exit 0 }
            "help" { Show-ProcHelp; exit 0 }
            default {
                Write-Host "Unknown proc command: $sub" -ForegroundColor Red
                Show-ProcHelp
                exit 1
            }
        }
    }
    "kill" {
        $target = if ($ExtraArgs.Count -gt 0) { $ExtraArgs[0] } else { "help" }
        switch ($target) {
            "all" { Cmd-KillAll; exit 0 }
            "help" { Show-KillHelp; exit 0 }
            default { Cmd-KillPid -PidText $target; exit 0 }
        }
    }
    "install" {
        $installer = Join-Path $ScriptDir "src/plugins/go/install.ps1"
        if (!(Test-Path $installer)) {
            Write-Host "Error: installer not found: $installer" -ForegroundColor Red
            exit 1
        }
        & $installer @ExtraArgs
        exit $LASTEXITCODE
    }
    default {
        Forward-Go -Cmd $Command -ForwardArgs $ExtraArgs
    }
}
