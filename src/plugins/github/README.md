# GitHub Plugin

The `github` plugin is the engine for Dialtone's issue and pull request management. It is designed for high-speed triage and automated development workflows.

## 1. Issue Management

### Core Commands
```bash
# List open issues in a markdown table (optimized for agents).
./dialtone.sh github issue list --markdown

# List open issues in raw JSON.
./dialtone.sh github issue list

# View full issue details, labels, and comments.
./dialtone.sh github issue view <id>
# Shortcut if first arg is an ID:
./dialtone.sh github issue <id>

# Create a new GitHub issue.
./dialtone.sh github issue create

# Comment on an issue.
./dialtone.sh github issue comment <id> "Your message here"

# Close specific issue(s) or all open issues.
./dialtone.sh github issue close <id>
./dialtone.sh github issue close-all
```

### Triage & Labeling Shortcuts
Fast-track triage by applying labels directly to an issue ID.
```bash
# Mark as 'ready' and 'ticket' validated.
./dialtone.sh github issue <id> --ready --ticket

# Apply priority and type labels.
./dialtone.sh github issue <id> --p0 --bug --enhancement

# Full Shortcut Suite:
# --p0, --p1, --bug, --ready, --ticket, --enhancement,
# --docs, --perf, --security, --refactor, --test, 
# --duplicate, --wontfix, --question
```

---

## 2. Pull Request Management
```bash
# Create a new PR (verifies context and clean branch).
# Flags: --draft, --ready
./dialtone.sh github pr create

# View, comment on, and manage PRs.
./dialtone.sh github pr view <id>
./dialtone.sh github pr comment <id> "Message"
./dialtone.sh github pr merge <id>
./dialtone.sh github pr close <id>
```

---

## 3. DevOps & Git
```bash
# Check Vercel deployment status.
./dialtone.sh github check-deploy

# Standard Dialtone Git workflow.
git status
git add .
git commit -m "type: description"
git push
```
