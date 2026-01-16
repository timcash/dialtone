# Plan: improve-cli-build-commands

## Goal
Simplify CLI commands to use intuitive names like `install`, `build`, `deploy`, `dev` with proper help and options for each command.

## Tests
- [x] test_install_auto_detect: Verify `dialtone install` auto-detects OS/arch and installs dependencies
- [x] test_install_macos_arm: Verify `dialtone install --macos-arm` installs Go, Node.js, Zig for Apple Silicon
- [x] test_install_linux_wsl: Verify `dialtone install --linux-wsl` installs deps for Linux x86_64
- [x] test_install_skip_existing: Verify install skips already-installed dependencies
- [x] test_install_deps_directory: Verify deps directory structure exists
- [x] test_install_go_version: Verify Go 1.25.x is installed
- [x] test_install_node_version: Verify Node.js v22.x is installed
- [x] test_install_zig_version: Verify Zig 0.13.x is installed
- [x] test_build_local: Verify `dialtone build --local` builds native binary
- [x] test_build_local_binary_exists: Verify binary is created in bin/
- [x] test_build_local_architecture: Verify binary matches current architecture
- [x] test_build_help: Verify `dialtone build --help` shows usage information
- [x] test_install_help: Verify `dialtone install --help` shows usage information
- [x] test_deploy_help: Verify `dialtone deploy --help` shows usage information
- [ ] test_build_podman: Verify `dialtone build --podman` builds ARM64 via container
- [ ] test_full_build: Verify `dialtone full-build` builds web + CLI + binary

## Completed Changes

### Install Command (`dialtone install`)
- Renamed from `install-deps` to `install`
- Added auto-detection of OS/architecture (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64)
- Added `--macos-arm` flag for explicit macOS Apple Silicon install
- Added `--linux-wsl` flag for explicit Linux/WSL x86_64 install
- Checks if dependencies already installed and skips if present
- Installs Go 1.25.5, Node.js 22.13.0, Zig 0.13.0 to `~/.dialtone_env`
- Added `--help` flag with detailed usage information

### Build Command (`dialtone build`)
- `--local` flag for native build using local toolchain
- Uses `~/.dialtone_env` deps if available
- Falls back to Podman if available and `--local` not specified
- Added `--help` flag with detailed usage information

### Deploy Command (`dialtone deploy`)
- Added `--help` flag with detailed usage information

### Documentation
- Updated `agent.md` with new command names
- Updated `docs/cli.md` with install/build options
- Added macOS ARM development section to docs

## Remaining Work
- [x] Add `--help` flag to all commands with detailed usage
- [ ] Implement `dialtone dev` command for development mode
- [ ] Add `--arch` and `--os` flags to build command for cross-compilation
- [ ] Test Podman build path

## Notes
- Dependencies installed to `~/.dialtone_env` (no sudo required)
- Zig used as portable C compiler for CGO cross-compilation
- macOS uses AVFoundation for camera (no V4L2)
- Linux uses V4L2 headers extracted from Ubuntu packages

## Progress Log
- 2026-01-16: Renamed install-deps to install, added auto-detect, added macOS ARM support
- 2026-01-16: Tested install and build on macOS ARM64 - both working
- 2026-01-16: Created plan file and branch
- 2026-01-16: Created unit tests in test/improve-cli-build-commands/unit_test.go - all 7 tests PASS
- 2026-01-16: Added --help flags to build, install, deploy commands - all 10 tests PASS
