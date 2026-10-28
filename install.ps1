# Install dialtone and join a channel with an invite (Windows x86_64).
#
#   $env:DIALTONE_INVITE='<invite>'; irm https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 | iex
#
# Downloads the latest release binary to ~\.dialtone\bin, checks its SHA-256, joins with the
# invite and installs the node so it starts at login. No build tools needed. Set
# $env:DIALTONE_NODE to choose the node name (default: the computer name). With
# $env:DIALTONE_STATE_DIR set (a second node on one machine, or a trial), everything goes in
# that folder and the node is started but not installed at login; set $env:DIALTONE_LISTEN
# too if another node already uses 127.0.0.1:8788.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$invite = $env:DIALTONE_INVITE
if (-not $invite -or -not $invite.StartsWith('dialtone-invite-v2:')) {
    throw "Set `$env:DIALTONE_INVITE to the invite (dialtone-invite-v2:...) first."
}
$node = $env:DIALTONE_NODE
if (-not $node) { $node = $env:COMPUTERNAME }
$node = ($node.ToLower() -replace '[^a-z0-9._-]', '-')
if ($node.Length -gt 64) { $node = $node.Substring(0, 64) }

$repo = 'https://github.com/timcash/dialtone/releases/latest/download'
$asset = 'dialtone-windows-x86_64.exe'
$state = $env:DIALTONE_STATE_DIR
if (-not $state) { $state = Join-Path $env:USERPROFILE '.dialtone' }
$listen = $env:DIALTONE_LISTEN
if (-not $listen) { $listen = '127.0.0.1:8788' }
$binDir = Join-Path $state 'bin'
$bin = Join-Path $binDir 'dialtone.exe'
New-Item -ItemType Directory -Force $binDir | Out-Null

Write-Host "Downloading $asset ..."
$tmp = "$bin.new"
Invoke-WebRequest "$repo/$asset" -OutFile $tmp -UseBasicParsing
$sum = (Invoke-WebRequest "$repo/$asset.sha256" -UseBasicParsing).Content
if ($sum -is [byte[]]) { $sum = [Text.Encoding]::ASCII.GetString($sum) }
$want = ($sum.Trim() -split '\s+')[0].ToUpper()
$got = (Get-FileHash $tmp -Algorithm SHA256).Hash
if ($got -ne $want) { Remove-Item $tmp; throw "checksum mismatch for $asset" }

# A node already installed here is stopped so its binary can be replaced.
Get-CimInstance Win32_Process |
    Where-Object { $_.ExecutablePath -eq $bin -or ($_.CommandLine -like '*start-dialtone.cmd*' -and -not $env:DIALTONE_STATE_DIR) } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Start-Sleep -Milliseconds 500
Move-Item -Force $tmp $bin

Write-Host "Joining as '$node' ..."
& $bin join $invite --node $node --listen $listen --once
if ($LASTEXITCODE -ne 0) { throw "join failed" }
if ($env:DIALTONE_STATE_DIR) {
    $log = Join-Path $state 'daemon.log'
    Start-Process conhost.exe -ArgumentList '--headless', 'cmd.exe', '/c', "`"`"$bin`" up --node $node --listen $listen >> `"$log`" 2>&1`"" -WindowStyle Hidden
    Write-Host "Done. '$node' is joining the mesh (started now, not installed at login)."
} else {
    & $bin daemon install --node $node --listen $listen | Out-Null
    $cmd = Join-Path $state 'start-dialtone.cmd'
    Start-Process conhost.exe -ArgumentList '--headless', 'cmd.exe', '/c', "`"$cmd`"" -WindowStyle Hidden
    Write-Host "Done. '$node' is joining the mesh; it starts again at login."
}
Write-Host "  HUD:    http://$listen"
Write-Host "  Status: $bin status"
