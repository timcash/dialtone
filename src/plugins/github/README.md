# GitHub Plugin (`src/plugins/github`)

This plugin is now `src_v1`-style and uses:
- `logs` library: `dialtone/dev/plugins/logs/src_v1/go`
- `test` library: `dialtone/dev/plugins/test/src_v1/go`

## Layout

```text
src/plugins/github/
  README.md
  scaffold/main.go
  src_v1/
    go/github.go
    issues/
    test/
      cmd/main.go
      01_self_check/suite.go
      02_example_library/suite.go
```

## Commands

```bash
./dialtone.sh github src_v1 help
./dialtone.sh github src_v1 install
./dialtone.sh github src_v1 test
```

### Issue Commands

Core issue commands:

```bash
./dialtone.sh github src_v1 issue list --state open --limit 30
./dialtone.sh github src_v1 issue view 313
./dialtone.sh github src_v1 issue print 313
./dialtone.sh github src_v1 issue verify
./dialtone.sh github src_v1 issue sync                        # default: open only
./dialtone.sh github src_v1 issue push
./dialtone.sh github src_v1 issue delete-closed
```

Notes:
- `issue sync` defaults to `--state open` (closed issues are not downloaded unless you explicitly set `--state all` or `--state closed`).
- `issue delete-closed` removes local markdown files for closed GitHub issues.
- default local dir is `src/plugins/github/src_v1/issues` (run from `src/` via `./dialtone.sh`); `--out` is optional override.

Each generated issue file:
- lives at `src/src/plugins/github/src_v1/issues/<issue_id>.md`
- starts with a `signature` block
- always starts with `- status: wait`

This is intended for a later LLM pass that upgrades each issue into full task format and flips status to `ready`.

Markdown workflow for agents/humans:
- agents write new outbound comments in `### comments-outbound:` (one `- ...` bullet per comment)
- humans run `./dialtone.sh github src_v1 issue push` to post pending outbound comments
- posted outbound lines are marked as sent (`[sent <timestamp>]`)
- `### comments-github:` mirrors GitHub comments on `issue sync`

Conflict safety:
- each issue markdown has `### sync:` metadata with `github-updated-at`
- `issue push` fetches live issue metadata first
- if GitHub changed since the last sync, push warns and skips that issue unless `--force`
- `issue push` fails if `### tags:` contains any label that does not exist in repo labels

### Pull Request Commands

Simple PR flow:

```bash
./dialtone.sh github src_v1 pr
```

Behavior:
- pushes current branch (`git push -u origin <branch>`)
- creates PR if missing
- updates existing PR if one already exists

Additional PR actions:

```bash
./dialtone.sh github src_v1 pr sync                # sync OPEN PRs to markdown files
./dialtone.sh github src_v1 pr push                # push outbound comments + label edits from markdown
./dialtone.sh github src_v1 pr print 315           # pretty local markdown view
./dialtone.sh github src_v1 pr review   # mark PR ready for review
./dialtone.sh github src_v1 pr view
./dialtone.sh github src_v1 pr merge
./dialtone.sh github src_v1 pr close
```

PR markdown workflow:
- local files are in `src/plugins/github/src_v1/prs/<pr_id>.md`
- `pr sync` writes open PR metadata, labels, and comments to markdown
- edit `### comments-outbound:` to queue comments to post
- edit `### tags:` to desired PR labels; `pr push` reconciles labels on GitHub
- `pr push` warns/skips on GitHub update conflicts unless `--force`
- `pr push` fails if `### tags:` contains any label that does not exist in repo labels
- after `pr merge`, plugin refreshes that PR markdown so merged status is reflected locally

## Allowed Labels

Use only labels that already exist in the repo label set. `issue push` and `pr push` reject unknown tags.

Check current labels at any time:

```bash
gh label list --limit 500
```

### Workflow Labels

Use workflow labels first. They control planning, priority, and readiness state for issues/PRs:

- `p0`: Highest urgency/importance.
- `p1`: Important, not urgent.
- `ready`: Ready for coding.
- `task`: Ready to be used as an LLM task.
- `bug`: Something is broken.
- `enhancement`: New feature/request.
- `refactor`: Code structure cleanup/simplification.
- `test`: Test-related work.
- `documentation`: Docs update/addition.
- `security`: Security-related work.
- `performance`: Speed/efficiency work.
- `help-wanted`: Needs extra contributors.
- `good-first-issue`: Good starter issue.
- `question`: Needs clarification.
- `duplicate`: Already tracked elsewhere.
- `invalid`: Not actionable as written.
- `wontfix`: Intentionally not planned.

Suggested use:
- pick 1 priority label: `p0` or `p1`
- pick readiness labels as status changes: `ready` -> `task`
- pick 1 or more work-type labels: `bug` / `enhancement` / `refactor` / `test` / `documentation`
- add `security` or `performance` when applicable

### Tech Labels

Tech labels are domain tags for routing and filtering. Pattern: `dialtone topic: <label>`.

- `3d`, `3dgs`, `agent`, `ai`, `api`, `architecture`, `bare`, `blender`, `caching`, `cad`, `camera`, `canbus`, `code`, `code-gen`, `codex`, `cv`, `detection`, `devops`, `discord`, `dspy`, `duckdb`, `electronics`, `environment`, `firmware`, `flakes`, `gemini`, `geometry`, `geospatial`, `go`, `graph`, `hardware`, `headscale`, `holepunch`, `install`, `kv-cache`, `long-context`, `manifold`, `maps`, `memory`, `mjpeg`, `ml`, `mocap`, `modeling`, `mujoco`, `navigation`, `network`, `nix`, `opencode`, `optimization`, `p2p`, `persistence`, `raspberry-pi`, `rendering`, `research`, `rlm`, `roboflow`, `robot`, `robotics`, `scaping`, `sdk`, `sim2real`, `simulation`, `sourcing`, `splatting`, `sql`, `streaming`, `supply-chain`, `tailscale`, `threejs`, `tui`, `ui`, `upgrade`, `urdf`, `vpn`, `wasm`, `web`

## Example Workflows

### Issue Markdown Workflow

```bash
# 1) Pull open issues from GitHub into markdown files
./dialtone.sh github src_v1 issue sync

# 2) Agent edits a file:
#    src/plugins/github/src_v1/issues/<id>.md
#    - add/update fields
#    - add bullets under `### comments-outbound:`

# 3) Human pushes outbound comments/label edits to GitHub
./dialtone.sh github src_v1 issue push

# 4) Refresh local mirror after push
./dialtone.sh github src_v1 issue sync

# 5) Optional cleanup of local closed issue files
./dialtone.sh github src_v1 issue delete-closed
```

### PR Markdown Workflow

```bash
# 1) Create/update PR for current branch
./dialtone.sh github src_v1 pr

# 2) Sync open PRs into markdown
./dialtone.sh github src_v1 pr sync

# 3) Agent edits:
#    src/plugins/github/src_v1/prs/<id>.md
#    - set `### tags:` labels
#    - add bullets under `### comments-outbound:`

# 4) Human pushes markdown changes to GitHub
./dialtone.sh github src_v1 pr push

# 5) Mark ready for review and merge when ready
./dialtone.sh github src_v1 pr review
./dialtone.sh github src_v1 pr merge
```

## Tests

Run:

```bash
./dialtone.sh github src_v1 test
```

Useful log filters while tests run:

```bash
./dialtone.sh logs stream --topic 'logfilter.level.error.>'     # all ERROR lines
./dialtone.sh logs stream --topic 'logfilter.tag.pass.>'        # all [PASS] tagged lines
./dialtone.sh logs stream --topic 'logfilter.tag.fail.>'        # all [FAIL] tagged lines
./dialtone.sh logs stream --topic 'logfilter.tag.fail.github'   # github plugin fail tags
```

Covers:
- issue markdown render includes `status: wait`
- library example runs and prints pass marker
- task-file output shape is valid for downstream task-upgrade workflows
