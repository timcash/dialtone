IMPORTANT! Use this prompt template for each change applied to the codebase
== START TEMPLATE ==
# FEATURE: <feature-branch-name>
## Plan Stage
1. use `dialtone install` to verfiy needed dependencies are installed
1. use `dialtone clone` to clone and verify the remote code repository is up to date
1. use `dialtone-dev branch <feature-branch-name>` to create to checkout the feature branch
1. use `dialtone-dev plan list <feature-branch-name>` to list all tests in the plan or to create
1. read the main `README.md` to get an overview of the system
2. look for or create all a plan file like `plan/plan-<feature-branch-name>.md` it should be a list of tests and notes about each test
4. if the branch or plan file are already in place you need to figure out how to continue the work
5. review the `docs/cli.md` file to understand the CLI commands as it is the center of all development work with tools like `install`, `dev`, `build` and `deploy`
4. try to use `gh pr create --title "feature-branch-name" --body "feature-branch-name"` to create a pull request with only the changes in the plan file. if it already exists skip this and add to the already created pull request

## Quick Start use of the `dialtone-dev` CLI
1. use `dialtone-dev` for all development work
1. use `dialtone-dev create-test <feature-branch-name>` to create a test
1. use `dialtone-dev run-test <feature-branch-name>` to run a test
1. use `dialtone-dev create-plan <feature-branch-name>` to create a plan
1. use `dialtone-dev pull-request <feature-branch-name> <message>` to create or update a pull request
1. use `dialtone-dev plan add <feature-branch-name>` to add a test to the plan
1. use `dialtone-dev plan remove <feature-branch-name>` to remove a test from the plan
1. use `dialtone-dev plan list <feature-branch-name>` to list all tests in the plan
1. use `dialtone-dev plan clear <feature-branch-name>` to clear all tests from the plan
1. use `dialtone-dev plan merge <feature-branch-name>` to merge a feature branch into main

## Quick Start use of the `dialtone` CLI
1. use `dialtone install` to install development dependencies
1. use `dialtone build` to build the architecture you need
1. use `dialtone deploy` to send a binary over ssh to a robot
1. use `dialtone web` to print the web dashboard URL
1. use `dialtone diagnostic --host <host_url>` to run system diagnostics on remote robot
1. use `dialtone env <var> <value>` to write to the local `.env` file (no reading for security reasons)

## Development Stage Small Iterative Loop
1. Make small changes and run tests to work iteratively
1. Improve an existing test if possible otherswise create a new one
1. Include logs and metrics as part of the test make sure they are the correct format and levels
1. All logs should use the project logging library `src/logger.
1. Write or change code test_data to pass the test
1. Update `README.md` and `docs/*` with any updates
2. Use `git commit` to stage and commit changes at each stage of the plan so you can undo work if needed.
1. Add summarized vendor or external dependecy documentation in markdown format to `docs/vendor/<vendor_name>.md`
1. Update the plan file at `plan/plan-<feature-branch-name>.md` with the changes you have made and mark completed tests with a checkmark

## Cleanup and Pull Request
0. Look at the branch and make sure it is clean and only contains the changes in the plan file.
1. Stage and commit the changes to the branch `git add .` and `git commit -m "     <feature-branch-name> comments"`
2. Update the PR with the new changes `gh pr edit --title "<feature-branch-name>" --body "<feature-branch-name> message"`

## Code Style

Use simple code pipelines, not pyramids of nested functions.

```golang
// contrived example on simple pipeline code avoid nesting code whenever possible
// use the simplest code features possible like functions and structs
// do not over architect the code or system with abstractions and interfaces
// in golang interfaces can be useful but dont overuse them
struct context {
    created_at time.Time
    updated_at time.Time
    src string
    dst string
    auth_token string
    database1_result string
    database2_result string
    logger_result string
    error error
}
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
== END TEMPLATE ==