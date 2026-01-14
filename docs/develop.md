# Development Workflow

## TDD for AI Agents

When adding features or fixing bugs (especially when utilizing LLM-based coding assistants), follow this Test-Driven Development (TDD) loop:

0. **Start a branch**: `git checkout -b feature-name`
1. **Create Test**: Add a local unit test in `test/local_test.go` or a remote integration test in `test/remote_rover_test.go`.
2. **Implement**: Write the minimal code needed to satisfy the test.
3. **Iterate**: Run `go test -v ./test/...` locally for immediate feedback.
4. **Lint**: Run `golangci-lint run` locally for immediate feedback.
5. **Build & Deploy**: Once local tests are green, run `dialtone full-build` followed by `dialtone deploy`.
6. **README Update**: If you changed interfaces (new NATS subjects, new API endpoints), update the documentation immediately.
7. **Security Audit**: Always verify no keys are commited in code, printed in logs or transferred to remote hosts. 
8. **Verify Live**: Run system-level tests against the Tailscale IP of the robot to verify end-to-end functionality.
9. **Commit**: Commit changes to the repository.
10. **Integrate main into feature branch**: before pushing to remote, ensure your feature branch is up to date with main.
11. **Push**: Push changes to the remote repository.
12. **Merge**: Merge changes into main.

## Code Style

Use simple code pipelines, not pyramids of nested functions.

```golang
// contrived example on simple pipeline code
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

## CLI Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `dialtone-1` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
