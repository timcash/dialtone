# Branch: www-improvements
# Task: WWW Improvements

## Goal
Migrate the ./dialtone-earth directory to the ./src/plugins/www/ directory and improve the CLI integration and testing.

## Test
- test: Verify src/plugins/www/ exists and contains the Next.js app.
- verification: `ls src/plugins/www/app` shows expected files.
- test: Local development server starts via CLI.
- verification: `go run dialtone-dev.go www dev` starts vite and responds on localhost.

## Subtask: Migration
- description: Migrate dialtone-earth to src/plugins/www and move cli commands to src/plugins/www/cli
- status: todo

## Subtask: CLI Features
- description: Provide dialtone-dev www --help and a dev command for local server
- status: todo

## Development Cycle
1. Run `go run dialtone-dev ticket start www_improvements` to change the git branch and verify development template files.
2. Update a test before writing new code and run the test to show a failure.
3. Change the system until the test passes.
4. Use `git add` to update git and ensure `.gitignore` is correct.

---
Template version: 3.0. To start work: dialtone-dev ticket start www_improvements