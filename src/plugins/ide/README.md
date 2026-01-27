# IDE Plugin
The `ide` plugin provides tools for setting up and interacting with integrated development environments, specifically tailored for the Dialtone project's agentic workflows and the Antigravity IDE.

## Core Commands

```bash
# Updates local .agent/ directories with latest documentation/rules.
# Use --symlink for development mode.
./dialtone.sh ide setup-workflows [--symlink]

# Tail Antigravity extension logs with session discovery.
# Filter for specific interaction types.
./dialtone.sh ide antigravity logs
./dialtone.sh ide antigravity logs --chat
./dialtone.sh ide antigravity logs --commands
./dialtone.sh ide antigravity logs --chat --commands
```


> [!TIP]
> The `--chat` flag replaces the legacy `--clean` flag for better semantic clarity.

> [!WARNING]
> **Current Limitation**: The `--chat` flag currently detects conversation updates (file growth) but cannot display the full message content due to proprietary compression in the `.pb` files. Metadata (roles) may be visible, but text content is currently unavailable.



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
