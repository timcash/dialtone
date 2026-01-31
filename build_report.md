# Build System Report

## Entry points and command routing

- Primary entry point is the shell wrapper `./dialtone.sh`. It parses global flags, loads `.env`, ensures `DIALTONE_ENV` is set, and then runs the Go CLI via `go run src/cmd/dev/main.go`.

```1:40:dialtone.sh
#!/bin/bash
set -e
...
Usage: ./dialtone.sh <command> [options]
...
  build         Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64)
```

```99:302:dialtone.sh
if [ -z "$DIALTONE_ENV" ]; then
    echo "Error: DIALTONE_ENV is not set."
...
if [ -n "$GO_BIN" ] && [ -f "$GO_BIN" ]; then
    run_with_timeout "$GO_BIN" "${ARGS[@]}"
elif command -v go &> /dev/null; then
    echo "Using system Go..."
    run_with_timeout "go" "${ARGS[@]}"
else
    echo "Error: Go binary not found at $GO_BIN and system Go not found."
    exit 1
fi
```

- The `build` command is implemented in Go at `src/core/build/build.go` and is wired into the CLI wrapper at `src/core/build/cli/cli.go`.

```1:39:src/core/build/cli/cli.go
// Run handles the 'build' command
func Run(args []string) {
    if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
        printUsage()
        return
    }

    build.RunBuild(args)
}
```

```15:66:src/core/build/build.go
func RunBuild(args []string) {
    fs := flag.NewFlagSet("build", flag.ExitOnError)
    full := fs.Bool("full", false, "Build Web UI, local CLI, and ARM binary")
    local := fs.Bool("local", false, "Build natively on the local system")
    remote := fs.Bool("remote", false, "Build on remote robot via SSH")
    podman := fs.Bool("podman", false, "Force build using Podman")
    linuxArm := fs.Bool("linux-arm", false, "Cross-compile for 32-bit Linux ARM (armv7)")
    linuxArm64 := fs.Bool("linux-arm64", false, "Cross-compile for 64-bit Linux ARM (aarch64)")
    builder := fs.Bool("builder", false, "Build the dialtone-builder image for faster ARM builds")
...
}
```

## High-level build flow

### Default build

The default path builds the web UI (if missing) and then builds the CLI binary. It prefers Podman for cross-compilation when available, otherwise builds locally.

```52:66:src/core/build/build.go
isCrossBuild := arch != runtime.GOARCH || targetOS != runtime.GOOS

if *local || !hasPodman() {
    if isCrossBuild && !hasZig() && !*local {
        logger.LogFatal("Cross-compilation for %s/%s requires either Podman or Zig. Please install Podman (recommended) or ensure Zig is installed in your DIALTONE_ENV.", targetOS, arch)
    }
    buildLocally(targetOS, arch)
} else {
    compiler := "gcc-aarch64-linux-gnu"
    if arch == "arm" {
        compiler = "gcc-arm-linux-gnueabihf"
    }
    buildWithPodman(arch, compiler)
}
```

### Web UI build and verification

The web UI build is delegated to the `ui` plugin (`./dialtone.sh ui install/build`). After running, the build verifies that `src/core/web/dist/index.html` exists and is non-empty.

```74:112:src/core/build/build.go
func buildWebIfNeeded(force bool) {
    distIndexPath := filepath.Join("src", "core", "web", "dist", "index.html")
...
    runShell(".", "./dialtone.sh", "ui", "install")
...
    runShell(".", "./dialtone.sh", "ui", "build")
...
    if info, err := os.Stat(distIndexPath); os.IsNotExist(err) {
        logger.LogFatal("Web UI build failed: %s not found after build", distIndexPath)
    } else {
        logger.LogInfo("Web UI build complete (size: %d bytes)", info.Size())
    }
}
```

### Local build details

Local builds:
- force-enable CGO to support V4L2,
- prepend tools from `DIALTONE_ENV` to `PATH`,
- detect GNU cross-compilers or Zig,
- and fail if cross-compiling without a suitable compiler.

```115:215:src/core/build/build.go
func buildLocally(targetOS, targetArch string) {
...
    // For local builds, we enable CGO to support V4L2 drivers
    os.Setenv("CGO_ENABLED", "1")
...
    // Prepend dependencies to PATH (Go, Node, Zig, Pixi, GH)
    paths := []string{
        filepath.Join(depsDir, "go", "bin"),
        filepath.Join(depsDir, "node", "bin"),
        filepath.Join(depsDir, "zig"),
        filepath.Join(depsDir, "gh", "bin"),
        filepath.Join(depsDir, "pixi"),
    }
...
    if targetArch != runtime.GOARCH && !compilerFound {
        logger.LogFatal("Local cross-compilation for %s requested, but no suitable compiler (Zig or GNU Toolchain) was found in %s", targetArch, depsDir)
    }
}
```

### Podman build details

Podman builds run in `golang:1.25.5`, mount the repo and Go cache, and build to `bin/dialtone-{arch}`. If a `dialtone-builder` image exists, it skips compiler installation.

```253:304:src/core/build/build.go
func buildWithPodman(arch, compiler string) {
...
    baseImage := "docker.io/library/golang:1.25.5"
    installCmd := fmt.Sprintf("apt-get update && apt-get install -y %s && ", compiler)
...
    if hasImage("dialtone-builder") {
        logger.LogInfo("Using optimized 'dialtone-builder' image (skipping apt-get install)")
        baseImage = "dialtone-builder"
        installCmd = ""
    }
...
    buildCmd := []string{
        "run", "--rm",
        "-v", fmt.Sprintf("%s:/src:Z", cwd),
        "-v", "dialtone-go-build-cache:/root/.cache/go-build:Z",
...
        "bash", "-c", fmt.Sprintf("%sgo build -buildvcs=false -o bin/%s src/cmd/dialtone/main.go", installCmd, outputName),
    }
...
}
```

### Full build

Full builds force the web UI rebuild, build AI components via `./dialtone.sh ai build`, rebuild the CLI, and build ARM64 (local or Podman).

```307:328:src/core/build/build.go
func buildEverything(local bool) {
    logger.LogInfo("Starting Full Build Process...")
...
    buildWebIfNeeded(true)
...
    runShell(".", "./dialtone.sh", "ai", "build")
...
    BuildSelf()
...
    if local || !hasPodman() {
        buildLocally("linux", "arm64")
    } else {
        buildWithPodman("arm64", "gcc-aarch64-linux-gnu")
    }
...
}
```

## Prerequisite checks and installation verification

### Shell-level prerequisites

- `dialtone.sh` enforces `DIALTONE_ENV` and verifies/installs Go.
- It warns if no C compiler is found (CGO disabled for install path).

```188:241:dialtone.sh
if [ -z "$DIALTONE_ENV" ]; then
    echo "Error: DIALTONE_ENV is not set."
...
if [ "$DIALTONE_CMD" = "install" ]; then
...
    if ! command -v gcc &> /dev/null && ! command -v clang &> /dev/null; then
        echo ""
        echo "WARNING: No C compiler (gcc/clang) found."
...
        export CGO_ENABLED=0
    fi
...
    if [ -n "$DIALTONE_ENV" ] && [ ! -d "$DIALTONE_ENV/go" ]; then
        echo "Go not found in $DIALTONE_ENV/go. Installing..."
...
    fi
elif [ -n "$DIALTONE_ENV" ] && [ ! -f "$GO_BIN" ]; then
    echo "Error: Go not found in $DIALTONE_ENV/go."
    echo "Please run './dialtone.sh install' first to set up the environment."
    exit 1
fi
```

### Go-side dependency verification

`CheckInstall()` checks for toolchain components in `DIALTONE_ENV` and fails if any mandatory dependency is missing. It also reports optional items like Podman and Vercel CLI.

```578:674:src/core/install/install.go
func CheckInstall(depsDir string) {
    logger.LogInfo("Checking dependencies in %s...", depsDir)
...
    // 1. Go
    goBin := filepath.Join(depsDir, "go", "bin", "go")
...
    // 2. Node.js
    nodeBin := filepath.Join(depsDir, "node", "bin", "node")
...
    // 2.2 GitHub CLI
    ghBin := filepath.Join(depsDir, "gh", "bin", "gh")
...
    // 2.3 Pixi
    pixiBin := filepath.Join(depsDir, "pixi", "pixi")
...
    // 2.5 Zig
    zigBin := filepath.Join(depsDir, "zig", "zig")
...
    // 2.6 Podman check (System-level)
    if _, err := exec.LookPath("podman"); err == nil {
        logger.LogInfo("Podman (system) is present")
    } else if runtime.GOOS != "darwin" {
        logger.LogInfo("Podman is MISSING (Optional, recommended for ARM builds)")
    }
...
    // 2.7 ARM Cross-Compilers (Local)
...
    // 3. V4L2 Header (Linux only)
...
    if missing == 0 {
        logger.LogInfo("All dependencies are present.")
    } else {
        logger.LogFatal("%d dependencies are missing. Run './dialtone.sh install' to fix.", missing)
    }
}
```

### Cross-compilation requirements

Cross-compilation is blocked unless Podman or Zig is present. Local cross-builds also require a compiler in `DIALTONE_ENV`.

```54:57:src/core/build/build.go
if isCrossBuild && !hasZig() && !*local {
    logger.LogFatal("Cross-compilation for %s/%s requires either Podman or Zig. Please install Podman (recommended) or ensure Zig is installed in your DIALTONE_ENV.", targetOS, arch)
}
```

```213:215:src/core/build/build.go
if targetArch != runtime.GOARCH && !compilerFound {
    logger.LogFatal("Local cross-compilation for %s requested, but no suitable compiler (Zig or GNU Toolchain) was found in %s", targetArch, depsDir)
}
```

## Error reporting and failure behavior

### Shell-level failures

The wrapper uses `set -e`, so any non-zero exit aborts the script. It also exits with explicit error messages for missing prerequisites.

```1:3:dialtone.sh
#!/bin/bash
set -e
```

```237:241:dialtone.sh
elif [ -n "$DIALTONE_ENV" ] && [ ! -f "$GO_BIN" ]; then
    echo "Error: Go not found in $DIALTONE_ENV/go."
    echo "Please run './dialtone.sh install' first to set up the environment."
    exit 1
fi
```

### Structured logging

The Go logger writes formatted messages to stdout and `dialtone.log`, and `LogFatal` exits with status 1.

```48:102:src/core/logger/logger.go
func LogMsgWithDepth(depth int, level string, format string, args ...interface{}) {
...
    out := fmt.Sprintf("[%s | %s | %s] %s\n", timestamp, level, details, msg)
    fmt.Print(out)
    if logFile != nil {
        logFile.WriteString(out)
    }
}
...
func LogFatal(format string, args ...interface{}) {
    LogMsgWithDepth(3, "FATAL", format, args...)
    os.Exit(1)
}
```

### Build-time failures

Build steps use `LogFatal` on key error paths (missing output, filesystem failures, Podman failures, failed command execution).

```124:131:src/core/build/build.go
if err := os.MkdirAll("bin", 0755); err != nil {
    logger.LogFatal("Failed to create bin directory: %v", err)
}
```

```107:111:src/core/build/build.go
if info, err := os.Stat(distIndexPath); os.IsNotExist(err) {
    logger.LogFatal("Web UI build failed: %s not found after build", distIndexPath)
} else {
    logger.LogInfo("Web UI build complete (size: %d bytes)", info.Size())
}
```

```295:302:src/core/build/build.go
if err := cmd.Run(); err != nil {
    logger.LogFatal("Podman build failed: %v", err)
}
```

```364:371:src/core/build/build.go
func runShell(dir string, name string, args ...string) {
...
    if err := cmd.Run(); err != nil {
        logger.LogFatal("Command failed in %s: %v %v: %v", dir, name, args, err)
    }
}
```

## Build outputs

- Local/native binary: `bin/dialtone` (or `bin/dialtone.exe` on Windows).
- Cross-compiled binaries: `bin/dialtone-arm` or `bin/dialtone-arm64`.
- Web UI output: `src/core/web/dist/index.html`.

```229:240:src/core/build/build.go
binaryName := "dialtone"
if targetOS == "linux" && targetArch != runtime.GOARCH {
    binaryName = fmt.Sprintf("dialtone-%s", targetArch)
} else if targetOS == "linux" {
    binaryName = "dialtone"
} else if targetOS == "windows" {
    binaryName = "dialtone.exe"
}

outputPath := filepath.Join("bin", binaryName)
```
