---
trigger: always_on
---

# Ticket Improvement Guide for LLMs

This guide outlines how to build robust, testable features for the Dialtone project.

## Core Philosophy
1. **Test-First Development**: Always define subtasks that can be verified with Go tests.
2. **Atomic Steps**: Break complex features into small, testable units.
3. **Implicit Context**: Assume the LLM has no prior knowledge. Include explicit CLI commands.

## How to use the `dialtone.sh` CLI for development

### 1. Installation & Setup
```bash
./dialtone.sh install             # Install tools
./dialtone.sh install --check     # Verify installation
```

### 2. Ticket Lifecycle
```bash
./dialtone.sh ticket start <name> # Start work (branch + scaffolding)
./dialtone.sh ticket done <name>  # Final verification before submission
```

### 3. Running Tests
Tests are your primary feedback loop.
- **Ticket Tests**: `./dialtone.sh ticket test <ticket-name>` (Runs tests in `tickets/<name>/test/`)
- **Plugin Tests**: `./dialtone.sh plugin test <plugin-name>` (Runs tests in `src/plugins/<name>/test/`)
- **Feature Tests**: `./dialtone.sh test <feature-name>` (Discovery across core, plugins, and tickets)
- **All Tests**: `./dialtone.sh test`

### 4. Build & Deploy
```bash
./dialtone.sh build --full  # Build Web UI + local CLI + robot binary
./dialtone.sh deploy        # Push to remote robot
./dialtone.sh diagnostic    # Run health checks
./dialtone.sh logs --remote # Stream remote logs
```

### 5. GitHub & Pull Requests
```bash
./dialtone.sh github pr           # Create or update a pull request
./dialtone.sh github pr --draft   # Create as a draft
./dialtone.sh github check-deploy # Verify Vercel deployment status
```

## Code Style: Linear Pipelines
Avoid "pyramid" nesting. Keep the main path of execution on the left margin.

```go
func HandleRequest(ctx *RequestContext) *RequestContext {
    authResult := auth(ctx)
    if authResult == nil {
        ctx.Error = errors.New("auth failed")
        return ctx
    }

    ctx.Database1Result = database1(ctx, authResult)
    ctx.Database2Result = database2(ctx, ctx.Database1Result)
    return ctx
}
```

**Style Rules:**
1. Use the project logger: `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal`.
2. Prefer functions and structs over complicated patterns.
3. Keep functions short and single-purpose.
4. Name variables descriptively.

## Logging
Always use the project logger from `src/logger.go`:
```go
dialtone.LogInfo("Starting capture")
dialtone.LogError("Failed to connect: %v", err)
dialtone.LogFatal("Unrecoverable error: %v", err) // Exits program
```

## Example #SUBTASK Format
```markdown
## #SUBTASK: Environment Check
- description: Verify V4L2 headers exist in the environment.
- test: Create `TestV4L2Headers` in `tickets/<name>/test/unit_test.go` using `os.Stat`.
- status: todo
```