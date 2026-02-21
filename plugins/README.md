# Plugin Dependency DAG

## New Plugin Layout (`src_vN`)

When creating a new plugin, use a versioned source layout so code/tests can evolve safely:

```text
src/plugins/<plugin-name>/
  README.md
  scaffold/main.go
  src_v1/
    go/           # library/runtime code
    test/
      01_setup/                 # bootstrap env, fixtures, preconditions
      02_example_library/       # shows library import/use from another binary
      03_smoke/                 # end-to-end plugin smoke flow
```

Recommended command shape:
- `./dialtone.sh <plugin> help`
- `./dialtone.sh <plugin> test src_v1`

### Import + Use `logs` (library example)

```go
package main

import logs "dialtone/dev/plugins/logs/src_v1/go"

func main() {
	logs.Info("my-plugin started")
	logs.Warn("example warning")
}
```

### Import + Use `test` (library example)

```go
package main

import testv1 "dialtone/dev/plugins/test/src_v1/go"

func main() {
	steps := []testv1.Step{
		{
			Name: "smoke",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				ctx.Logf("step running")
				return testv1.StepRunResult{Report: "ok"}, nil
			},
		},
	}

	_ = testv1.RunSuite(testv1.SuiteOptions{
		Version: "src_v1",
	}, steps)
}
```

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
