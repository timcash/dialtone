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

    $cmdBytes = [System.Text.Encoding]::UTF8.GetBytes($CommandText)
    $cmdB64 = [System.Convert]::ToBase64String($cmdBytes)
    $bashRunner = 'eval "$(echo ' + $cmdB64 + ' | base64 -d)"; exec /bin/bash -i'
    $wslArgs = @('--cd', '~', '-e', 'bash', '-ic', $bashRunner)
    Write-DialtoneLaunchLog ("launch requested; WslPath={0}; WtPath={1}; WtProfile={2}; LogPath={3}" -f $WslPath, $WtPath, $WtProfile, $LogPath)
    Write-DialtoneLaunchLog ("command b64 length: {0}" -f $cmdB64.Length)
    Write-DialtoneLaunchLog ("wsl args: {0}" -f ($wslArgs -join " "))

    if (-not [string]::IsNullOrWhiteSpace($WtPath) -and -not [string]::IsNullOrWhiteSpace($WtProfile)) {
        try {
            $wtArgs = @('new-window', '-p', $WtProfile, '--', $WslPath) + $wslArgs
            Write-DialtoneLaunchLog ("wt args: {0}" -f ($wtArgs -join " "))
            Start-Process -FilePath $WtPath -WorkingDirectory 'C:\' -ArgumentList $wtArgs
            Write-DialtoneLaunchLog 'launched via wt.exe'
            return
        } catch {
            Write-DialtoneLaunchLog ("wt.exe launch failed: {0}" -f $_.Exception.Message)
        }
    }

    try {
        $pwshRunner = @(
            '$ErrorActionPreference = "Continue"; ' +
            '& "' + $WslPath + '" --cd ~ -e bash -ic ''' + $bashRunner.Replace("'", "''") + ''''
        )
        Write-DialtoneLaunchLog ("powershell -NoExit runner: {0}" -f $pwshRunner[0])
        Start-Process -FilePath 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' -WorkingDirectory 'C:\' -ArgumentList @('-NoExit', '-Command', $pwshRunner[0])
        Write-DialtoneLaunchLog 'launched via powershell.exe -NoExit wrapper'
    } catch {
        Write-DialtoneLaunchLog ("powershell -NoExit launch failed: {0}" -f $_.Exception.Message)
        throw
    }
}
