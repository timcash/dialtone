# GitHub plugin
The `ticket` plugin delegates several commands to the `github` plugin for seamless issue management.

## GitHub issue commands
```bash
./dialtone.sh github issue create # Create a new GitHub issue.
./dialtone.sh github issue view <id> # View details of an issue.
./dialtone.sh github issue comment <id> <msg> # Comment on an issue.
./dialtone.sh github issue close <id> # Close an issue.
./dialtone.sh github issue list # List open issues (JSON output).
./dialtone.sh github issue sync # Sync open issues into tickets.
./dialtone.sh github issue close-all # Close all open issues.
```

## GitHub pull request commands
```bash
# Notes:
# Checks for `gh` in `DIALTONE_ENV` or PATH.
# flags: `--draft`, `--ready`, and `--view`.
# Verifies you are not on `main`/`master` before creating a PR.
./dialtone.sh github pr create # Create a new pull request.
./dialtone.sh github pr view <id> # View details of a pull request.
./dialtone.sh github pr comment <id> <msg> # Comment on a pull request.
./dialtone.sh github pr merge <id> # Merge a pull request (defaults to --merge).
./dialtone.sh github pr close <id> # Close a pull request.
```

## `dialtone.sh github check-deploy`
```bash
#1. Checks for Vercel CLI.
#2. Runs `vercel list` in `src/plugins/www/app` to show deployments.
./dialtone.sh github check-deploy # Check Vercel deployment status.
```
