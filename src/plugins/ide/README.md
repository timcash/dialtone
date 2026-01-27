# IDE Plugin
The `ide` plugin provides tools for setting up and interacting with integrated development environments, specifically tailored for the Dialtone project's agentic workflows and the Antigravity IDE.

## Core Commands

### `ide setup-workflows`
Updates the local `.agent/workflows` and `.agent/rules` directories with the latest documentation from `docs/workflows` and `docs/rules`. This ensures that any AI agents working in your local environment have the most up-to-date instructions.

```bash
# Update agent files by copying (default)
./dialtone.sh ide setup-workflows

# Update agent files using symlinks (recommended for development)
./dialtone.sh ide setup-workflows --symlink
```

### `ide antigravity logs`
Specifically for users of the Antigravity IDE. This command automatically discovers the latest extension log file and tails it. The command supports additive flags to filter for specific log types with minimal coloring.

- If **no flags** are provided, all logs are shown (with coloring for known types).
- If **one or more flags** are provided, only logs of those types are shown.

```bash
# Tail all logs
./dialtone.sh ide antigravity logs

# Show only chat interaction logs (colored [CHAT])
./dialtone.sh ide antigravity logs --chat

# Show only terminal command logs (colored [CMD])
./dialtone.sh ide antigravity logs --commands

# Combine flags to show multiple types
./dialtone.sh ide antigravity logs --chat --commands
```



## Antigravity Log Discovery Logic
The plugin uses a multi-step discovery process to find the correct log file on macOS:
1. It scans `~/Library/Application Support/Antigravity/logs/` for the most recent session folder.
2. Within that folder, it looks for the `window*` subdirectory that contains the most recently modified `google.antigravity/Antigravity.log` file.
3. It then executes a `tail -f` on that specific path.

## Examples

### Update agent instructions after documentation change
```bash
# You just updated a workflow in docs/workflows/ticket.md
./dialtone.sh ide setup-workflows
```

### Monitor a long-running agent task
```bash
# Start an autonomous task, then in a new terminal:
./dialtone.sh ide antigravity logs
```
