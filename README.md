# Dialtone

Install **v0.5.3** on Linux or macOS:

```sh
sh -c 's=$(curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh) && eval "$s"'
```

Windows x64 (PowerShell 5.1 or 7; Windows 10 1803+, Windows 11, or Server 2019+ for the built-in `curl.exe`):

```powershell
$s = curl.exe -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 | Out-String; if ($LASTEXITCODE) { throw 'install.ps1 download failed' }; iex $s
```

Dialtone is a single-binary peer-to-peer mesh, local web HUD, blob store, and process supervisor. Installation verifies SHA-256 and places only the executable in `~/.local/bin` (`$HOME\.local\bin` on Windows). It needs no administrator privileges and starts nothing.

Use the full path first (or [add the directory to PATH](docs/install.md#path)):

```sh
"$HOME/.local/bin/dialtone" --help
"$HOME/.local/bin/dialtone" up
```

```powershell
& "$HOME\.local\bin\dialtone.exe" --help
& "$HOME\.local\bin\dialtone.exe" up
```

### Plugins & Cross-OS Terminal Sessions (`dialtone install shell`)

Install blob-backed plugins and run long-running, stateful Bash or PowerShell sessions across macOS, Windows, and Linux/WSL over Dialtone QUIC (replacing SSH and `tmux`):

```sh
# Plugins (content-addressed via Dialtone's iroh-blobs store)
dialtone install shell                                           # Install the persistent shell manager plugin
dialtone install <plugin> [--ticket <ticket>] [--file <path>]    # Install a plugin from a peer blob ticket or file
dialtone plugin list [--json]                                    # List installed plugins & verified digests
dialtone plugin share <plugin>                                   # Share an installed plugin as a blob ticket
dialtone plugin remove <plugin>                                  # Remove an installed plugin

# Long-running terminal sessions (local or any peer via --node <peer>)
dialtone shell new <name> [--node <peer>] [--shell auto|bash|powershell] [--cwd <dir>]
dialtone shell send <name> [--node <peer>] [--timeout 30s] [--] <command...>
dialtone shell capture <name> [--node <peer>] [--lines 100]
dialtone shell ls [--node <peer>] [--json]
dialtone shell attach <name> [--node <peer>]
dialtone shell kill <name> [--node <peer>]
```

`up` starts an unmanaged foreground node and prints its HUD URL (normally http://127.0.0.1:8788); this `.local/bin` copy does not self-update. To join an existing channel, obtain an invite from a member and follow the [join instructions](docs/guide.md#3-join-other-computers-with-an-invite). Joining with an invite or running `daemon install` explicitly installs and starts the managed `~/.dialtone/bin` copy, which can receive signed updates; the installer does neither.

| Supported platform | v0.5.3 binary |
| --- | --- |
| Linux x86_64 | Static musl |
| Linux ARM64 | GNU/glibc; not an Alpine/musl binary |
| macOS Apple Silicon | Native ARM64 (including Rosetta shells) |
| Windows x86_64 | Native x64 |

Intel macOS, Windows ARM64, and 32-bit systems have no published binary in this release; the installers refuse them. See [requirements and source builds](docs/install.md#supported-platforms).

[Download and inspect before running](docs/install.md#download-and-inspect) · [Version, directory, and manual download options](docs/install.md) · [Full user guide](docs/guide.md) · [Releases](https://github.com/timcash/dialtone/releases) · [Design and domain language](goal.md)
