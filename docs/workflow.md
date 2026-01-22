Development Workflow
```bash
# 1. Install local tools (Go, Node.js, Zig, etc.) for macOS ARM
./dialtone.sh install --macos-arm

# 2. Verify all required tools are correctly installed
./dialtone.sh install --check

# 3. Perform a full build including Web UI assets
# (Use --full to ensure web assets are rebuilt when changed)
./dialtone.sh build --full

# 4. Deploy the compiled binary and assets to the robot
./dialtone.sh deploy

# 5. Run health diagnostics (local & remote)
./dialtone.sh diagnostic

# 6. Stream remote logs to verify operational status
./dialtone.sh logs --remote
```



## Example Test File

File: `test/linux-wsl-camera-support/wsl_camera_unit_test.go`

```go
package test

import (
    "os"
    "os/exec"
    "testing"

    "dialtone/src"
)

func TestV4L2HeadersExist(t *testing.T) {
    headerPath := os.ExpandEnv("$HOME/.dialtone_env/usr/include/linux/videodev2.h")
    if _, err := os.Stat(headerPath); os.IsNotExist(err) {
        t.Fatalf("V4L2 header not found at %s", headerPath)
    }
    dialtone.LogInfo("V4L2 headers found")
}

func TestNativeBuildCompiles(t *testing.T) {
    cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
    cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("Build failed: %s\n%s", err, output)
    }
    dialtone.LogInfo("Native build succeeded")
}
```

---

## Code Style

Use linear pipelines, not nested pyramids.

```go
type RequestContext struct {
    CreatedAt       time.Time
    Src             string
    Dst             string
    AuthToken       string
    Database1Result string
    Database2Result string
    Error           error
}

func HandleRequest(ctx *RequestContext) *RequestContext {
    authResult := auth(ctx)
    if authResult == nil {
        ctx.Error = errors.New("auth failed")
        return ctx
    }

    ctx.Database1Result = database1(ctx, authResult)
    ctx.Database2Result = database2(ctx, ctx.Database1Result)
    logger(ctx, authResult, ctx.Database1Result, ctx.Database2Result)
    return ctx
}
```

**Rules:**
1. Use the project logger: `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal`
2. Prefer functions and structs over complicated patterns
3. Keep functions short and single-purpose
4. Name variables descriptively

---

## Logging

Always use the project logger from `src/logger.go`:

```go
dialtone.LogInfo("Starting camera capture")
dialtone.LogError("Failed to connect", err)
dialtone.LogFatal("Unrecoverable error", err)  // exits program
```
---

