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
            Add-Content -Path $LogPath -Value ("{0} {1}" -f (Get-Date -Format o), $Message)
        } catch {
        }
    }

    Write-DialtoneLaunchLog ("launch requested; WtPath={0}; WtProfile={1}; LogPath={2}; WslPath={3}" -f $WtPath, $WtProfile, $LogPath, $WslPath)

    $statePath = $LogPath + '.state.json'
    $queuePath = $LogPath + '.queue.txt'
    $outputLogPath = $LogPath + '.window.log'
    $windowTitle = 'DialtoneTypingMain'

    try {
        if (-not [string]::IsNullOrWhiteSpace($CommandText)) {
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($CommandText)
            $line = [Convert]::ToBase64String($bytes)
            Add-Content -Path $queuePath -Value $line
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
            Write-DialtoneLaunchLog ("reusing existing window pid={0}" -f $existingPid)
            return
        }
    }

    try {
        $bootstrap = '$Host.UI.RawUI.WindowTitle = ''' + $windowTitle + '''; ' +
            '$script:__queuePath = ''' + $queuePath.Replace("'", "''") + '''; ' +
            '$script:__outPath = ''' + $outputLogPath.Replace("'", "''") + '''; ' +
            '$script:__idx = 0; ' +
            'try { Start-Transcript -Path $script:__outPath -Append -Force | Out-Null } catch {} ; ' +
            'while ($true) { ' +
            'if (Test-Path -LiteralPath $script:__queuePath) { ' +
            '$lines = Get-Content -LiteralPath $script:__queuePath; ' +
            'for ($i = $script:__idx; $i -lt $lines.Count; $i++) { ' +
            '$line = $lines[$i]; if ([string]::IsNullOrWhiteSpace($line)) { continue } ' +
            'try { $cmd = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($line)); Write-Host (''PS C:\> '' + $cmd); Invoke-Expression $cmd } catch { Write-Host $_.Exception.Message } } ' +
            '$script:__idx = $lines.Count } ; Start-Sleep -Milliseconds 250 }'
        $encoded = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($bootstrap))
        $proc = Start-Process -FilePath 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' -WorkingDirectory 'C:\' -ArgumentList @('-NoExit', '-EncodedCommand', $encoded) -PassThru
        @{ Pid = $proc.Id; WindowTitle = $windowTitle } | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding UTF8
        Write-DialtoneLaunchLog ("launched powershell window pid={0}" -f $proc.Id)
        Write-DialtoneLaunchLog ("window transcript path={0}" -f $outputLogPath)
        Write-DialtoneLaunchLog ("queue path={0}" -f $queuePath)
    } catch {
        Write-DialtoneLaunchLog ("powershell launch failed: {0}" -f $_.Exception.Message)
        throw
    }
}
