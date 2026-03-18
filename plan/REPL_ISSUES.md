# REPL Issues

This file tracks plugins that are still out of alignment with [REPL_STANDARDS.md](/home/user/dialtone/plan/REPL_STANDARDS.md).

## Current Inventory

### Shared Relay

Status:
- mostly aligned

Resolved:
- shell relay now shows only the invoking command's lifecycle and promoted summaries
- unrelated helper subtone noise is filtered from plain shell output

Remaining:
- preflight/bootstrap output still appears for direct maintenance commands like `repl src_v3 process-clean`
- that is acceptable for now because those are not ordinary routed operator commands

### `autoswap src_v1`

Status:
- mostly aligned

Resolved:
- promoted `DIALTONE>` summary lines added for `deploy` and `update`
- plain `./dialtone.sh autoswap src_v1 deploy --host rover --service --repo ...` now shows high-level progress
- plain `./dialtone.sh autoswap src_v1 update --host rover` now shows high-level progress
- `service`, `stage`, and `run` now emit promoted summaries for the main operator path
- local manifest paths for `stage` now resolve from repo root in plain `./dialtone.sh` usage

Remaining:
- README examples still focus on command syntax more than the shared REPL transcript contract

### `ssh src_v1`

Status:
- mostly aligned

Resolved:
- plugin now emits promoted `DIALTONE_INDEX:` summaries for `resolve`, `probe`, and `run`
- plain shell relay ordering now keeps the main `ssh probe` summaries ahead of the final lifecycle line in normal runs

Remaining:
- `sync-code` and `bootstrap` are intentionally out of scope for this pass

### `cloudflare src_v1`

Status:
- partially aligned

Resolved:
- promoted `DIALTONE>` summary lines added for `tunnel`, `shell`, `serve`, `robot`, and `login`
- README now describes the default REPL-routed shell path and points operators to subtone logs
- `provision` and `cleanup` emit high-level summaries in the plugin runtime

Remaining:
- provisioning/cleanup now emit summaries, but runtime verification still depends on Cloudflare credentials on the local host

### `chrome src_v3`

Status:
- mostly aligned

Resolved:
- clean shell lifecycle
- promoted summaries for actions/navigation/screenshot
- remote daemon autostart now works on `legion`
- screenshot output path now resolves from repo root
- README now describes the REPL-first shell path, transcript contract, and subtone-log workflow

Remaining:
- no blocking gaps for the current pass

### `robot src_v2`

Status:
- aligned enough to use as the reference pattern

Resolved:
- promoted summary lines for publish/diagnostic
- plugin-local `--host` preserved

Remaining:
- none required for current pass
