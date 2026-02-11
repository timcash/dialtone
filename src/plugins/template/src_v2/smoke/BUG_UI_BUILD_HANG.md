# Template Smoke UI Build Hang (`src_v2`)

## Summary

`./dialtone.sh template smoke src_v2` can fail before UI interaction steps begin because preflight `UI Build` (`vite build`) does not complete before the default 30s total smoke timeout.

## Repro

Run from repo root:

```bash
./dialtone.sh template smoke src_v2
```

Observed stage progression:

1. Go preflight (`fmt`, `vet`, `build`) passes.
2. UI install and lint pass.
3. UI build starts (`./dialtone.sh bun exec --cwd ... run build` -> `vite build`) and stalls.
4. Smoke runner panics at total timeout with stage `preflight:UI Build`.

## Current Signals

- `src/plugins/template/src_v2/smoke/smoke.log` includes live command output and progress heartbeats.
- `src/plugins/template/src_v2/smoke/SMOKE.md` is written incrementally and includes completed preflight stages.
- Exit is fail-fast/non-zero by design.

## Expected

- `UI Build` completes in preflight for `src_v2`.
- Smoke continues through section navigation and lifecycle assertions.

## Acceptance Criteria for Fix

- `./dialtone.sh template smoke src_v2` completes all test steps.
- No timeout panic at `preflight:UI Build`.
- `SMOKE.md` contains full UI step logs + screenshots.
