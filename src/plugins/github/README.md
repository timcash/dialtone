# GitHub Plugin

The `github` plugin is the engine for Dialtone's issue and pull request management. It is designed for high-speed triage and automated development workflows, optimized for both human developers and AI agents.

## 1. Issue Management

```bash
# List open issues. Use --markdown for agent-optimized tables.
./dialtone.sh github issue list [--markdown]

# View full issue details, labels, and comments.
./dialtone.sh github issue view <issue-id>
./dialtone.sh github issue <issue-id>  # Shortcut

# Create or comment on issues.
./dialtone.sh github issue create
./dialtone.sh github issue comment <issue-id> "Your message here"

# Close specific issue(s) or all open issues.
./dialtone.sh github issue close <issue-id>...
./dialtone.sh github issue close-all

# Triage Shortcuts: Apply labels directly via flags.
# Supported: --p0, --p1, --bug, --ready, --ticket, --enhancement,
# --docs, --perf, --security, --refactor, --test, --duplicate...
./dialtone.sh github issue <issue-id> --ready --ticket --p0 --bug
```

---

## 2. Pull Request Management

```bash
# Create or update a PR (verifies context and clean branch).
# Flags: --draft, --ready, --view
./dialtone.sh github pr create

# View, comment on, and manage PRs.
./dialtone.sh github pr view <pr-id>
./dialtone.sh github pr comment <pr-id> "Message"
./dialtone.sh github pr merge [<pr-id>]
./dialtone.sh github pr close [<pr-id>]
```

---

## 3. Git Workflow

Dialtone encourages a clean Git workflow using standardized commit messages and branch management.

```bash
./dialtone.sh branch <feature-name>
git add .
git commit -m "type: description"
git push
./dialtone.sh github pr
```
