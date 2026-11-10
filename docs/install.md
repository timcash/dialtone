# Installing Dialtone

The [README](../README.md) has the one-command installers. The scripts in the `main` source branch default to **v0.5.3**, not a moving `latest/download` alias. They download the binary and its `.sha256` from the same explicit tag, validate the complete checksum record and expected filename, compare SHA-256, and only then replace the executable. Repeating the installation is safe. Failed downloads, missing or malformed checksums, and hash mismatches leave an existing executable unchanged. Temporary files are cleaned up on normal exit and errors, except recovery files retained after a failed Windows rollback.

Windows replacement keeps an independent copy of the previous executable as well as passing a backup path to `File.Replace`. If replacement partially fails, the installer restores the independent copy. If the OS also prevents restoration, the installer fails and prints the retained `INSTALL_DIR\.dialtone-install-<id>\dialtone.previous.exe` recovery path and destination. Keep that directory. After resolving the reported file/permission error, copy the recovery file to the printed destination; do not run an unknown partial replacement. The installer never deletes this recovery copy when rollback fails.

The scripts install only the executable. They do not run it, access node state, join channels, open ports, register services, edit PATH, or request administrator privileges. The default directory is separate from the node's managed `~/.dialtone/bin` directory. Choose a directory you own; do not point the installer at a running node's managed binary. A locked executable on Windows causes installation to fail without deleting it.

## Supported platforms

| OS / architecture | Asset at the v0.5.3 release | Requirements |
| --- | --- | --- |
| Linux x86_64 / amd64 | `dialtone-linux-x86_64` | Static musl binary |
| Linux aarch64 / arm64 | `dialtone-linux-arm64` | GNU/glibc runtime; not musl/Alpine |
| macOS aarch64 / arm64 | `dialtone-macos-arm64` | Apple Silicon; a Rosetta shell is detected with `sysctl` |
| Windows AMD64 / x86_64 | `dialtone-windows-x86_64.exe` | Windows PowerShell 5.1 or PowerShell 7; the one-line bootstrap needs the built-in `curl.exe` (Windows 10 1803+, Windows 11, Server 2019+) |

The Unix installer needs `curl`, standard POSIX utilities, and either `sha256sum` (Linux) or `shasum` (macOS). Windows uses .NET HTTP downloads and SHA-256. Its download handler explicitly enables TLS 1.2, including Windows PowerShell 5.1; any process-wide .NET TLS setting used for older Framework handlers is restored on success and failure. No machine-wide TLS settings change. An HTTPS connection to GitHub and its release asset hosts is required. Local policy may require inspection or approval before downloaded scripts can run; follow that policy without changing machine-wide execution settings.

The release pipeline builds exactly these four targets. The installers reject unsupported OS/architecture selections. Windows requires both its machine environment registry value (from the shared `HKLM\SYSTEM` view) and shell architecture to identify AMD64; an unknown architecture fails closed. This avoids relying on an [emulated shell's processor report](https://learn.microsoft.com/en-us/windows/arm/apps-on-arm-x86-emulation#detecting-emulation) alone. ARM64/unknown registry results with an AMD64 shell are tested by deterministic injection on x64. **Actual Windows ARM64 hardware and emulated PowerShell rejection have not been tested.** There is no Intel macOS, Windows ARM64, or 32-bit release URL to use. Native build and runtime coverage would be required before adding support. You can try a [source build](#build-from-source) on an unsupported target, but it is not a verified release platform.

An exact minimum glibc version for the existing ARM64 asset has not been verified from its ELF symbol requirements. The GNU target and build runner alone do not establish that minimum. If its loader reports a missing glibc version, build on your target system or use a compatible system; do not substitute the x86_64 musl binary on ARM64.

## Download and inspect

The bootstrap scripts live in the source repository; v0.5.3 release assets do **not** contain `install.sh` or `install.ps1`. The short README commands trust the current `main` script over HTTPS.

The Windows command fetches the script with the built-in `curl.exe` (Windows 10 1803+, Windows 11, Server 2019+), which uses the OS TLS stack, so Windows PowerShell 5.1's legacy .NET TLS default does not apply to the bootstrap. Write `curl.exe`, not `curl`: in Windows PowerShell 5.1, `curl` is an alias for `Invoke-WebRequest`. `iex (...)` receives the whole script at once and parses it before running anything. If the download fails, `iex` gets nothing and reports an error. A cut-off script either fails to parse, or stops before its final `Install-Dialtone` line, so nothing is downloaded or installed. From `cmd.exe`, run `powershell -NoProfile -Command "iex (curl.exe -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 | Out-String)"`.

Without `curl.exe` (older Windows), use PowerShell's own download with TLS 1.2 enabled for the session:

```powershell
& { $tls = [Net.ServicePointManager]::SecurityProtocol; try {
  [Net.ServicePointManager]::SecurityProtocol = $tls -bor [Net.SecurityProtocolType]::Tls12
  irm https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 -ErrorAction Stop | iex
} finally { [Net.ServicePointManager]::SecurityProtocol = $tls } }
```

Download it first to review it:

```sh
curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh -o install.sh
less install.sh
sh install.sh
```

```powershell
curl.exe -fsSLo install.ps1 https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1
Get-Content -LiteralPath .\install.ps1
& .\install.ps1
```

For a reproducible bootstrap, replace `main` in the script URL with a full commit SHA that contains the installers, obtained from [repository history](https://github.com/timcash/dialtone/commits/main/). Check out or inspect that commit before executing. The original v0.5.3 source tag predates these scripts, so it cannot supply a pinned installer script. Pinning the script source and pinning the binary version are separate choices.

## Advanced options

After downloading and reviewing the script:

```sh
sh install.sh --version v0.5.3 --install-dir "$HOME/Applications/Dialtone tools"
# Or pass options through the one-command form:
curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh | sh -s -- --version v0.5.3 --install-dir "$HOME/Applications/Dialtone tools"
```

```powershell
& .\install.ps1 -Version v0.5.3 -InstallDir "$HOME\Applications\Dialtone tools"
```

Both scripts also accept `DIALTONE_VERSION` and `DIALTONE_INSTALL_DIR` environment variables; explicit options take precedence. Versions must be explicit tags such as `v0.5.3` (or a prerelease tag); `latest` is deliberately rejected. An unavailable tag or asset fails without changing an existing executable. Custom directories can contain spaces and may be relative to your current directory. PowerShell resolves them against its current FileSystem provider location, including after `Set-Location`, rather than the process working directory. They must be writable by your user.

For a trusted release mirror, `DIALTONE_RELEASE_BASE_URL` replaces `https://github.com/timcash/dialtone/releases`. It must serve `download/<tag>/<asset>` and `download/<tag>/<asset>.sha256` over HTTPS. HTTP is accepted only for `127.0.0.1` fixture servers. This changes the source of first-install integrity data, so use only a mirror you trust. It does not change the node's `DIALTONE_RELEASE_URL` or signed update verification.

## PATH

You can always use the quoted full path shown in the README. For the default install directory, add it to the **current session** explicitly:

```sh
export PATH="$HOME/.local/bin:$PATH"
dialtone --help
```

```powershell
$env:Path = "$HOME\.local\bin;" + $env:Path
dialtone --help
```

For a custom directory, substitute that exact directory. To keep the change in future sessions, add the shell command once to your shell's startup file (`~/.profile` for login sh/bash, `~/.zprofile` for login zsh), or add the directory through Windows **Edit environment variables for your account → User variables → Path** and open a new terminal. PowerShell users may instead place the session command in their existing `$PROFILE`. The installer never silently changes user or system settings.

If another Dialtone is already on PATH, `command -v dialtone` (Unix) or `Get-Command dialtone -All` (PowerShell) shows what will run. Use the full new path when you need to choose the installed version explicitly.

## Manual binary download and trust

You can download a binary and its adjacent `.sha256` directly from the [v0.5.3 release](https://github.com/timcash/dialtone/releases/tag/v0.5.3). Pick the exact asset from the platform table and download both from that **same tag**. Compare its SHA-256 with the 64 hex characters in the checksum file using `sha256sum`, `shasum -a 256`, or `Get-FileHash -Algorithm SHA256`. Confirm that the checksum record names exactly the intended asset. Do not execute or replace an existing binary if either download or verification fails. The scripts automate these steps and atomic replacement.

First-install SHA-256 detects corruption using checksums fetched over HTTPS from the same release source. It is **not independent publisher-signature verification**. Trusting that source and the bootstrap script is part of installation. Running nodes separately require the release-key-signed manifest for automatic updates; these installers do not disable or weaken that check.

`--version` / `-Version` selects the version installed now. Running `~/.local/bin/dialtone up` (or the Windows/custom-directory equivalent) starts an **unmanaged** foreground node; that binary does not self-update. `update::Paths::managed` permits self-updates only for a binary inside the node's state-directory `bin` (normally `~/.dialtone/bin`). Explicitly joining with an invite or running `daemon install` creates and starts that managed copy, which can then receive signed automatic updates. The original `.local/bin` copy remains unchanged. To keep the managed node at a chosen version, set `DIALTONE_AUTO_UPDATE=0` in its environment before starting it (and in its service environment if applicable). See [update settings and state](guide.md#9-environment-variables--local-paths).

## Build from source

From the repository root, install a Rust toolchain and the native C build tools for your OS, then run:

```sh
cargo build --locked --release -p dialtone
```

The executable is `target/release/dialtone` (`target\release\dialtone.exe` on Windows). This only builds; explicitly run or copy the binary when ready. On architectures without published releases, dependency compilation and runtime behavior still need validation. See the [developer guide](guide.md#10-working-on-dialtone-dialtonesh) for the existing checks and release process.

## Installer tests

`python tests/installers.py` (Python 3.10+) runs the native installer against an in-process local HTTP fixture release. It never runs a downloaded binary or a real Dialtone command. The repository entrypoint also exposes this as `./dialtone.sh test --here --only installers`; options after `--` go to the fixture runner. This explicit installer task requires Python; the existing Rust runtime test tasks are unchanged. On Windows, run it once with `--powershell powershell` and once with `--powershell pwsh`. On Unix, it also simulates target selection with `uname`/`sysctl` fixtures; this verifies selection, not those architectures' binaries. `--shell <path-to-sh>` can exercise those selection tests under Git Bash on Windows, which is not native Linux/macOS coverage.

CI runs the fixture tests on native Linux x86_64 and ARM64, macOS ARM64, and Windows x86_64 with both PowerShell versions. The payloads are fixtures, so these jobs test installer behavior and native target selection, not execution of release binaries. Linux ARM64 release building remains in the release matrix. No native Intel macOS or Windows ARM64 support is claimed.

The opt-in `python tests/installers.py --live-release Installers.test_live_release_https` (add `--powershell powershell` on Windows) downloads the actual v0.5.3 release into an isolated custom directory and checks the installed bytes again against the published `.sha256`. On Windows it starts from a legacy process TLS default and checks that the TLS state is restored. Only after verification does it run `dialtone --help`, with isolated `HOME`, `USERPROFILE` and `DIALTONE_STATE_DIR`. It never runs `up`, `join`, `daemon` or `probe`, and never contacts a node. CI runs it on native Linux x86_64 and ARM64, macOS ARM64 and Windows x64, separately from the offline fixtures. A network-blocked local run must be reported as pending, not counted as a pass.
