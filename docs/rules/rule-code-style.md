---
trigger: always_on
---
# Code Style: Linear Pipelines
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

## Style Rules
1. Use the project logger: `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal`.
2. Prefer functions and structs over complicated patterns.
3. Keep functions short and single-purpose.
4. Name variables descriptively.

# Logging
Always use the project logger from `src/logger.go`:
```go
dialtone.LogInfo("Starting capture")
dialtone.LogError("Failed to connect: %v", err)
dialtone.LogFatal("Unrecoverable error: %v", err) // Exits program
```