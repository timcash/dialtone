# REPL Plan

This file records the recommended next steps for getting the main plugins working well with:
- the REPL
- plain `./dialtone.sh`
- default arguments
- current READMEs

It is intended to be used together with:
- [REPL_STANDARDS.md](/home/user/dialtone/plan/REPL_STANDARDS.md)
- [REPL_ISSUES.md](/home/user/dialtone/plan/REPL_ISSUES.md)

## Recommended Order

### 1. Finish Shared REPL Shell Behavior

Goals:
- make the shell relay deterministic for very fast commands
- ensure promoted summaries appear before or with the final lifecycle line
- reduce remaining bootstrap noise for ordinary routed commands

Why first:
- every plugin depends on the shell relay
- plugin work is harder to judge if the shared layer is still noisy or inconsistent

Current focus:
- very fast commands like `ssh resolve` / `ssh probe`
- consistent ordering of:
  - summary lines
  - final `Subtone for ... exited with code ...`

### 2. Finish Plugin Alignment by Operator Importance

#### `autoswap src_v1`

Next:
- add promoted summaries for:
  - `service`
  - `stage`
  - `run`

Reason:
- `autoswap` is directly in the rover/robot deployment path
- its runtime state matters to operators and LLM agents

#### `ssh src_v1`

Scope:
- only `resolve`
- only `probe`
- only `run`

Next:
- make those three transcripts perfect through plain `./dialtone.sh`
- do not work on `sync-code` or `bootstrap` in this pass

Reason:
- those are the SSH commands used most often by operators and other plugins

#### `cloudflare src_v1`

Next:
- add promoted summaries for:
  - `provision`
  - `cleanup`

Reason:
- runtime tunnel flows are now better aligned
- provisioning and teardown are still summary-light

#### `chrome src_v3`

Next:
- keep current runtime behavior
- polish any remaining shell output rough edges
- align README to the actual REPL-first/default-shell path

Reason:
- Chrome is now close to the desired standard

#### `robot src_v2`

Next:
- mostly leave as-is unless docs drift again

Reason:
- `robot` is already the reference pattern for promoted summaries

### 3. Normalize Default Behavior

Goals:
- every plugin should work through plain:

```bash
./dialtone.sh <plugin> ...
```

without requiring:
- explicit `inject`
- explicit `--nats-url`
- unnecessary user overrides when mesh defaults already exist

Requirements:
- plugin-local `--host` is preserved consistently
- default users come from `env/dialtone.json` where appropriate
- autostart behavior is predictable for services like:
  - REPL leader
  - Chrome daemon

### 4. Do the README Pass

Each plugin README should have a short runtime note near the top.

Every README should clearly explain:
- plain `./dialtone.sh ...` is the default path
- what `DIALTONE>` should contain
- where detailed output goes
- how to inspect subtone logs:

```bash
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

Priority docs:
- `repl src_v3`
- `chrome src_v3`
- `autoswap src_v1`
- `cloudflare src_v1`
- `ssh src_v1`

### 5. Add Plain-Shell Verification per Plugin

For each plugin, keep one or two plain-shell smoke commands that verify:
- correct REPL lifecycle
- useful promoted summaries
- detailed output preserved in subtone log
- no extra flags needed

Suggested examples:
- `ssh src_v1 resolve --host rover`
- `ssh src_v1 run --host rover --cmd hostname`
- `autoswap src_v1 update --host rover`
- `cloudflare src_v1 shell status`
- `chrome src_v3 screenshot --host legion --role dev --out ...`
- `robot src_v2 diagnostic --host rover --skip-ui --public-check=false`

## Most Practical Next Task

If only one short follow-up task should be done next, use this order:

1. fix the last fast-command relay ordering issue
2. finish `autoswap service/stage/run`
3. finish `cloudflare provision/cleanup`
4. update `chrome`, `autoswap`, and `cloudflare` READMEs together

This gives the best operator and LLM-agent value with the least duplicated effort.
