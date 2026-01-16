# LLM Agent Workflow Guide
---

## About Dialtone

Dialtone is a robotic video operations network for cooperative human-AI robot control. Key technologies: Go, NATS messaging, Tailscale VPN, V4L2 cameras, MAVLink protocol.

---

## Quick Start (WSL/Linux No-Sudo)

The fastest way to get started on WSL or Linux without administrative privileges:
```
# Clone the repo
git clone https://github.com/timcash/dialtone.git
export DAILTONE_ENV="~/dialtone_env"

# For bootstrapping only - Install go for linux and macos
./setup.sh

# For bootstrapping only - Install go for windows
./setup.ps1

# Environment setup if no previous install or cli binary is avaible to help with build
export CC="${DAILTONE_ENV}/zig/zig cc -target x86_64-linux-gnu"
export CGO_ENABLED=1
export CGO_CFLAGS="-I${DAILTONE_ENV}/usr/include -I${DAILTONE_ENV}/usr/include/x86_64-linux-gnu"
export PATH="${DAILTONE_ENV}/go/bin:${DAILTONE_ENV}/node/bin:$PATH"

# Install dependencies into ~/.dialtone_env (Go, Node, Zig, V4L2 headers)
go run dialtone-dev install --linux-wsl

# Perform a native full-build (includes Web UI and Camera support)
go run dialtone-dev build --local

# Start the node locally
go run ./bin/dialtone start --local
```

---

## Workflow Stages

### 1. Plan Stage

1. Run `go run dialtone-dev.go install` to verify dependencies are installed
2. Run `go run dialtone-dev.go clone` to clone and verify the repository is up to date
3. Run `go run dialtone-dev.go branch <feature-branch-name>` to create or checkout the feature branch
4. Run `go run dialtone-dev.go plan <feature-branch-name>` to list plan sections or create a new plan file
5. Read `README.md` to get an overview of the system
6. Look for the plan file at `plan/plan-<feature-branch-name>.md`
7. Write your plan into it as you go with a test list. then mark off each test when it passes
8. If the branch, plan or tests already exists for `<feature-branch-name>` you should continue the work
9. Review `docs/cli.md` to understand CLI commands
10. Create a draft PR: `go run dialtone-dev.go pull-request --draft --title "<feature-branch-name>" --body "Plan file"`

### 2. Development Stage (Iterative Loop)

1. Pick ONE test from the plan
2. Improve an existing test if possible, otherwise create a new one
3. Include logs and metrics as part of the test
4. Use the project logger from `src/logger.go`
5. Write or change code to pass the test
6. Run test: `go run dialtone-dev.go test <feature-branch-name>`
7. If PASS → commit and update plan. If FAIL → debug and retry
8. Update `README.md` and `docs/*` with any changes
9. Commit changes: `git add .` and `git commit -m "<message>"`
10. Add vendor docs to `docs/vendor/<vendor_name>.md` if needed
11. Mark completed tests with `[x]` in the plan file

### 3. Cleanup and Pull Request

1. Verify the branch only contains changes related to the feature
2. Stage and commit: `git add .` and `git commit -m "<feature-branch-name>: complete"`
3. Update PR: `go run dialtone-dev.go pull-request --title "<feature-branch-name>" --body "<summary>"`

---

## Test Rules

1. **Unit tests** — Simple tests that run locally without IO operations
2. **Integration tests** — Test 2 components together using `test_data/` possibly using IO between the two components 
   - Example test_data: premade video file, MAVLink message file, known-correct response snapshot
3. **End-to-end tests** — Browser and CLI tests on a live system or simulator

---

## CLI Reference

### Implemented Commands (`dialtone`)

1. `go run dialtone-dev.go install` — Install development dependencies
   - Example: `go run dialtone-dev.go install --linux-wsl` (for Linux/WSL x86_64)
   - Example: `go run dialtone-dev.go install --macos-arm` (for macOS Apple Silicon)
   - Example: `go run dialtone-dev.go install --macos-intel` (for macOS Intel)
   - Example: `go run dialtone-dev.go install --linux-arm64` (for Linux ARM64)
2. `go run dialtone-dev.go build` — Build web UI + binary
   - Example: `go run dialtone-dev.go build`
   - Example: `go run dialtone-dev.go build --podman`
   - Example: `go run dialtone-dev.go build --arch arm64 --os linux --podman`
3. `go run dialtone-dev.go deploy` — Send binary over SSH to a robot
   - Example: `go run dialtone-dev.go deploy` to use .env for connection details
   - Example: `go run dialtone-dev.go deploy --host tim@192.168.4.36 --port 22 --user tim --pass password` to override .env values
4. `go run dialtone-dev.go web` — Print the robot web dashboard URL
   - Example: `go run dialtone-dev.go web`
5. `go run dialtone-dev.go diagnostic` — Run system diagnostics on remote robot
   - Example: `go run dialtone-dev.go diagnostic --host 192.168.4.36`
6. `go run dialtone-dev.go env` — Write to the local `.env` file (no reading for security)
   - Example: `go run dialtone-dev.go env TS_AUTHKEY tskey-auth-xxxxx`
7. `go run dialtone-dev.go logs` tail the local logs
   - Example: `go run dialtone-dev.go logs --remote` — Tail remote execution logs via SSH and .env settings
8. `go run dialtone-dev.go start` — Stop any running server and start a new one
   - Example: `go run dialtone-dev.go start`
9. `go run dialtone-dev.go provision` — Generate a fresh Tailscale Auth Key and update `.env`
   - Requires `TS_API_KEY` in `.env` or environment
10. `go run dialtone-dev.go clone` — Clone the repository to a local directory
   - Example: `go run dialtone-dev.go clone ./dialtone
11. `go run dialtone-dev.go env <var> <value>` — Write to the local `.env` file
   - Example: `go run dialtone-dev.go env TS_AUTHKEY tskey-auth-xxxxx`
12. `go run dialtone-dev.go branch <name>` — Create or checkout feature branch
   - Example: `go run dialtone-dev.go branch linux-wsl-camera-support`
13. `go run dialtone-dev.go test` — Run all tests in `test/` directory
   - Example: `go run dialtone-dev.go test`
14. `go run dialtone-dev.go test <name>` — Run tests in `test/<name>/` or create example test
   - Example: `go run dialtone-dev.go test linux-wsl-camera-support`
15. `go run dialtone-dev.go plan <name>` — List plan sections or create plan file
   - Example: `go run dialtone-dev.go plan linux-wsl-camera-support`
16. `go run dialtone-dev.go pull-request <name> <message>` — Create or update a PR
   - Example: `go run dialtone-dev.go pull-request linux-wsl-camera-support "Added V4L2 support"`
17. `go run dialtone-dev.go issue <subcmd>` — Manage GitHub issues
   - Subcommands: `list`, `add`, `comment`, `view`
   - Example: `go run dialtone-dev.go issue view 10`
   - Example: `go run dialtone-dev.go issue add --title "Bug" --body "Desc" --label "bug"`
18. `go run dialtone-dev.go www <subcmd>` — Manage public webpage (Vercel wrapper/pass-through)
   - Subcommands: `publish`, `logs`, `domain`, `login` (unrecognized commands pass through to Vercel)
   - Example: `go run dialtone-dev.go www publish`

---

## File Conventions

1. **Plan files**: `plan/plan-<feature-branch-name>.md`
2. **Feature tests**: `test/<feature-branch-name>/`
   - `<feature>_unit_test.go`
   - `<feature>_integration_test.go`
   - `<feature>_end_to_end_test.go`
3. **Vendor docs**: `docs/vendor/<vendor_name>.md`

---

## Example Plan File

File: `plan/plan-linux-wsl-camera-support.md`

```markdown
# Plan: linux-wsl-camera-support

## Goal
Enable camera support on Linux/WSL without requiring sudo for development.

## Tests
- [x] test_install_deps: Verify `dialtone install --linux-wsl` creates ~/.dialtone_env
- [x] test_v4l2_headers: Verify V4L2 headers are extracted and accessible
- [ ] test_native_build: Verify `dialtone build` compiles with CGO_ENABLED=1
- [ ] test_camera_open: Verify camera device can be opened on WSL
- [ ] test_frame_capture: Verify a frame can be captured from /dev/video0

## Notes
- Using Zig as cross-compiler for CGO
- V4L2 headers extracted from Ubuntu .deb without root
- Camera device must be passed through from Windows host

## Blocking Issues
- None

## Progress Log
- 2026-01-15: Created plan, implemented install-deps command
- 2026-01-16: V4L2 headers working, native build compiles
```

---

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

## Final Checklist

1. All plan tests marked complete with `[x]`
3. `go run dialtone-dev.go test` passes
4. `go run dialtone-dev.go build` succeeds
5. No secrets/keys in commits look for any way they can leak as a result of these changes
6. README/docs updated if behavior changed
7. PR created/updated with summary
8. Plan file reflects final state
