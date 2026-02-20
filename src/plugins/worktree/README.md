# Worktree Plugin

The `worktree` plugin facilitates parallel development environments for LLM agents by combining Git worktrees, `tmux` sessions, and task-based isolation.

## Workflow: LLM Agent Parallelism

To start an LLM agent on a specific task without interrupting your current workspace:

1.  **Define the Task**: Create a task file in the repository root (e.g., `task_14.md`) describing the goal.
2.  **Provision Worktree**: Use the REPL to create a new worktree dedicated to this task.
    ```bash
    worktree add fix-navigation --task task_14.md
    ```
3.  **Automatic Orchestration**:
    - The plugin creates a new directory `../fix-navigation`.
    - It initializes a new `tmux` session named `fix-navigation`.
    - It launches the LLM agent inside that `tmux` session, pointed at the specific task file.
4.  **Monitor/Attach**: You can continue working in your main directory. To check on the agent, run `tmux attach -t fix-navigation`.

## Usage

### Interactive REPL

Start the REPL with `./dialtone.sh` and use the following commands:

-   **Add Worktree**:
    ```bash
    USER-1> worktree add <name> [--task <file>] [--branch <branch>]
    ```
    *Creates a new worktree at `../<name>` and a detached tmux session named `<name>`.*

-   **List Worktrees**:
    ```bash
    USER-1> worktree list
    ```
    *Lists active git worktrees and tmux sessions. Check the generated log file for output.*

-   **Remove Worktree**:
    ```bash
    USER-1> worktree remove <name>
    ```
    *Removes the worktree directory and kills the associated tmux session.*

### Command Line Interface (CLI)

You can also use the plugin directly from the shell:

```bash
./dialtone.sh worktree <command> [args...]
```

#### Commands

*   `add <name> [--task <file>] [--branch <branch>]`
    Creates a worktree and tmux session.
    *   `--task`: Path to a markdown file describing the task (copied to worktree root).
    *   `--branch`: Specify a custom branch name (defaults to worktree name).

*   `remove <name>`
    Cleans up the worktree folder and kills the tmux session.

*   `list`
    Displays active worktrees and sessions.

*   `test`
    Runs the plugin verification suite.

#### Example

```bash
./dialtone.sh worktree add feature-login --task ticket-123.md
tmux attach -t feature-login
```

## Implementation Details

### Tmux Orchestration
The repository already contains a `.tmux.conf`, indicating that `tmux` is a standard part of the environment. 

- **Tmux vs. Go Libraries**: While Go libraries like `github.com/creack/pty` allow for terminal emulation, they do not provide the persistent session management (attach/detach) that `tmux` offers. For LLM agents that may run for extended periods, `tmux` is the superior choice for visibility and recovery.
- **Orchestration**: We will use Go's `os/exec` to control `tmux`.
  - Create session: `tmux new-session -d -s <name> -c <path>`
  - Send commands: `tmux send-keys -t <name> "command" C-m`
- **Installation**: The plugin should check for `tmux` in the PATH. If missing, it can suggest installation via the system package manager or a `dialtone` setup script.

### Plugin Structure (src_v1)
Following the pattern in `src/plugins/test/src_v1`:
- **CLI/REPL Integration**: Implement a command handler that parses `--task` and the worktree name.
- **Process Management**: Use `context` and `os/exec` to manage the lifecycle of the worktree creation and the initial `tmux` launch.
- **Task Isolation**: The `task.md` should be either copied into the worktree root or symlinked to ensure the agent has a clear, isolated source of truth for its objectives.
