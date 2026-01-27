# AI Plugin (Opencode & Autocode)

The AI plugin integrates autonomous development and AI assistance into the Dialtone ecosystem. It consists of two main parts: a **CLI-based developer loop** and a **background assistant server**.

## Architecture

### 1. Opencode Server (`app`)
The `opencode` server is the background "brain" of the AI.
- **Start**: Automatically started by the main `dialtone` daemon when using `./dialtone start --opencode`, or manually via `./dialtone.sh ai opencode start`.
- **Port**: Listens on port `3000` by default.
- **Logs**: Output is streamed to `opencode.log` in the project root.
- **UI**: Accessible via `./dialtone.sh ai opencode ui` or directly at `http://localhost:3000`.

### 2. Developer Loop (`cli developer`)
This is the autonomous engine that solves tickets.
- **Command**: `./dialtone.sh ai developer --capability <label>`
- **Workflow**:
  1. Fetches open issues from GitHub via the `gh` CLI.
  2. Ranks tickets based on matching labels/capabilities.
  3. Creates a feature branch (e.g., `ticket-123`).
  4. Generates an implementation `task.md`.
  5. Launches a subagent to execute the task autonomously.
  6. Monitors progress and restarts the subagent if it gets stuck.
  7. Runs verification tests once the subagent finishes.
  8. Automatically submits a Pull Request on success.

## CLI Usage

```bash
# Starts or stops the background AI assistant server.
./dialtone.sh ai opencode start
./dialtone.sh ai opencode stop

# Opens the server UI.
./dialtone.sh ai opencode ui

# Starts the autonomous developer loop to solve open tickets.
./dialtone.sh ai developer [--capability <label>]

# Runs a specific task file using the AI assistant.
./dialtone.sh ai subagent --task <file-path>

# Authenticates with Google/Gemini.
./dialtone.sh ai auth

# Proxies a prompt to the Google Gemini CLI.
./dialtone.sh ai --gemini "<prompt>"

# Installs or verifies plugin dependencies.
./dialtone.sh ai install
./dialtone.sh ai build
```


## Dependencies
- **Binary**: Requires the `opencode` binary installed at `$HOME/.opencode/bin/opencode`.
- **Gemini CLI**: Requires `@google/gemini-cli` installed via `./dialtone.sh ai install`.
- **Environment**:
    - `DIALTONE_ENV`: Absolute path to a directory for plugin dependencies (e.g., node_modules).
    - `GOOGLE_API_KEY`: Required for Gemini CLI. Get one at [AI Studio](https://aistudio.google.com/app/apikey). The plugin automatically maps this to `GEMINI_API_KEY` used by the underlying CLI.
- **LLM API Key**: Needs a valid API key configured (via `.env` or system environment) for `opencode` operations.
