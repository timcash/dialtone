# LLM Agent Workflow Guide

> **Note**: Many `dialtone-dev` commands are **aspirational** (not yet implemented). When a command doesn't exist, use manual git/go commands instead. Replace `<feature-branch-name>` with your actual branch name.

---

## About Dialtone

Dialtone is a robotic video operations network for cooperative human-AI robot control. Key technologies: Go, NATS messaging, Tailscale VPN, V4L2 cameras, MAVLink protocol.

---

## Workflow Stages

### 1. Plan Stage

1. Run `dialtone install` to verify dependencies are installed
2. Run `dialtone clone` to clone and verify the repository is up to date
3. Run `dialtone-dev branch <feature-branch-name>` to create or checkout the feature branch
4. Run `dialtone-dev plan <feature-branch-name>` to list plan sections or create a new plan file
5. Read `README.md` to get an overview of the system
6. Look for or create a plan file at `plan/plan-<feature-branch-name>.md`
7. If branch or plan file already exists, figure out how to continue the work
8. Review `docs/cli.md` to understand CLI commands
9. Create a draft PR: `dialtone-dev pull-request --draft --title "<feature-branch-name>" --body "Plan file"`

### 2. Development Stage (Iterative Loop)

1. Pick ONE test from the plan
2. Improve an existing test if possible, otherwise create a new one
3. Include logs and metrics as part of the test
4. Use the project logger from `src/logger.go`
5. Write or change code to pass the test
6. Run test: `dialtone-dev test <feature-branch-name>`
7. If PASS → commit and update plan. If FAIL → debug and retry
8. Update `README.md` and `docs/*` with any changes
9. Commit changes: `git add .` and `git commit -m "<message>"`
10. Add vendor docs to `docs/vendor/<vendor_name>.md` if needed
11. Mark completed tests with `[x]` in the plan file

### 3. Cleanup and Pull Request

1. Verify the branch only contains changes related to the feature
2. Stage and commit: `git add .` and `git commit -m "<feature-branch-name>: complete"`
3. Update PR: `dialtone-dev pull-request --title "<feature-branch-name>" --body "<summary>"`

---

## Test Rules

1. **Unit tests** — Simple tests that run locally without IO operations
2. **Integration tests** — Test 2 components together using `test_data/`
   - Example test_data: premade video file, MAVLink message file, known-correct response snapshot
3. **End-to-end tests** — Browser and CLI tests on a live system or simulator

---

## CLI Reference

### Implemented Commands (`dialtone`)

1. `dialtone install` — Install development dependencies
   - Example: `dialtone install`
2. `dialtone build` — Build web UI + binary
   - Example: `dialtone build`
   - Example: `dialtone build --podman`
   - Example: `dialtone build --arch arm64 --os linux --podman`
3. `dialtone deploy` — Send binary over SSH to a robot
   - Example: `dialtone deploy`
   - Example: `dialtone deploy --host tim@192.168.4.36 --port 22 --user tim --pass password`
4. `dialtone web` — Print the web dashboard URL
   - Example: `dialtone web`
5. `dialtone diagnostic` — Run system diagnostics on remote robot
   - Example: `dialtone diagnostic --host 192.168.4.36`
6. `dialtone env` — Write to the local `.env` file (no reading for security)
   - Example: `dialtone env TS_AUTHKEY tskey-auth-xxxxx`
7. `dialtone logs` — Tail remote execution logs via SSH
   - Example: `dialtone logs`
8. `dialtone start` — Stop any running server and start a new one
   - Example: `dialtone start`
9. `dialtone provision` — Generate a fresh Tailscale Auth Key and update `.env`
   - Requires `TS_API_KEY` in `.env` or environment
10. `dialtone clone` — Clone the repository to a local directory
   - Example: `dialtone clone ./dialtone
11. `dialtone env <var> <value>` — Write to the local `.env` file
   - Example: `dialtone env TS_AUTHKEY tskey-auth-xxxxx`

### Aspirational Commands (`dialtone-dev`)

1. `dialtone-dev branch <name>` — Create or checkout feature branch
   - Example: `dialtone-dev branch linux-wsl-camera-support`
2. `dialtone-dev test` — Run all tests in `test/` directory
   - Example: `dialtone-dev test`
3. `dialtone-dev test <name>` — Run tests in `test/<name>/` or create example test
   - Example: `dialtone-dev test linux-wsl-camera-support`
4. `dialtone-dev plan <name>` — List plan sections or create plan file
   - Example: `dialtone-dev plan linux-wsl-camera-support`
5. `dialtone-dev pull-request <name> <message>` — Create or update a PR
   - Example: `dialtone-dev pull-request linux-wsl-camera-support "Added V4L2 support"`

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
- [x] test_install_deps: Verify `dialtone install-deps --linux-wsl` creates ~/.dialtone_env
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

## When You Get Stuck

1. **Command not implemented?** → Use manual git/go commands
2. **Test failing repeatedly?** → Add debug logging, check assumptions
3. **Build failing?** → Check `docs/cli.md` for environment setup
4. **Unsure what to do next?** → Re-read the plan file, ask for clarification
5. **Need external docs?** → Add summary to `docs/vendor/<name>.md`

---

## Final Checklist

1. All plan tests marked complete with `[x]`
2. `go test -v ./test/...` passes
3. `dialtone build` succeeds
4. No secrets/keys in commits
5. README/docs updated if behavior changed
6. PR created/updated with summary
7. Plan file reflects final state
