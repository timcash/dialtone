# Dialtone

Install **v0.5.3** on Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh | sh
```

Windows x64 (PowerShell 5.1 or 7; Windows 10 1803+, Windows 11, or Server 2019+ for the built-in `curl.exe`):

```powershell
iex (curl.exe -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1 | Out-String)
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

`up` starts an unmanaged foreground node and prints its HUD URL (normally http://127.0.0.1:8788); this `.local/bin` copy does not self-update. To join an existing channel, obtain an invite from a member and follow the [join instructions](docs/guide.md#3-join-other-computers-with-an-invite). Joining with an invite or running `daemon install` explicitly installs and starts the managed `~/.dialtone/bin` copy, which can receive signed updates; the installer does neither.

| Supported platform | v0.5.3 binary |
| --- | --- |
| Linux x86_64 | Static musl |
| Linux ARM64 | GNU/glibc; not an Alpine/musl binary |
| macOS Apple Silicon | Native ARM64 (including Rosetta shells) |
| Windows x86_64 | Native x64 |

Intel macOS, Windows ARM64, and 32-bit systems have no published binary in this release; the installers refuse them. See [requirements and source builds](docs/install.md#supported-platforms).

[Download and inspect before running](docs/install.md#download-and-inspect) · [Version, directory, and manual download options](docs/install.md) · [Full user guide](docs/guide.md) · [Releases](https://github.com/timcash/dialtone/releases) · [Design and domain language](goal.md)
