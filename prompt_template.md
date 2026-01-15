# Use this prompt template for each change applied to the codebase
1. read the main `README.md` to get an overview of the system
2. read the `docs/develop.md`


# Development Workflow

## TDD for AI Agents
1. clone the repo from `https://github.com/timcash/dialtone.git` if needed
2. checkout main branch: `git checkout main`
3. pull the latest changes: `git pull origin main`
4. create a new branch for your feature or bugfix: `git checkout -b feature-name`
5. run the local tests
6. create a new test if you are adding a feature or fixing a bug
6. add code or refactor to make the test pass
7. verify the new binary builds
8. update the README and docs if anything changed
9. deploy to a remote robot
10. run system tests on the remote robot
11. commit your changes
12. merge main into your branch to resolve any conflicts
13. push your branch to the remote repository
14. merge your branch into main


When adding features or fixing bugs (especially when utilizing LLM-based coding assistants), follow this Test-Driven Development (TDD) loop:

0. **Start a branch**: `git checkout -b feature-name`
1. **Create Test**: Add a test in the appropriate `test/` subdirectory that defines the expected behavior.
2. **Implement**: Write the minimal code needed to satisfy the test.
3. **Iterate**: Run `go test -v ./test/...` locally for immediate feedback.
3. **Logging**: Use `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal` for logging.
4. **Lint**: Run linter locally for immediate feedback.
5. **Build & Deploy**: Once local tests are green, run `dialtone full-build` followed by `dialtone deploy`.
6. **README Update**: If you changed interfaces (new NATS subjects, new API endpoints), update the documentation immediately.
7. **Security Audit**: Always verify no keys are commited in code, printed in logs or transferred to remote hosts. 
8. **Verify Live**: Run system-level tests against the Tailscale IP of the robot to verify end-to-end functionality.
9. **Commit**: Commit changes to the repository.
10. **Integrate main into feature branch**: before pushing to remote, ensure your feature branch is up to date with main.
11. **Pull Request**: Open a pull request to merge your feature branch into main.

## Code Style

Use simple code pipelines, not pyramids of nested functions.

```golang
// contrived example on simple pipeline code
func request(ctx context.Context) {
    var auth_result = auth(ctx)
    if auth_result = false {
        return ctx
    }
    var database1_result = database1(ctx, auth_result)
    var database2_result = database2(ctx, database1_result)
    var logger_result = logger(ctx, auth_result, database1_result, database2_result)
    return ctx
}
```

## Useful Dialtone CLI Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-hostname` | `dialtone-1` | Tailscale hostname for this node |
| `-port` | `4222` | NATS port on the tailnet |
| `-web-port` | `80` | Dashboard port |
| `-local-only`| `false` | Run without Tailscale for local debugging |
| `-ephemeral` | `false` | Node is removed from tailnet on exit |
| `-log-level` | `info` | Set log level (debug, info, warn, error) |
| `-dev` | `false` | Install build dependencies and download source code for development |