[CmdletBinding(PositionalBinding = $false)]
param(
  [Parameter(Position = 0)]
  [ValidateSet("start", "stop", "restart", "status", "logs")]
  [string]$Action = "status",
  [string[]]$Role = @("dev"),
  [string]$HostId = "",
  [string]$NatsUrl = "",
  [int]$Tail = 80,
  [int]$WaitSeconds = 20,
  [switch]$NoWait
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$BinPath = Join-Path $env:USERPROFILE ".dialtone\bin\dialtone_chrome_v3.exe"
$BinDir = Split-Path -Parent $BinPath

function Normalize-Role {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) {
    return "dev"
  }
  return $Value.Trim()
}

function Get-RolePort {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  if ($RoleName -eq "dev") {
    return 19464
  }
  $bytes = [System.Text.Encoding]::UTF8.GetBytes($RoleName)
  [uint32]$hash = 2166136261
  foreach ($b in $bytes) {
    $hash = $hash -bxor [uint32]$b
    [uint64]$next = ([uint64]$hash) * 16777619
    $hash = [uint32]($next % 4294967296)
  }
  $offset = [int]($hash % 2000) + 1
  return 19464 + ($offset * 2)
}

function Get-RoleProfileDir {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  return Join-Path $env:USERPROFILE (".dialtone\chrome-v3\" + $RoleName)
}

function Get-RoleServiceDir {
  param([string]$RoleName)
  return Join-Path (Get-RoleProfileDir $RoleName) "service"
}

function Get-RoleStdoutPath {
  param([string]$RoleName)
  return Join-Path (Get-RoleServiceDir $RoleName) "daemon.out.log"
}

function Get-RoleStderrPath {
  param([string]$RoleName)
  return Join-Path (Get-RoleServiceDir $RoleName) "daemon.err.log"
}

function Get-RoleStatePath {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  return Join-Path $env:USERPROFILE (".dialtone\chrome-src-v3\" + $RoleName + "\state.json")
}

function Read-JsonFile {
  param([string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    return $null
  }
  try {
    return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
  }
  catch {
    return $null
  }
}

function Resolve-HostIdValue {
  if (-not [string]::IsNullOrWhiteSpace($HostId)) {
    return $HostId.Trim()
  }
  $configPath = Join-Path $RepoRoot "env\dialtone.json"
  $config = Read-JsonFile $configPath
  if ($null -ne $config -and $null -ne $config.mesh_nodes) {
    foreach ($node in $config.mesh_nodes) {
      if ([string]::Equals([string]$node.os, "windows", [System.StringComparison]::OrdinalIgnoreCase)) {
        return [string]$node.name
      }
    }
  }
  return $env:COMPUTERNAME.ToLowerInvariant()
}

function Import-ChromeConfigEnvironment {
  $configPath = Join-Path $RepoRoot "env\dialtone.json"
  $config = Read-JsonFile $configPath
  if ($null -eq $config) {
    return
  }
  foreach ($prop in $config.PSObject.Properties) {
    if ($prop.Name -notlike "DIALTONE_CHROME_SRC_V3_*") {
      continue
    }
    if ($null -eq $prop.Value) {
      continue
    }
    $value = [string]$prop.Value
    if ([string]::IsNullOrWhiteSpace($value)) {
      continue
    }
    [System.Environment]::SetEnvironmentVariable($prop.Name, $value.Trim(), "Process")
  }
}

function Get-ChromeConfigValue {
  param([string]$Name)
  $value = [System.Environment]::GetEnvironmentVariable($Name, "Process")
  if ($null -eq $value) {
    return ""
  }
  return [string]$value
}

function Resolve-LeaderState {
  $leaderRaw = & wsl.exe bash -lc "test -f /home/user/.dialtone/repl-v3/leader.json && cat /home/user/.dialtone/repl-v3/leader.json" 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($leaderRaw | Out-String))) {
    return $null
  }
  try {
    return ($leaderRaw | Out-String) | ConvertFrom-Json
  }
  catch {
    return $null
  }
}

function Resolve-NatsUrlValue {
  if (-not [string]::IsNullOrWhiteSpace($NatsUrl)) {
    return $NatsUrl.Trim()
  }
  foreach ($roleName in $Role) {
    $state = Read-JsonFile (Get-RoleStatePath $roleName)
    if ($null -ne $state -and -not [string]::IsNullOrWhiteSpace([string]$state.nats_url)) {
      return [string]$state.nats_url
    }
  }
  $leaderState = Resolve-LeaderState
  if ($null -ne $leaderState) {
    if (-not [string]::IsNullOrWhiteSpace([string]$leaderState.tsnet_nats_url)) {
      return [string]$leaderState.tsnet_nats_url
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$leaderState.nats_url) -and -not [string]$leaderState.nats_url.StartsWith("nats://127.0.0.1")) {
      return [string]$leaderState.nats_url
    }
  }
  throw "Unable to resolve NATS URL. Start the WSL leader from tmux first or pass -NatsUrl explicitly."
}

function Test-NatsEndpoint {
  param([string]$Url)
  $uri = [System.Uri]$Url
  $client = New-Object System.Net.Sockets.TcpClient
  try {
    $iar = $client.BeginConnect($uri.Host, $uri.Port, $null, $null)
    if (-not $iar.AsyncWaitHandle.WaitOne(3000, $false)) {
      $client.Close()
      return $false
    }
    $client.EndConnect($iar) | Out-Null
    return $true
  }
  catch {
    return $false
  }
  finally {
    $client.Close()
  }
}

function Get-DaemonProcessesForRole {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  $pattern = "(^|\s)--role(?:\s+|=)""?" + [regex]::Escape($RoleName) + """?(?:\s|$)"
  return @(Get-CimInstance Win32_Process | Where-Object {
    $_.Name -eq "dialtone_chrome_v3.exe" -and
    $_.ExecutablePath -eq $BinPath -and
    [regex]::IsMatch([string]$_.CommandLine, $pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
  })
}

function Get-ChromeProcessesForRole {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  $profileDir = Get-RoleProfileDir $RoleName
  $chromePort = Get-RolePort $RoleName
  return @(Get-CimInstance Win32_Process | Where-Object {
    $_.Name -eq "chrome.exe" -and (
      $_.CommandLine -like ("*--remote-debugging-port=" + $chromePort + "*") -or
      $_.CommandLine -like ("*--dialtone-role=" + $RoleName + "*") -or
      $_.CommandLine -like ("*" + $profileDir + "*")
    )
  })
}

function Stop-RoleProcesses {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  foreach ($proc in Get-ChromeProcessesForRole $RoleName) {
    Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
  }
  foreach ($proc in Get-DaemonProcessesForRole $RoleName) {
    Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
  }
}

function Wait-RoleReady {
  param(
    [string]$RoleName,
    [int]$TimeoutSeconds
  )
  $RoleName = Normalize-Role $RoleName
  $stdoutPath = Get-RoleStdoutPath $RoleName
  $statePath = Get-RoleStatePath $RoleName
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    $state = Read-JsonFile $statePath
    $hasState = $null -ne $state -and [int]$state.service_pid -gt 0
    $logText = ""
    if (Test-Path -LiteralPath $stdoutPath) {
      $logText = Get-Content -LiteralPath $stdoutPath -Raw
    }
    $hasReady = $logText -match ("chrome src_v3 daemon ready role=" + [regex]::Escape($RoleName))
    if ($hasState -and $hasReady) {
      return
    }
    Start-Sleep -Milliseconds 250
  } while ((Get-Date) -lt $deadline)
  throw ("Timed out waiting for chrome src_v3 daemon readiness for role=" + $RoleName)
}

function Start-RoleDaemon {
  param(
    [string]$RoleName,
    [string]$ResolvedHostId,
    [string]$ResolvedNatsUrl,
    [int]$TimeoutSeconds
  )
  $RoleName = Normalize-Role $RoleName
  $chromePort = Get-RolePort $RoleName
  $serviceDir = Get-RoleServiceDir $RoleName
  $stdoutPath = Get-RoleStdoutPath $RoleName
  $stderrPath = Get-RoleStderrPath $RoleName
  New-Item -ItemType Directory -Path $serviceDir -Force | Out-Null
  Remove-Item -LiteralPath $stdoutPath,$stderrPath -Force -ErrorAction SilentlyContinue
  Import-ChromeConfigEnvironment
  $proc = Start-Process -FilePath $BinPath -ArgumentList @(
    "src_v3", "daemon",
    "--role", $RoleName,
    "--chrome-port", "$chromePort",
    "--host-id", $ResolvedHostId,
    "--nats-url", $ResolvedNatsUrl
  ) -WorkingDirectory $BinDir -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -WindowStyle Hidden -PassThru
  if (-not $NoWait) {
    Wait-RoleReady -RoleName $RoleName -TimeoutSeconds $TimeoutSeconds
  }
  return $proc
}

function Show-RoleStatus {
  param([string]$RoleName)
  $RoleName = Normalize-Role $RoleName
  $state = Read-JsonFile (Get-RoleStatePath $RoleName)
  $daemons = Get-DaemonProcessesForRole $RoleName
  $chromes = Get-ChromeProcessesForRole $RoleName
  Write-Output ("role=" + $RoleName)
  Write-Output ("  daemon_count=" + (@($daemons).Count))
  Write-Output ("  chrome_count=" + (@($chromes).Count))
  Write-Output ("  chrome_port=" + (Get-RolePort $RoleName))
  Write-Output ("  stdout=" + (Get-RoleStdoutPath $RoleName))
  Write-Output ("  stderr=" + (Get-RoleStderrPath $RoleName))
  Write-Output ("  cfg_actions_per_second=" + (Get-ChromeConfigValue "DIALTONE_CHROME_SRC_V3_ACTIONS_PER_SECOND"))
  Write-Output ("  cfg_interaction_count=" + (Get-ChromeConfigValue "DIALTONE_CHROME_SRC_V3_INTERACTION_COUNT"))
  Write-Output ("  cfg_headless=" + (Get-ChromeConfigValue "DIALTONE_CHROME_SRC_V3_HEADLESS"))
  Write-Output ("  cfg_step_delay_ms=" + (Get-ChromeConfigValue "DIALTONE_CHROME_SRC_V3_STEP_DELAY_MS"))
  if ($null -ne $state) {
    Write-Output ("  state_service_pid=" + [string]$state.service_pid)
    Write-Output ("  state_browser_pid=" + [string]$state.browser_pid)
    Write-Output ("  state_nats_url=" + [string]$state.nats_url)
    Write-Output ("  state_last_healthy_at=" + [string]$state.last_healthy_at)
  }
}

Import-ChromeConfigEnvironment

if (-not (Test-Path -LiteralPath $BinPath)) {
  throw "Missing chrome daemon binary at $BinPath. Run the WSL deploy step first."
}

$resolvedHostId = Resolve-HostIdValue
$resolvedNatsUrl = ""
if ($Action -in @("start", "restart")) {
  $resolvedNatsUrl = Resolve-NatsUrlValue
  if (-not (Test-NatsEndpoint $resolvedNatsUrl)) {
    throw "NATS endpoint is not reachable from Windows: $resolvedNatsUrl"
  }
}

switch ($Action) {
  "stop" {
    foreach ($roleName in $Role) {
      $roleName = Normalize-Role $roleName
      Stop-RoleProcesses $roleName
      Write-Output ("STOPPED role=" + $roleName)
    }
  }
  "start" {
    foreach ($roleName in $Role) {
      $roleName = Normalize-Role $roleName
      $proc = Start-RoleDaemon -RoleName $roleName -ResolvedHostId $resolvedHostId -ResolvedNatsUrl $resolvedNatsUrl -TimeoutSeconds $WaitSeconds
      Write-Output ("STARTED role=" + $roleName + " pid=" + $proc.Id + " port=" + (Get-RolePort $roleName))
    }
  }
  "restart" {
    foreach ($roleName in $Role) {
      $roleName = Normalize-Role $roleName
      Stop-RoleProcesses $roleName
      $proc = Start-RoleDaemon -RoleName $roleName -ResolvedHostId $resolvedHostId -ResolvedNatsUrl $resolvedNatsUrl -TimeoutSeconds $WaitSeconds
      Write-Output ("RESTARTED role=" + $roleName + " pid=" + $proc.Id + " port=" + (Get-RolePort $roleName))
    }
  }
  "status" {
    foreach ($roleName in $Role) {
      Show-RoleStatus $roleName
    }
  }
  "logs" {
    foreach ($roleName in $Role) {
      $roleName = Normalize-Role $roleName
      Write-Output ("=== " + $roleName + " stdout ===")
      if (Test-Path -LiteralPath (Get-RoleStdoutPath $roleName)) {
        Get-Content -LiteralPath (Get-RoleStdoutPath $roleName) -Tail $Tail
      }
      Write-Output ("=== " + $roleName + " stderr ===")
      if (Test-Path -LiteralPath (Get-RoleStderrPath $roleName)) {
        Get-Content -LiteralPath (Get-RoleStderrPath $roleName) -Tail $Tail
      }
    }
  }
}
