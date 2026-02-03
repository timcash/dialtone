# Dialtone CLI manual (for agents and operators)

This is a technical user manual for using the Dialtone CLI (`./dialtone.sh`) to make changes, run tests, and operate the system. It is written to be copy/paste friendly for LLM agents.

### Install

```bash
git clone https://github.com/timcash/dialtone.git
cd dialtone
./dialtone.sh install
```

### Core conventions
- Prefer `./dialtone.sh` for project tasks.
- Prefer small, testable changes.
- Use logs to understand what the system is doing.

### Tickets (commands)
Tickets are the primary unit of work for changes to the system.

```bash
./dialtone.sh ticket add <ticket-name>
./dialtone.sh ticket start <ticket-name>
./dialtone.sh ticket ask <question>
./dialtone.sh ticket ask --subtask <subtask-name> <question>
./dialtone.sh ticket log <message>
./dialtone.sh ticket next
./dialtone.sh ticket done
```

### Tickets (storage)
- Each ticket stores its own DuckDB file at:
  - `src/tickets/<ticket-name>/<ticket-name>.duckdb`
- The “current ticket” pointer is stored in:
  - `src/tickets/.current_ticket`

### Tickets (tests + browser logging)
- For `www-*` tickets, the default baseline validation is typically:

```bash
./dialtone.sh plugin test www
```

- If you want `ticket next` to run a specific test command for a subtask, set the subtask test command:

```bash
# Set the test command for the "init" subtask on the current ticket
./dialtone.sh ticket subtask testcmd init ./dialtone.sh plugin test www
```

- `./dialtone.sh plugin test www` runs a Chromedp-based integration test that **captures browser `console.*` output and unhandled JS exceptions** and prints them into the test output. The test fails if it detects console errors or exceptions.

### Plugins (commands)

```bash
./dialtone.sh plugin add <plugin-name>
./dialtone.sh plugin install <plugin-name>
./dialtone.sh plugin build <plugin-name>
./dialtone.sh plugin test <plugin-name>
```

### Logs (commands)

```bash
./dialtone.sh logs
./dialtone.sh logs --lines 50
./dialtone.sh logs --remote
./dialtone.sh logs --remote --lines 50
```

### Build and deploy (commands)

```bash
./dialtone.sh build
./dialtone.sh deploy
./dialtone.sh diagnostic
```

### GitHub pull requests (commands)

```bash
./dialtone.sh github pr
./dialtone.sh github pr --draft
```

### WWW site (commands)

```bash
./dialtone.sh www dev
./dialtone.sh www build
./dialtone.sh www publish
./dialtone.sh www logs <deployment-url-or-id>
./dialtone.sh www domain [deployment-url]
./dialtone.sh www login
```

### Where things live
- Tickets: `src/tickets/<ticket-name>/`
- Plugins: `src/plugins/<plugin-name>/`
- Core code: `src/core/`
- Workflows: `docs/workflows/`

### Quick troubleshooting
- If a command fails, run logs and re-run with smaller scope.

```bash
./dialtone.sh logs --lines 200
```

