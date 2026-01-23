# github plugin

## `dialtone.sh github pull-request`
1. Checks for `gh` CLI availability (looks in `DIALTONE_ENV` or PATH).
2. Supports subcommands:
    - `merge`: Merges current PR (defaults to `--merge`).
    - `close`: Closes current PR.
3. If no subcommand, creates or updates a PR:
    - Verifies not on `main`/`master` branch.
    - Checks if PR already exists.
    - **Create**: Creates new PR with title/body (or from plan file) and optional `--draft` flag.
    - **Update**: Updates existing PR title/body if provided.
    - **Ready**: Marks PR as ready if `--ready` flag used.
    - **View**: Opens PR in browser if `--view` flag used.

## `dialtone.sh github issue`
1. Supports subcommands:
    - `list`: Lists open issues (JSON output).
    - `sync`: Syncs open GitHub issues to local tickets (creates `tickets/<slug>/ticket.md` from template).
    - `close <number>`: Closes specific issue(s).
    - `close-all`: Closes ALL open issues.

## `dialtone.sh github check-deploy`
1. Checks for Vercel CLI.
2. Runs `vercel list` in `src/plugins/www/app` to show deployments.
