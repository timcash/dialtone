# Plugin Dependency DAG

## New Plugin Layout (`src_vN`)

When creating a new plugin, use a versioned source layout so code/tests can evolve safely:

```text
src/plugins/<plugin-name>/
  README.md
  scaffold/main.go
  src_v1/
    go/           # library/runtime code
    cmd/          # optional service/runtime command entrypoint
    test/
      cmd/main.go               # single-process suite orchestrator
      01_setup/suite.go         # bootstrap env, fixtures, preconditions
      02_example_library/suite.go
      03_smoke/suite.go         # end-to-end plugin smoke flow
```

Recommended command shape:
- `./dialtone.sh <plugin> src_v1 help`
- `./dialtone.sh <plugin> src_v1 test`

## New Plugin Workflow (Shell)

```sh
# 1) Create plugin skeleton
mkdir -p src/plugins/my-plugin/{scaffold,src_v1/go,src_v1/test/01_setup,src_v1/test/02_example_library,src_v1/test/03_smoke}

# 2) Add scaffold entrypoint (logs-backed)
cat > src/plugins/my-plugin/scaffold/main.go <<'EOF'
package main

import (
  "os"
  logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
  logs.SetOutput(os.Stdout)
  if len(os.Args) < 2 {
    logs.Info("Usage: my-plugin <command> [args]")
    return
  }
  // route help/test/run commands to src_v1 code
}
EOF

# 3) Implement runtime library in src_v1/go using logs
# Add this to your imports:
# import logs "dialtone/dev/plugins/logs/src_v1/go"

# 4) Write tests in src_v1/test using the test library
# Add this to your imports:
# import testv1 "dialtone/dev/plugins/test/src_v1/go"
# Register steps from 01_*/02_*/03_* folders into test/cmd/main.go
# (single process; no main.go inside subfolders)

# 5) Document commands in src/plugins/my-plugin/README.md

# 6) Run plugin test workflow
./dialtone.sh my-plugin src_v1 help
./dialtone.sh my-plugin src_v1 test
```

## Foundation Library Integration

To maintain Rank 2+ status, plugins **must** import and use the foundation libraries located in `src/plugins/logs` and `src/plugins/test`.

### 1. Using the `logs` Library (Rank 0)
**Core Mandate:** Always use the `logs` library instead of `fmt` for ALL operational output in plugin code, scaffolds, orchestrators, and `dev.go`. 

- **Silence by Default:** Logs are redirected to `io.Discard` by default. Output is only visible via NATS subscription or explicit listeners.
- **No `fmt`:** Direct usage of the `fmt` package for printing to stdout/stderr is prohibited in the core dev loop.

**Import Path:** `dialtone/dev/plugins/logs/src_v1/go`

```go
package main

import (
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.Info("Plugin initialized successfully") // Flows to NATS, silent on stdout
}
```

Rendered lines use:
- `[T+0000s|LEVEL|src/path/file.go:line] message`

Topic filtering:
- raw stream: `logs.>`
- level filters: `logfilter.level.error.>`
- tag filters: `logfilter.tag.pass.>`, `logfilter.tag.fail.>`, `logfilter.tag.test.>`
- combined level+tag: `logfilter.level.error.tag.fail.>`

### 2. Using the `test` Library (Rank 1)
All plugin verification must be implemented as a test suite using the `test` library.

- **NATS Verification:** Use `ctx.WaitForMessage` to verify system behavior via NATS topics instead of inspecting stdout or log files.
- **Action -> Wait Contract:** every step should trigger an action, then wait for expected log output.
- **Status Tags:** framework emits `[TEST][PASS]` and `[TEST][FAIL]` status lines for each step.

**Import Path:** `dialtone/dev/plugins/test/src_v1/go`

```go
package main

import (
	"time"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	steps := []testv1.Step{
		{
			Name: "message-verify",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				ctx.Logf("Triggering action...")
				
				// Verify success via NATS topic
				err := ctx.WaitForMessage("logs.my-plugin.audit", "action-complete", 5*time.Second)
				if err != nil {
					return testv1.StepRunResult{}, err
				}
				return testv1.StepRunResult{Report: "Verified via NATS!"}, nil
			},
		},
	}

	_ = testv1.RunSuite(testv1.SuiteOptions{Version: "src_v1"}, steps)
}
```

Status stream examples:
- `./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.pass.>'`
- `./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.>'`

## REPL Foundation Order

`repl src_v1 test` is organized bottom-up so foundational plugins validate first via real REPL subtone execution:
- `01_repl_core`: REPL-only behavior (help, input handling, line format)
- `02_proc_plugin`: `proc src_v1 test`
- `03_logs_plugin`: `logs src_v1 test`
- `04_test_plugin`: `test src_v1 test`
- `05_chrome_plugin`: `chrome src_v1 list`
- `06_go_bun_plugins`: `go src_v1 test` then `bun src_v1 test`

REPL commands are entered directly (for example `logs src_v1 test`), without any `@DIALTONE` prefix.

This document defines the core dependency contract for plugin structure in this repo.

## The Golden Rule
A plugin node must be testable.

## Layer Rules
- **Rank 0 (Foundation): `dialtone.sh`, `logs`**
  - `dialtone.sh` is bootstrap entrypoint and installs/launches `src/dev.go`.
  - `logs` is the shared logging substrate for all plugin code paths (CLI/lib/UI adapters).
- **Rank 1 (Verification): `test`**
  - Shared test harness/utilities used to verify plugin behavior.
- **Rank 2+ (Core + Feature Plugins)**
  - Any plugin implementation (`library`, `cli`, `ui`) must depend on `logs` and should integrate with `test` for verification.

## Legend
| Rank | Color | Meaning |
|---|---|---|
| **0. Bootstrap + Logging Foundation** | <span style="color:red">█</span> Red | Entrypoint bootstrap + global logging |
| **1. Test Foundation** | <span style="color:orange">█</span> Orange | Global test plugin |
| **2. Core Runtime Plugins** | <span style="color:yellow">█</span> Yellow | Core orchestration/runtime plugins |
| **3. Feature Plugins** | <span style="color:blue">█</span> Blue | Product/task-specific plugins |
| **4. Artifacts** | <span style="color:green">█</span> Green | Build/runtime output artifacts |

## Next DAG (Target)

The target plugin dependency structure is maintained in a separate Mermaid file for planning and future state visualization.

- [Target DAG (Mermaid Source)](target.mermaid)

## Current DAG (As Implemented)

The current plugin dependency structure is maintained in a separate Mermaid file for better version control and visualization.

- [Current DAG (Mermaid Source)](current.mermaid)

## Plugin Links
- [dialtone.sh](../dialtone.sh)
- [src/dev.go](../src/dev.go)
- [logs](../src/plugins/logs/README.md)
- [test](../src/plugins/test/README.md)
- [chrome](../src/plugins/chrome/README.md)
- [go](../src/plugins/go/README.md)
- [bun](../src/plugins/bun/README.md)
- [proc](../src/plugins/proc/README.md)
- [repl](../src/plugins/repl/README.md)
- [ui](../src/plugins/ui/README.md)
- [ssh](../src/plugins/ssh/README.md)
- [github](../src/plugins/github/README.md)
- [gemini](../src/plugins/gemini/README.md)
- [worktree](../src/plugins/worktree/README.md)
- [dag](../src/plugins/dag/README.md)
- [robot](../src/plugins/robot/README.md)
- [vpn](../src/plugins/vpn/README.md)
- [ai](../src/plugins/ai/README.md)
- [cad](../src/plugins/cad/README.md)
- [www](../src/plugins/www/README.md)
- [cloudflare](../src/plugins/cloudflare/README.md)
- [swarm](../src/plugins/swarm/README.md)
- [diagnostic](../src/plugins/diagnostic/README.md)
- [camera](../src/plugins/camera/README.md)
- [mavlink](../src/plugins/mavlink/README.md)
- [ide](../src/plugins/ide/README.md)
- [task](../src/plugins/task/README.md)
- [template](../src/plugins/template/README.md)
- [wsl](../src/plugins/wsl/README.md)
- [jax-demo](../src/plugins/jax-demo/README.md)
- [deploy](../src/plugins/deploy/README.md)
- [install](../src/plugins/install/README.md)
- [nix](../src/plugins/nix/README.md)
- [plugin](../src/plugins/plugin/README.md)
- [simple-test](../src/plugins/simple-test/README.md)
- [./robot](../robot)

## Structure Contract (Per Plugin)
Each plugin may contain one or more of:
- `lib`: shared/package code
- `cli`: command entrypoints and ops
- `ui`: frontend modules

Each layer should follow these requirements:
- `lib` code must use `logs` APIs for structured logging.
- `cli` code must emit operational logs through `logs` and expose runnable verification via `test`.
- `ui` code must have testable behavior (directly or through plugin test runners) and report runtime diagnostics through `logs` bridge/adapters.
- `worktree` orchestrates agent execution and depends on `gemini` for `start` / `test` agent runs.
- `dev.go` routes interactive mode through `repl` (which runs plugin commands via subtone execution).

## Circularity Policy
- Allowed core direction: `dialtone.sh -> dev.go -> repl -> plugins`, plus `logs -> test -> all other plugins`.
- Avoid reverse edges into `logs` or `test` from higher ranks.
- If a helper is needed by both `logs` and `test`, move it to a neutral shared library path (not to either plugin).
