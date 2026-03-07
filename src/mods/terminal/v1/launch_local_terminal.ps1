function Start-DialtoneLocalTerminal {
    param(
        [Parameter(Mandatory = $true)]
        [string]$WslPath,
        [Parameter(Mandatory = $true)]
        [string]$CommandText,
        [string]$LogPath,
        [string]$WtPath,
        [string]$WtProfile
    )

    if ([string]::IsNullOrWhiteSpace($LogPath)) {
        $LogPath = 'C:\Users\Public\dialtone-typing-terminal.log'
    }

    function Write-DialtoneLaunchLog {
        param([string]$Message)
        try {
            $logDir = Split-Path -Path $LogPath -Parent
            if (-not [string]::IsNullOrWhiteSpace($logDir)) {
                New-Item -ItemType Directory -Path $logDir -Force | Out-Null
            }
            Add-Content -Path $LogPath -Value ("{0} {1}" -f (Get-Date -Format o), $Message) -ErrorAction SilentlyContinue
        } catch {
        }
    }

    Write-DialtoneLaunchLog ("launch requested; WtPath={0}; WtProfile={1}; LogPath={2}; WslPath={3}" -f $WtPath, $WtProfile, $LogPath, $WslPath)

    $statePath = $LogPath + '.state.json'
    $tagPath = $LogPath + '.tagged-pids.txt'
    $queuePath = $LogPath + '.queue.txt'
    $eventLogPath = $LogPath + '.events.log'
    $outputLogPath = $LogPath + '.window.log'
    $terminalTag = 'dialtone-terminal-v1'
    $windowTitle = 'DialtoneTypingMain[' + $terminalTag + ']'

    function Register-DialtoneTaggedPid {
        param([int]$ProcessId)
        try {
            if ($ProcessId -le 0) {
                return
            }
            $existing = @()
            if (Test-Path -LiteralPath $tagPath) {
                $existing = Get-Content -LiteralPath $tagPath -ErrorAction SilentlyContinue
            }
            $pidText = [string]$ProcessId
            if ($existing -contains $pidText) {
                return
            }
            Add-Content -LiteralPath $tagPath -Value $pidText -Encoding Ascii -ErrorAction SilentlyContinue
            Write-DialtoneLaunchLog ("tagged pid={0} tag={1}" -f $ProcessId, $terminalTag)
        } catch {
            Write-DialtoneLaunchLog ("tag register failed: {0}" -f $_.Exception.Message)
        }
    }

    try {
        if (-not [string]::IsNullOrWhiteSpace($CommandText)) {
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($CommandText)
            $line = [Convert]::ToBase64String($bytes)
            Add-Content -LiteralPath $queuePath -Value $line -Encoding Ascii -ErrorAction SilentlyContinue
            Write-DialtoneLaunchLog ("queued command b64 length={0}" -f $line.Length)
        }
    } catch {
        Write-DialtoneLaunchLog ("queue write failed: {0}" -f $_.Exception.Message)
        throw
    }

    $existingPid = $null
    if (Test-Path -LiteralPath $statePath) {
        try {
            $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
            if ($state -and $state.Pid) {
                $existingPid = [int]$state.Pid
            }
        } catch {
            Write-DialtoneLaunchLog ("state read failed: {0}" -f $_.Exception.Message)
        }
    }

    if ($existingPid) {
        $proc = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
        if ($proc) {
            Register-DialtoneTaggedPid -ProcessId $existingPid
            Write-DialtoneLaunchLog ("reusing existing window pid={0}" -f $existingPid)
            return
        }
    }

    try {
        $bootstrap = '$Host.UI.RawUI.WindowTitle = ''' + $windowTitle + '''; ' +
            '$env:DIALTONE_TERMINAL_TAG = ''' + $terminalTag + '''; ' +
            '$script:__queuePath = ''' + $queuePath.Replace("'", "''") + '''; ' +
            '$script:__outPath = ''' + $outputLogPath.Replace("'", "''") + '''; ' +
            '$script:__eventPath = ''' + $eventLogPath.Replace("'", "''") + '''; ' +
            '$script:__idx = 0; ' +
            'try { Start-Transcript -Path $script:__outPath -Append -Force | Out-Null } catch {} ; ' +
            'while ($true) { ' +
            'if (Test-Path -LiteralPath $script:__queuePath) { ' +
            '$lines = Get-Content -LiteralPath $script:__queuePath; ' +
            '$newIdx = $script:__idx; ' +
            'for ($i = $script:__idx; $i -lt $lines.Count; $i++) { ' +
            '$line = $lines[$i]; $line = [string]$line; $line = $line.Trim(); $line = $line.Trim([char]0xFEFF); if ([string]::IsNullOrWhiteSpace($line)) { $newIdx = $i + 1; continue } ' +
            'try { $cmd = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($line)); Write-Host (''PS C:\> '' + $cmd); try { Add-Content -LiteralPath $script:__eventPath -Value ((Get-Date -Format o) + '' exec-start '' + $cmd) -ErrorAction SilentlyContinue } catch {}; Invoke-Expression $cmd; try { Add-Content -LiteralPath $script:__eventPath -Value ((Get-Date -Format o) + '' exec-end '' + $cmd) -ErrorAction SilentlyContinue } catch {}; $newIdx = $i + 1 } catch { if (($line -match ''^[A-Za-z0-9+/=]+$'') -and (($line.Length % 4) -ne 0)) { break }; try { Add-Content -LiteralPath $script:__eventPath -Value ((Get-Date -Format o) + '' exec-error '' + $_.Exception.Message) -ErrorAction SilentlyContinue } catch {}; $newIdx = $i + 1 } } ' +
            '$script:__idx = $newIdx } ; Start-Sleep -Milliseconds 250 }'
        $encoded = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($bootstrap))
        $proc = Start-Process -FilePath 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' -WorkingDirectory 'C:\' -ArgumentList @('-NoExit', '-EncodedCommand', $encoded) -PassThru
        @{ Pid = $proc.Id; WindowTitle = $windowTitle } | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding UTF8
        Register-DialtoneTaggedPid -ProcessId $proc.Id
        Write-DialtoneLaunchLog ("launched powershell window pid={0}" -f $proc.Id)
        Write-DialtoneLaunchLog ("window transcript path={0}" -f $outputLogPath)
        Write-DialtoneLaunchLog ("queue path={0}" -f $queuePath)
        Write-DialtoneLaunchLog ("event log path={0}" -f $eventLogPath)
    } catch {
        Write-DialtoneLaunchLog ("powershell launch failed: {0}" -f $_.Exception.Message)
        throw
    }
}
