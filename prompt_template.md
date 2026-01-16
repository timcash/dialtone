> Use this prompt template for each change applied to the codebase
== start template ==
# FEATURE: feature-branch-name
## Plan Stage
1. read the main `README.md` to get an overview of the system
3. look for or create a feature branch using `gh branch feature-branch-name`
2. look for or create all a plan file like `plan/plan-feature-branch-name.md` it should be a list of tests and notes about each test
4. if the branch or plan file are already in place you need to figure out how to continue the work
5. review the `docs/cli.md` file to understand the CLI commands and it is the center of all development work with tools like `--dev` and `--full-build` and `--deploy`
4. use `gh pr create --title "feature-branch-name" --body "feature-branch-name"` to create a pull request with only the changes in the plan file.

## Development Stage Small Changes
1. Make small changes and run tests to work iteratively
1. plan, create or use an existing test
1. include logs and metrics as part of the test make sure they are the correct format and levels
1. write or change code ,test_data and docs to pass the test
2. use `git commit` to stage and commit the changes

## Verify on live robot
1. use `dialtone deploy` to deploy the changes to a remote robot
2. use `dialtone web` to access the web interface
3. use `dialtone test` to run system tests on the remote robot

## Cleanup and Pull Request
0. Look at the branch and make sure it is clean and only contains the changes in the plan file.
1. stage and commit the changes to the branch `git add .` and `git commit -m "feature-branch-name comments"`
2. update the PR with the new changes `gh pr edit --title "feature-branch-name" --body "feature-branch-name"`
3. use `gh pr merge` to merge the pull request

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