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

