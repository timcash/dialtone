param(
    [int[]]$Ports = @(9222, 9223, 9224, 9225),
    [switch]$StartChrome,
    [string]$ChromePath = "",
    [string]$UserDataDir = "$env:TEMP\dialtone-chrome-wsl-devtools",
    [string]$Url = "about:blank",
    [switch]$Remove
)

$ErrorActionPreference = "Stop"

function Test-IsAdmin {
    $current = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($current)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Resolve-ChromePath {
    param([string]$Preferred)
    if ($Preferred -and (Test-Path $Preferred)) {
        return $Preferred
    }
    $candidates = @(
        "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
        "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
        "$env:LocalAppData\Google\Chrome\Application\chrome.exe",
        "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
        "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    )
    foreach ($p in $candidates) {
        if ($p -and (Test-Path $p)) {
            return $p
        }
    }
    return ""
}

if (-not (Test-IsAdmin)) {
    Write-Error "Run this script in an elevated PowerShell window (Administrator)."
}

$ports = @($Ports | Where-Object { $_ -gt 0 } | Select-Object -Unique)
if ($ports.Count -eq 0) {
    Write-Error "No valid ports provided."
}

foreach ($port in $ports) {
    $ruleName = "Dialtone Chrome DevTools WSL $port"
    Write-Host "Configuring port $port ..."

    & netsh interface portproxy delete v4tov4 listenport=$port listenaddress=0.0.0.0 | Out-Null
    if (-not $Remove) {
        & netsh interface portproxy add v4tov4 listenport=$port listenaddress=0.0.0.0 connectport=$port connectaddress=127.0.0.1 | Out-Null
        Write-Host "  portproxy: 0.0.0.0:$port -> 127.0.0.1:$port"
    } else {
        Write-Host "  portproxy removed for $port"
    }

    $existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    if ($existing) {
        Remove-NetFirewallRule -DisplayName $ruleName | Out-Null
    }
    if (-not $Remove) {
        New-NetFirewallRule `
            -DisplayName $ruleName `
            -Direction Inbound `
            -Action Allow `
            -Protocol TCP `
            -LocalPort $port `
            -Profile Any | Out-Null
        Write-Host "  firewall rule added: $ruleName"
    } else {
        Write-Host "  firewall rule removed: $ruleName"
    }
}

if ($StartChrome -and -not $Remove) {
    $chrome = Resolve-ChromePath -Preferred $ChromePath
    if (-not $chrome) {
        Write-Error "Could not find Chrome/Edge executable. Pass -ChromePath explicitly."
    }
    if (-not (Test-Path $UserDataDir)) {
        New-Item -ItemType Directory -Path $UserDataDir -Force | Out-Null
    }
    $port = $ports[0]
    $args = @(
        "--remote-debugging-port=$port",
        "--remote-debugging-address=127.0.0.1",
        "--remote-allow-origins=*",
        "--no-first-run",
        "--no-default-browser-check",
        "--user-data-dir=$UserDataDir",
        "--new-window",
        "--dialtone-origin=true",
        "--dialtone-role=dev",
        $Url
    )
    Start-Process -FilePath $chrome -ArgumentList $args | Out-Null
    Write-Host "Started browser: $chrome"
    Write-Host "DevTools JSON endpoint: http://127.0.0.1:$port/json/version"
}

$wslAdapter = Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias "vEthernet (WSL)" -ErrorAction SilentlyContinue | Select-Object -First 1
if ($wslAdapter) {
    Write-Host ""
    Write-Host "WSL host IP: $($wslAdapter.IPAddress)"
    Write-Host "From WSL, test with:"
    Write-Host "  curl http://$($wslAdapter.IPAddress):$($ports[0])/json/version"
}

Write-Host ""
if ($Remove) {
    Write-Host "Done. WSL DevTools forwarding removed."
} else {
    Write-Host "Done. WSL DevTools forwarding configured."
}
