# LLM Agent Workflow Guide

> **Note**: This document helps LLM agents navigate the Dialtone codebase. Many CLI commands described here are **aspirational** and may not yet be implemented. When a command doesn't exist, note it and proceed with manual steps.

## About Dialtone
Dialtone is a robotic video operations network for cooperative human-AI robot control. Key technologies: Go, NATS messaging, Tailscale VPN, V4L2 cameras, MAVLink protocol.

---

## Two CLI Tools

| Tool | Purpose | Status |
|------|---------|--------|
| `dialtone` | Production CLI for build, deploy, diagnostics | Partially implemented |
| `dialtone-dev` | Development CLI for plans, tests, branches | Aspirational |

---

## Workflow Template

Use this workflow for **every feature or bugfix**:

### Phase 1: Setup

```bash
# 1. Verify dependencies
dialtone install

# 2. Clone and sync repo
dialtone clone

# 3. Create or checkout feature branch
dialtone-dev branch <feature-branch-name>

# 4. Check for existing plan
dialtone-dev plan list <feature-branch-name>
```

**First steps:**
1. Read `README.md` for project overview
2. Read `docs/cli.md` for CLI command reference
3. Look for existing plan at `plan/plan-<feature-branch-name>.md`
4. If plan exists → continue where previous work left off
5. If no plan → create one with tests and checkpoints

### Phase 2: Plan File

Create or update `plan/plan-<feature-branch-name>.md`:

```markdown
# Plan: <feature-branch-name>

## Goal
<one sentence describing the objective>

## Tests
- [ ] test_1: <description>
- [ ] test_2: <description>

## Notes
<implementation details, blockers, decisions>

## Progress Log
- YYYY-MM-DD: <what was done>
```

**Early PR:** Create a draft PR with just the plan file:
```bash
gh pr create --draft --title "<feature-branch-name>" --body "Plan file for <feature-branch-name>"
```

### Phase 3: Development Loop

Iterate in small steps:

```
┌─────────────────────────────────────────────────┐
│  1. Pick ONE test from the plan                 │
│  2. Write or improve the test                   │
│  3. Write minimal code to pass the test         │
│  4. Run test: go test -v ./test/<test_file>.go  │
│  5. If PASS → commit and update plan            │
│  6. If FAIL → debug and retry step 3            │
│  7. Repeat until all tests pass                 │
└─────────────────────────────────────────────────┘
```

**CLI shortcuts (aspirational):**
```bash
dialtone-dev create-test <feature-branch-name>   # scaffold a test
dialtone-dev run-test <feature-branch-name>      # run tests for feature
dialtone-dev plan add <feature-branch-name>      # add test to plan
dialtone-dev plan remove <feature-branch-name>   # remove test from plan
```

**Commit often:**
```bash
git add .
git commit -m "<feature-branch-name>: <brief description>"
```

### Phase 4: Verification

Before finalizing:

1. **Build check:** `go run . full-build -local`
2. **All tests pass:** `go test -v ./test/...`
3. **Security audit:** Search for leaked keys/passwords
4. **Docs updated:** Update `README.md` and `docs/*` if behavior changed
5. **Plan file updated:** Mark completed tests with `[x]`

### Phase 5: Cleanup & PR

```bash
# Ensure clean branch
git status

# Stage and commit final changes
git add .
git commit -m "<feature-branch-name>: complete"

# Update PR
gh pr edit --title "<feature-branch-name>" --body "<summary of changes>"

# Or use aspirational CLI
dialtone-dev pull-request <feature-branch-name> "<message>"
```

```
== END: FEATURE/<feature-branch-name> ==
```

---

## dialtone CLI Reference (Implemented)

```bash
dialtone install                        # Install development dependencies
dialtone build                          # Build ARM64 binary
dialtone build -local                   # Build for current system
dialtone full-build                     # Build web UI + binary (Podman)
dialtone full-build -local              # Build web UI + binary (native)
dialtone deploy                         # Deploy to robot via SSH
dialtone web                            # Print web dashboard URL
dialtone diagnostic --host <url>        # Run diagnostics on remote robot
dialtone env <var> <value>              # Write to local .env file
dialtone logs                           # Tail remote logs via SSH
```

---

## dialtone-dev CLI Reference (Aspirational)

```bash
dialtone-dev branch <name>              # Create/checkout feature branch
dialtone-dev create-test <name>         # Scaffold a new test
dialtone-dev run-test <name>            # Run tests for a feature
dialtone-dev create-plan <name>         # Create a plan file
dialtone-dev pull-request <name> <msg>  # Create/update PR

# Plan management
dialtone-dev plan list <name>           # List tests in plan
dialtone-dev plan add <name>            # Add test to plan
dialtone-dev plan remove <name>         # Remove test from plan
dialtone-dev plan clear <name>          # Clear all tests
dialtone-dev plan merge <name>          # Merge feature into main
```

---

## File Conventions

| Type | Location | Naming |
|------|----------|--------|
| Plan files | `plan/` | `plan-<feature-branch-name>.md` |
| Unit tests | `test/unit/` | `<feature>_test.go` |
| Integration tests | `test/integration/` | `<feature>_test.go` |
| End-to-end tests | `test/end_to_end/` | `<feature>_test.go` |
| Feature tests | `test/` | `<feature>_test.go` |
| Vendor docs | `docs/vendor/` | `<vendor_name>.md` |

---

## Code Style

**Principle:** Simple pipelines, not nested pyramids.

```go
// Good: Linear flow with explicit context passing
type RequestContext struct {
    CreatedAt       time.Time
    UpdatedAt       time.Time
    Src             string
    Dst             string
    AuthToken       string
    Database1Result string
    Database2Result string
    LoggerResult    string
    Error           error
}

func HandleRequest(ctx *RequestContext) *RequestContext {
    authResult := auth(ctx)
    if authResult == nil {
        ctx.Error = errors.New("auth failed")
        return ctx
    }
    
    db1Result := database1(ctx, authResult)
    db2Result := database2(ctx, db1Result)
    logResult := logger(ctx, authResult, db1Result, db2Result)
    
    ctx.Database1Result = db1Result
    ctx.Database2Result = db2Result
    ctx.LoggerResult = logResult
    return ctx
}
```

**Rules:**
- Use the project logger: `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal`
- Avoid over-abstraction: prefer functions and structs over interfaces
- Keep functions short and single-purpose
- Name variables descriptively

---

## Logging

Always use the project logger from `src/logger.go`:

```go
import "dialtone/src"

dialtone.LogInfo("Starting camera capture")
dialtone.LogError("Failed to connect", err)
dialtone.LogFatal("Unrecoverable error", err)  // exits program
```

---

## When You Get Stuck

1. **Command not implemented?** → Note it, use manual git/go commands
2. **Test failing repeatedly?** → Add debug logging, check assumptions
3. **Build failing?** → Check `docs/cli.md` for environment setup
4. **Unsure what to do next?** → Re-read the plan file, ask user for clarification
5. **Need external docs?** → Add summary to `docs/vendor/<name>.md`

---

## Checklist Before Finishing

- [ ] All plan tests marked complete
- [ ] `go test -v ./test/...` passes
- [ ] `go run . full-build -local` succeeds
- [ ] No secrets/keys in commits
- [ ] README/docs updated if needed
- [ ] PR created/updated with summary
- [ ] Plan file reflects final state
