# Task Plugin

The `task` plugin provides a structured system for managing and tracking engineering tasks using Markdown-based data and a versioned workflow (`v1` vs `v2`).

## The Engineer's Path: From Discovery to Completion

```sh
# 1) A new issue arrives (#104: "Improve install plugin"). 
./dialtone.sh github issue sync src_v1

# 2) Promote to a versioned task. This creates task folder `104` with `v1/root.md` and `v2/root.md`.
./dialtone.sh task sync 104

# 3) DECOMPOSITION: We discover task 104 is too big. 
#    We create specialized input tasks.
./dialtone.sh task create 104-mock-server
./dialtone.sh task create 104-test-folder

# 4) LINKING: Use arrow syntax to define the flow.
#    "104 depends on 104-mock-server" (Input direction)
./dialtone.sh task link 104<--104-mock-server

#    "104 depends on 104-test-folder"
./dialtone.sh task link 104<--104-test-folder

# 5) Bidirectional Logic: 
#    - '104' now has '104-mock-server' as an INPUT.
#    - '104-mock-server' now has '104' as an OUTPUT.
#    Clickable Markdown links are added to both root.md files.

# 6) Visual Confirmation.
./dialtone.sh task tree 104
# Output:
# - 104
#   - 104-mock-server
#   - 104-test-folder

# 7) ESTIMATION MANDATE: Before starting work, the LLM MUST fill in 
#    'time_est' and 'token_est' in v2/root.md.
#    Example:
#    ### token_est:
#    - 40,000 tokens
#    ### time_est:
#    - 1.5 hours

# 8) EXECUTION: Fix a bug discovered during work.
./dialtone.sh task create 104-fix-compile-error
./dialtone.sh task link 104-mock-server<--104-fix-compile-error

# 9) Tree update:
# - 104-root
#   - 104-mock-server
#     - 104-fix-compile-error
#   - 104-test-folder

# 10) SIGN-OFF: Work from the leaves (inputs) up to the root.
./dialtone.sh task sign 104-fix-compile-error --role LLM-CODE
./dialtone.sh task sign 104 --role LLM-CODE

# 11) Archive and PR.
./dialtone.sh task resolve 104 --pr-url https://github.com/<org>/<repo>/pull/<id>
./dialtone.sh task archive 104
./dialtone.sh github pr src_v1
```

---

## Core Concepts

### 1. Inputs and Outputs
- **`### inputs:`** Tasks that **must** be completed before this task can be finished.
- **`### outputs:`** Tasks that are waiting for **this** task to be completed.
- Linking is **bidirectional** and uses relative Markdown paths for easy navigation.

### 2. The v1/v2 Workflow
- **`v1` (Baseline):** The state of the task at the beginning of the work cycle.
- **`v2` (WIP):** The current working state. All updates are recorded here.

## CLI Commands

### `sync [issue-id]`
Migrates GitHub issues as task folders `<id>/v1/root.md` and `<id>/v2/root.md`.

Sync behavior:
- creates root task markdown from GitHub issue markdown
- keeps root task `### outputs:` as `- none`
- writes `### issue:` link back to the source issue
- writes `### pr:` placeholder (`- none`) for later PR link
- auto-creates dependency input tasks from `### task-dependencies:` and links bidirectionally

### `link <a<--b> or <a-->b>`
- `a<--b`: Links `b` as an **input** to `a`.
- `a-->b`: Links `b` as an **output** of `a`.
- `a-->b-->c`: Chain syntax in one command.
- `a-->b,b-->c`: Comma-separated multiple links in one command.

### `tree [id]`
Prints the recursive **input** dependency tree.

### `archive <task-name>`
Promotes `v2` to `v1` to set a new baseline.

### `resolve <root-id> [--pr-url URL]`
- verifies the full input tree for `<root-id>` is done
- requires `reviewed` + `tested` signatures on all input tasks and the root task
- signs final root review and sets root signature status to `done`
- updates source issue markdown status to `done` and appends completion comment
- stores PR link in root `### pr:` when `--pr-url` is passed

## Verification

The task system itself is verified via:
- `./dialtone.sh task test src_v1`
