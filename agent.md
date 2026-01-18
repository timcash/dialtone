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
export DIALTONE_ENV="~/dialtone_env"

# For bootstrapping only - Install go for linux and macos
./setup.sh

# For bootstrapping only - Install go for windows
./setup.ps1

# Environment setup if no previous install or cli binary is avaible to help with build
export CC="${DIALTONE_ENV}/zig/zig cc -target x86_64-linux-gnu"
export CGO_ENABLED=1
export CGO_CFLAGS="-I${DIALTONE_ENV}/usr/include -I${DIALTONE_ENV}/usr/include/x86_64-linux-gnu"
export PATH="${DIALTONE_ENV}/go/bin:${DIALTONE_ENV}/node/bin:$PATH"

# Install dependencies into ~/.dialtone_env (Go, Node, Zig, V4L2 headers)
go run dialtone-dev.go install --linux-wsl

# Perform a native build (includes Web UI and Camera support)
go run dialtone-dev.go build --local

# Start the node locally
go run dialtone.go start --local
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
   - Use Puppeteer for live site verification: `dialtone-earth/test/live_test.ts`

---

## CLI Reference

### Production CLI (`dialtone`)

1. `go run dialtone.go start` — Stop any running server and start a new one
   - Example: `go run dialtone.go start --local-only`

### Development CLI (`dialtone-dev.go`)

1. `go run dialtone-dev.go install` — Install development dependencies
   - Example: `go run dialtone-dev.go install --linux-wsl` (for Linux/WSL x86_64)
2. `go run dialtone-dev.go build` — Build web UI + binary
   - Example: `go run dialtone-dev.go build --local` (Native build)
   - Example: `go run dialtone-dev.go build --full` (Force full rebuild of everything)
   - Example: `go run dialtone-dev.go build --remote` (Build on remote robot via SSH)
3. `go run dialtone-dev.go deploy` — Send binary over SSH to a robot
   - Example: `go run dialtone-dev.go deploy`
4. `go run dialtone-dev.go clone` — Clone the repository to a local directory
   - Example: `go run dialtone-dev.go clone ./dialtone`
5. `go run dialtone-dev.go sync-code` — Sync source code to remote robot
   - Example: `go run dialtone-dev.go sync-code`
6. `go run dialtone-dev.go ssh` — SSH tools (upload, download, cmd)
   - Example: `go run dialtone-dev.go ssh download /tmp/log.txt`
7. `go run dialtone-dev.go provision` — Generate a fresh Tailscale Auth Key and update `.env`
   - Example: `go run dialtone-dev.go provision`
8. `go run dialtone-dev.go logs` — Tail remote execution logs via SSH
   - Example: `go run dialtone-dev.go logs`
9. `go run dialtone-dev.go diagnostic` — Run system diagnostics
   - Example: `go run dialtone-dev.go diagnostic --remote`
10. `go run dialtone-dev.go env <var> <value>` — Write to the local `.env` file
    - Example: `go run dialtone-dev.go env TS_AUTHKEY tskey-auth-xxxxx`
11. `go run dialtone-dev.go branch <name>` — Create or checkout feature branch
    - Example: `go run dialtone-dev.go branch my-feature`
12. `go run dialtone-dev.go plan <name>` — List plan sections or create plan file
    - Example: `go run dialtone-dev.go plan my-feature`
13. `go run dialtone-dev.go test [name]` — Run tests (all or for specific feature)
    - Example: `go run dialtone-dev.go test my-feature`
14. `go run dialtone-dev.go pull-request [options]` — Create or update a PR
    - Example: `go run dialtone-dev.go pull-request --draft`
15. `go run dialtone-dev.go issue <subcmd>` — Manage GitHub issues
    - Example: `go run dialtone-dev.go issue view 20`
16. `go run dialtone-dev.go www <subcmd>` — Manage public webpage (Vercel wrapper)
    - Example: `go run dialtone-dev.go www publish`

---

## Remote Robot Deployment

For deploying to a Raspberry Pi or other remote robot, use the following workflow:

### 1. Configure `.env`
Ensure your local `.env` file contains the correct credentials and host information:
```ini
ROBOT_HOST=192.168.4.36
ROBOT_USER=tim
ROBOT_PASSWORD=password
REMOTE_DIR_SRC=/home/tim/dialtone_src
REMOTE_DIR_DEPLOY=/home/tim/dialtone_deploy
DIALTONE_HOSTNAME=drone_1
MAVLINK_ENDPOINT=udp:0.0.0.0:14550
```

### 2. Synchronization & Building
The deployment process involves two main steps:
1. **Sync Code**: `go run dialtone-dev.go sync-code`
   - Uploads `src/`, `go.mod`, `go.sum`, and web assets to the robot.
2. **Remote Build**: `go run dialtone-dev.go build --remote`
   - Compiles the binary on the robot to ensure ARM compatibility and correct CGO linking.
   - Built assets are stored in `src/web_build`.

### 3. Deploy & Verify
- **Deploy**: `go run dialtone-dev.go deploy`
  - Uploads the compiled binary and starts it as a system service.
- **Logs**: `go run dialtone-dev.go logs`
  - Tails the remote logs to verify the service started correctly.

### Web UI Clarification
- **Robot Interface ([src/web](file:///home/user/dialtone/src/web))**: The operational dashboard embedded in the `dialtone` binary via `src/web_build`.
- **Marketing Site ([dialtone-earth](file:///home/user/dialtone/dialtone-earth))**: A standalone Next.js app for public RSI, published via `dialtone-dev www publish` (typically to Vercel).

---

## Testing the Webpage

The `dialtone-earth` project includes Puppeteer-based live site verification.

1. **Environment Setup**: 
   - Ensure Node.js v22+ is installed (use `nvm install 22`).
   - Install system dependencies for Chrome (see `walkthrough.md` for the list of `apt` packages).
2. **Running Tests**:
   - Run all tests (includes Go + Web): `go run dialtone-dev.go test`
   - Run web tests specifically: `go run dialtone-dev.go test www`
3. **Test Script**: Located at `dialtone-earth/test/live_test.ts`.

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
