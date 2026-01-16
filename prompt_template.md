IMPORTANT! Use this prompt template for each change applied to the codebase
== START TEMPLATE ==
# FEATURE: <feature-branch-name>
## Plan Stage
1. read the main `README.md` to get an overview of the system
3. look for or create a feature branch using `git checkout -b <feature-branch-name>`
2. look for or create all a plan file like `plan/plan-<feature-branch-name>.md` it should be a list of tests and notes about each test
4. if the branch or plan file are already in place you need to figure out how to continue the work
5. review the `docs/cli.md` file to understand the CLI commands as it is the center of all development work with tools like `install`, `dev`, `build` and `deploy`
4. try to use `gh pr create --title "feature-branch-name" --body "feature-branch-name"` to create a pull request with only the changes in the plan file. if it already exists skip this and add to the already created pull request

## Development Stage Small Iterative Loop
1. Make small changes and run tests to work iteratively
1. improve an existing test if possible otherswise create a new one
1. include logs and metrics as part of the test make sure they are the correct format and levels
1. all logs should use the project logging library `src/logger.
1. write or change code test_data to pass the test
1. update `README.md` and `docs/*` with any updates
2. use `git commit` to stage and commit changes at each stage of the plan so you can undo work if needed.
1. add summarized vendor or external dependecy documentation in markdown format to `docs/vendor/<vendor_name>.md`

## Verify with a final test of the whole system
1. use `dialtone install` to install development dependencies
1. use `dialtone build` to build the architecture you need
1. use `
1. use `dialtone deploy` to send a binary over ssh to a robot
2. use `dialtone web` to access the web interface
3. use `dialtone test` to run system tests on the remote robot
1. use `dialtone env` to write only access the local `.env` file

## Cleanup and Pull Request
0. Look at the branch and make sure it is clean and only contains the changes in the plan file.
1. stage and commit the changes to the branch `git add .` and `git commit -m "     <feature-branch-name> comments"`
2. update the PR with the new changes `gh pr edit --title "<feature-branch-name>" --body "<feature-branch-name> message"`

## Code Style

Use simple code pipelines, not pyramids of nested functions.

```golang
// contrived example on simple pipeline code
// use the simplest code features possible less is better when possible
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