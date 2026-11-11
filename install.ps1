# Download only. Compatible with Windows PowerShell 5.1 and PowerShell 7.
[CmdletBinding()]
param(
    [string]$Version = $(if ($env:DIALTONE_VERSION) { $env:DIALTONE_VERSION } else { 'v0.5.3' }),
    [string]$InstallDir = $(if ($env:DIALTONE_INSTALL_DIR) { $env:DIALTONE_INSTALL_DIR } else { Join-Path $HOME '.local\bin' })
)

# Small file-operation boundaries let local tests reproduce partial Windows
# replacement errors without changing the registry or using a real executable.
function Get-DialtoneMachineArchitecture {
    [Microsoft.Win32.Registry]::GetValue(
        'HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
        'PROCESSOR_ARCHITECTURE', $null)
}

function Replace-DialtoneExecutable($Source, $Destination, $Backup) {
    [IO.File]::Replace($Source, $Destination, $Backup)
}

function Restore-DialtoneExecutable($Backup, $Destination) {
    # Copy, never move: keep the independent recovery copy if restoration fails.
    [IO.File]::Copy($Backup, $Destination, $true)
}

function Install-Dialtone {
    $ErrorActionPreference = 'Stop'
    $downloadDir = $null
    $stageDir = $null
    $keepRecovery = $false
    $previousTls = [Net.ServicePointManager]::SecurityProtocol
    try {
        if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
            throw 'Unsupported OS: use install.sh on Linux or macOS.'
        }
        # An emulated shell can report AMD64 on ARM64. Also require the machine
        # environment from the shared HKLM\SYSTEM registry view; fail if unknown.
        $arch = $env:PROCESSOR_ARCHITEW6432
        if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
        $machineArch = Get-DialtoneMachineArchitecture
        if ($arch -ne 'AMD64' -or $machineArch -ne 'AMD64') {
            throw "Unsupported Windows architecture: $machineArch (shell: $arch). Only x86_64 is published; see docs/install.md."
        }
        if ($Version -cnotmatch '\Av[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9.-]*)?\z') {
            throw 'Version must be a tag such as v0.5.3.'
        }
        if ([string]::IsNullOrWhiteSpace($InstallDir)) { throw 'Install directory is empty.' }
        $provider = $null
        $drive = $null
        $destinationDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallDir, [ref]$provider, [ref]$drive)
        if ($provider.Name -ne 'FileSystem') { throw 'InstallDir must use the FileSystem provider.' }
        $releaseBase = 'https://github.com/timcash/dialtone/releases'
        if ($env:DIALTONE_RELEASE_BASE_URL) { $releaseBase = $env:DIALTONE_RELEASE_BASE_URL.TrimEnd('/') }
        $baseUri = [Uri]$releaseBase
        if (-not $baseUri.IsAbsoluteUri -or ($baseUri.Scheme -ne 'https' -and
            -not ($baseUri.Scheme -eq 'http' -and $baseUri.Host -eq '127.0.0.1'))) {
            throw 'Release base must use HTTPS (or loopback HTTP for tests).'
        }
        $asset = 'dialtone-windows-x86_64.exe'
        # Both requests use this one validated tag, never latest/download.
        $url = "$releaseBase/download/$Version/$asset"
        $downloadDir = Join-Path ([IO.Path]::GetTempPath()) ('dialtone-install-' + [Guid]::NewGuid().ToString('N'))
        $null = [IO.Directory]::CreateDirectory($downloadDir)
        $binary = Join-Path $downloadDir $asset
        $checksum = Join-Path $downloadDir 'checksum'
        # HttpClient is available on both supported PowerShell versions. Disable
        # automatic redirects so HTTPS can never silently downgrade to HTTP.
        Add-Type -AssemblyName System.Net.Http
        $handler = New-Object System.Net.Http.HttpClientHandler
        $handler.AllowAutoRedirect = $false
        # .NET Framework may default to TLS 1.0 in stock Windows PowerShell 5.1.
        # Prefer the handler's own setting; older Framework handlers use the
        # process-wide ServicePointManager setting, restored on every exit.
        [Net.ServicePointManager]::SecurityProtocol = $previousTls -bor [Net.SecurityProtocolType]::Tls12
        if ($handler.PSObject.Properties['SslProtocols']) {
            $handler.SslProtocols = [Security.Authentication.SslProtocols]::Tls12
        }
        $client = New-Object System.Net.Http.HttpClient($handler)
        $client.Timeout = [TimeSpan]::FromSeconds(300)
        try {
            foreach ($item in @(@($url, $binary), @("$url.sha256", $checksum))) {
                $uri = [Uri]$item[0]
                for ($redirects = 0; ; $redirects++) {
                    if ($uri.Scheme -ne 'https' -and -not
                        ($baseUri.Scheme -eq 'http' -and $uri.Scheme -eq 'http' -and $uri.Host -eq '127.0.0.1')) {
                        throw 'Refusing insecure download redirect.'
                    }
                    # Buffer the response so HttpClient.Timeout covers the body
                    # as well as headers on .NET Framework (PowerShell 5.1).
                    $response = $client.GetAsync($uri).GetAwaiter().GetResult()
                    try {
                        if ([int]$response.StatusCode -in 301, 302, 303, 307, 308) {
                            if ($redirects -ge 5 -or -not $response.Headers.Location) { throw 'Invalid download redirect.' }
                            $uri = New-Object Uri($uri, $response.Headers.Location)
                            continue
                        }
                        $null = $response.EnsureSuccessStatusCode()
                        $stream = [IO.File]::Create($item[1])
                        try { $null = $response.Content.CopyToAsync($stream).GetAwaiter().GetResult() } finally { $stream.Dispose() }
                        break
                    } finally { $response.Dispose() }
                }
            }
        } finally { $client.Dispose(); $handler.Dispose() }
        $record = [Text.Encoding]::ASCII.GetString([IO.File]::ReadAllBytes($checksum))
        $pattern = '\A([0-9a-fA-F]{64})  ' + [Regex]::Escape($asset) + '\r?\n\z'
        if ($record -cnotmatch $pattern) { throw 'Invalid checksum record.' }
        $expected = $Matches[1]
        $hasher = [Security.Cryptography.SHA256]::Create()
        $stream = [IO.File]::OpenRead($binary)
        try { $actual = [BitConverter]::ToString($hasher.ComputeHash($stream)).Replace('-', '') }
        finally { $stream.Dispose(); $hasher.Dispose() }
        if ($actual -ne $expected) {
            throw 'SHA-256 mismatch; previous install preserved.'
        }
        if ((Get-Item -LiteralPath $binary).Length -eq 0) { throw 'Empty binary.' }

        $null = [IO.Directory]::CreateDirectory($destinationDir)
        $destination = Join-Path $destinationDir 'dialtone.exe'
        $stageDir = Join-Path $destinationDir ('.dialtone-install-' + [Guid]::NewGuid().ToString('N'))
        $null = [IO.Directory]::CreateDirectory($stageDir)
        $staged = Join-Path $stageDir 'dialtone.exe'
        [IO.File]::Copy($binary, $staged)
        if ([IO.File]::Exists($destination)) {
            # ReplaceFile can fail AFTER removing the destination (e.g. 1176).
            # Keep an independent copy in addition to its own backup argument.
            $recovery = Join-Path $stageDir 'dialtone.previous.exe'
            [IO.File]::Copy($destination, $recovery)
            $keepRecovery = $true
            $replaceBackup = Join-Path $stageDir 'dialtone.replaced.exe'
            try {
                Replace-DialtoneExecutable $staged $destination $replaceBackup
            } catch {
                $replaceError = $_
                try {
                    Restore-DialtoneExecutable $recovery $destination
                    $keepRecovery = $false
                } catch {
                    $keepRecovery = $true
                    throw "Replacement and restore failed. Previous executable kept at '$recovery'. Copy it to '$destination' after resolving the file error. Restore error: $_; replacement error: $replaceError"
                }
                throw "Replacement failed; previous executable restored. $replaceError"
            }
            $keepRecovery = $false
        } else {
            [IO.File]::Move($staged, $destination)
        }
        Write-Host "Installed $Version to $destination"
        Write-Host 'No node started. Add the install directory to your PATH, or use the full path.'
        Write-Host 'See https://github.com/timcash/dialtone/blob/main/docs/install.md'
    } finally {
        [Net.ServicePointManager]::SecurityProtocol = $previousTls
        # Only our GUID-named temporary children are removed, never InstallDir.
        if ($downloadDir) { Remove-Item -LiteralPath $downloadDir -Recurse -Force -ErrorAction SilentlyContinue }
        if ($stageDir -and -not $keepRecovery) { Remove-Item -LiteralPath $stageDir -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

Install-Dialtone
