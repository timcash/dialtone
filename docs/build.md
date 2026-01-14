# Build & Deployment System

To ensure consistent builds for ARM64 robots from any development machine (Windows/Mac/Linux), Dialtone uses a containerized build loop.

## Containerized Build Loop (Podman)

- **Cross-Compilation**: The `dialtone` CLI uses **Podman** to spin up a specialized Linux container (`golang:1.25.5`) with the `aarch64-linux-gnu-gcc` toolchain.
- **CGO Support**: This enables building the V4L2 camera drivers (which require Linux headers) correctly for the target platform even when developing on Windows.
- **Asset Embedding**: The `dialtone` CLI compiles the Vite frontend and uses `go:embed` to package the entire UI into the final binary during `full-build`.
- **Library Architecture**: The core logic resides in `src/` as `package dialtone`, allowing for clean imports in both the CLI entry point (`dialtone.go`) and the comprehensive test suite in `test/`.

## Deployment Commands (Unified CLI)

Deployment is handled directly through the `dialtone` binary:

```bash
# Full sequence: build manager, build app, and deploy
go build -o bin/dialtone.exe .
bin/dialtone.exe full-build
bin/dialtone.exe deploy

# Flags (-host, -user, -pass) are optional if configured in .env:
# bin/dialtone.exe deploy -host 192.168.4.36 -user tim -pass raspberry

# Tail remote execution logs via SSH
bin/dialtone.exe logs
```

## Manual Build Steps

If you need to build components individually:

### 1. Build the Web Interface
```bash
cd src/web
npm install
npm run build
```

### 2. Build the ARM64 Binary (via Podman)
```bash
go build -o bin/dialtone.exe .
bin/dialtone.exe build
```

### 3. Simple Build (Go only)
If you are developing locally or don't need ARM64/CGO cross-compilation:
```powershell
./build.ps1
```
or 
```bash
./build.sh
```
