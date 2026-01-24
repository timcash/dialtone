# Install Plugin

The `install` plugin manages the development environment dependencies for Dialtone. It ensures a consistent toolchain across Linux/WSL and macOS (Apple Silicon).

## Usage

```bash
# Install dependencies for the current system (auto-detect)
dialtone install

# Install explicitly for Linux/WSL
dialtone install --linux-wsl

# Install explicitly for macOS ARM
dialtone install --macos-arm

# Check valid installation
dialtone install --check

# Clean/Remove dependencies
dialtone install --clean
```

## Installed Tools & Dependencies

The plugin installs the following tools into `DIALTONE_ENV` location (defaults to `~/.dialtone_env`).

### Common Dependencies
| Tool | Version | Notes |
| :--- | :--- | :--- |
| **Go** | `1.25.5` | Main language |
| **Node.js** | `22.13.0` | Runtime for web & scripts |
| **Zig** | `0.13.0` | C/C++ Cross-compiler |
| **GitHub CLI** | `2.66.1` | `gh` command |
| **Pixi** | `latest` | Package management |
| **Vercel CLI** | `latest` | Installed via npm |

### Linux / WSL Specific
| Tool | Version | Notes |
| :--- | :--- | :--- |
| **V4L2 Headers** | `latest` | `libv4l-dev`, `linux-libc-dev` |
| **AArch64 Compiler**| `13.3.rel1`| `aarch64-none-linux-gnu-gcc` |
| **ARMhf Compiler** | `13.3.rel1`| `arm-none-linux-gnueabihf-gcc` |

### System Checks
| Tool | Version | Notes |
| :--- | :--- | :--- |
| **Podman** | `system` | Checked only (manual install required) |
