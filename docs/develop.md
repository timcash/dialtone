# Development Workflow

## TDD for AI Agents

1. Checkout main branch: `git checkout main`
2. Pull the latest changes: `git pull origin main`
3. Create a new branch for your feature or bugfix: `git checkout -b feature-name`
4. Run the local tests
5. Create a new test if you are adding a feature or fixing a bug
6. Add code or refactor to make the test pass
7. Verify the new binary builds
8. Update the README and docs if anything changed
9. Deploy to a remote robot
10. Commit your changes

When adding features or fixing bugs, follow this Test-Driven Development (TDD) loop:

1. **Create Test**: Add a test in the appropriate `test/` subdirectory.
2. **Implement**: Write the minimal code needed to satisfy the test.
3. **Iterate**: Run `go test -v ./test/...` locally.
4. **Logging**: Use `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal` for logging.
5. **Build & Deploy**: See [CLI Reference](cli.md) for build and deployment commands.
6. **Security Audit**: Always verify no keys are committed.
7. **Commit & Push**: Commit changes and push to the remote repository.

## Code Style

Use simple code pipelines, not pyramids of nested functions.

```golang
func request(ctx context.Context) {
    var auth_result = auth(ctx)
    if auth_result == nil {
        return ctx
    }
    var database1_result = database1(ctx, auth_result)
    var database2_result = database2(ctx, database1_result)
    var logger_result = logger(ctx, auth_result, database1_result, database2_result)
    return ctx
}
```
