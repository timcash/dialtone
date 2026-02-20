# Plugin Dependency DAG (logs, test, ui, repl, worktree)

This map focuses on cross-plugin coupling between:
- `logs`
- `test`
- `ui`
- `repl`
- `worktree`

## Legend
| Layer | Color | Meaning |
|---|---|---|
| **1. Foundation** | <span style="color:red">█</span> Red | Base/shared capability plugins |
| **2. Test Harness** | <span style="color:orange">█</span> Orange | Reusable test orchestration |
| **3. Orchestration** | <span style="color:yellow">█</span> Yellow | User-facing orchestration plugins |

```mermaid
---
config:
  theme: dark
---
flowchart TD
    %% Layer 1: Foundation
    L1_LOGS[logs]
    L1_UI[ui]

    %% Layer 2: Test Harness
    L2_TEST[test]

    %% Layer 3: Orchestration
    L3_REPL[repl]
    L3_WORKTREE[worktree]

    %% Dependencies (plugin-level)
    L1_LOGS --> L2_TEST

    L2_TEST --> L3_REPL
    L3_REPL --> L3_WORKTREE

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;

    class L1_LOGS,L1_UI layer1;
    class L2_TEST layer2;
    class L3_REPL,L3_WORKTREE layer3;
```

## Circularity Check
- No cycle among the mapped plugins after removing `logs -> test` coupling.
- Remaining direction is `logs -> test` only (the `test` plugin imports logs APIs).

## Notes Per Plugin
- `logs`:
  - Base rank plugin for logging infrastructure.
  - Does not depend on `test` plugin.
  - Is depended on by `test` for logging APIs.
- `test`:
  - Reused by `repl` tests (`src/plugins/repl/src_v1/test/test_ctx.go`).
- `ui`:
  - No direct coupling to the other four plugins in this set.
- `repl`:
  - Uses `test` for its test harness.
  - Orchestrates `worktree` commands at runtime/test flow level.
- `worktree`:
  - No direct Go import dependency on `test`, `logs`, or `ui` in current implementation.

## If You Want `logs` as Strict First Rank
`logs` is now strict first rank within this plugin set.
