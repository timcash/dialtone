# REPL v3 Test Cleanup Plan

## Goal
- Define the real REPL v3 test sequence we should run.
- Separate two valid but different test categories:
  - transport-level REPL tests using `inject`
  - user-interaction REPL tests using `join` and typed slash commands
- Make every test follow the same structure:
  1. do one action
  2. verify the resulting state or external effect
  3. verify the `DIALTONE>` / `DIALTONE:<pid>` logs
  4. clean up when the test creates state
- Prefer real integration coverage over mocked success where the capability matters.

## Command Discipline
- Use `./dialtone.sh <plugin> <src_vN> format` for formatting plugin code.
- Use `./dialtone.sh <plugin> <src_vN> check` or the plugin's `./dialtone.sh` workflow for verification.
- Do not use `go`, `gofmt`, or direct scaffold `go run` commands when working on the system.
- Test support and harness code should exercise the same public `./dialtone.sh` entrypoints a user would use whenever possible.

## Current Progress
- The REPL v3 test runtime now drives REPL through `./dialtone.sh`, not direct scaffold `go run`.
- The runtime captures both:
  - NATS room traffic
  - visible `llm-codex>` / `DIALTONE>` output from a real `join` session
- Shared subtone lifecycle expectations now exist in test support so workload tests can assert a standard pattern.
- `logs src_v1` / `StepContext` waiting is now part of the runtime path for room synchronization.
- A new observability step exists for:
  - `subtone-list`
  - `subtone-log --pid`
- The suite is tmp-first only.

## Current Blocker
- The tmp bootstrap wrapper path was the main blocker because tmp runs did not refresh the checkout's `src/plugins/repl/src_v3/TEST.md` / `TEST_RAW.md`.
- The current failing assertion on `/help` is not a REPL execution failure; it was a brittle room-message match that assumed a specific JSON field order.
- Manual REPL verification in a tmp repo still shows:
  - join as `llm-codex`
  - type `/help`
  - visible output renders correctly
- Current product gap remains separate:
  - prompt-driven guided setup flows for host-add and Cloudflare are still not implemented in REPL itself

## Next Work Order
1. Re-run `interactive-help-and-ps` through the synced tmp wrapper and make it green with non-brittle room assertions.
2. Re-run `interactive-add-host-updates-dialtone-json` through the wrapper and keep its lifecycle assertions standard.
3. Re-run `subtone-list-and-log-match-real-command` through the wrapper and keep it green.
4. Tighten transport and interactive assertions around room `input` frames without depending on JSON field order.
5. Add product support for prompt-driven `DIALTONE>` host-add question/answer flow.
6. Add product support for prompt-driven Cloudflare setup flow.

## Critical Distinction
- `inject` does put the command into REPL.
- That makes `inject` a valid test tool for command-bus and dispatch behavior.
- But `inject` is not the same thing as proving the interactive user experience.
- We should test both paths on purpose.

## Two Test Categories

### 1. Transport Tests
- These use:
  - `./dialtone.sh repl src_v3 inject --user llm-codex ...`
- These should prove:
  - command envelope reaches REPL
  - REPL routes and dispatches correctly
  - subtone lifecycle is correct
  - file/process/network side effects are real
- These are appropriate for:
  - bootstrap apply
  - SSH workload dispatch
  - Cloudflare tunnel dispatch
  - tsnet follow-up command dispatch
  - remote `--host` routing checks

### 2. User Interaction Tests
- These use:
  - `./dialtone.sh repl src_v3 join --name llm-codex ...`
  - then typed slash commands and prompt responses into the REPL stdin
- These should prove:
  - prompt/input path works
  - slash-command parsing works
  - visible user lines are rendered correctly
  - control-plane REPL behavior matches the user experience
- These are appropriate for:
  - `/help`
  - `/ps`
  - `/repl src_v3 join <room>` style room commands
  - at least one real workload command typed by the user

## Transcript-Driven Dialog Tests
- Prompt-based tests should not require manual typing during the suite.
- Use a transcript runner that drives a real `join` session as `llm-codex`.
- The runner should:
  - wait for a specific `DIALTONE>` prompt
  - send the next user response
  - continue only after the expected prompt or outcome line appears
- This should behave like a user conversation, not a direct config file write.
- This is the preferred automation model for all guided REPL flows.

### Transcript Format
- Each prompt-driven test should be defined as a small transcript.
- Minimum transcript fields:
  - `send`: what `llm-codex` types
  - `expect`: what REPL must print before the next response
- Example:

```yaml
name: add-host-wsl
user: llm-codex
steps:
  - send: /repl src_v3 add-host
  - expect: "DIALTONE> Host name?"
  - send: wsl
  - expect: "DIALTONE> Host address?"
  - send: wsl.shad-artichoke.ts.net
  - expect: "DIALTONE> SSH user?"
  - send: user
  - expect: "DIALTONE> Save host wsl to dialtone.json? [y/N]"
  - send: y
  - expect: "DIALTONE> Host wsl saved"
```

### Dialog Runner Rules
- Do not blindly pipe all lines at once.
- Always `expect` before the next `send` for guided flows.
- Fail the test if:
  - the expected prompt never appears
  - prompts arrive in the wrong order
  - the final outcome line is missing
  - the resulting state does not match the dialog inputs
- Keep transcripts small and explicit so they read like user sessions.

## Standard Test Contract
- Default test user is `llm-codex`.
- Each test should have one primary action.
- Each test should verify a real state change, process state, endpoint, file update, or external resource.
- Each workload test should verify the same subtone lifecycle contract.
- Tests that create external resources must stop and clean them up.
- Any guided setup flow should use a transcript-driven REPL dialog unless there is a clear reason not to.

## Standard Log Pattern
- For interactive user input:
  - `llm-codex> /<command>`
- For injected user input:
  - command should still be attributable to `llm-codex` in room events
- For REPL acknowledgement:
  - `DIALTONE> Request received. Spawning subtone for <plugin> <version>...`
- For subtone start:
  - `DIALTONE:<pid>> Started at <timestamp>`
  - `DIALTONE:<pid>> Command: [<argv...>]`
  - `DIALTONE:<pid>> Log: <path>`
- For subtone verification/status lines:
  - `DIALTONE:<pid>> [INFO] <what changed or what was confirmed>`
  - `DIALTONE:<pid>> [ERROR] <failure detail>`
- For completion:
  - `DIALTONE> Subtone for <plugin> <version> exited with code <n>.`

## Logging Cleanup Targets
- Avoid vague lines like `verified ...` without naming the target state.
- Prefer lines like:
  - `DIALTONE:<pid>> [INFO] mesh host wsl written to env/dialtone.json`
  - `DIALTONE:<pid>> [INFO] ssh command completed on host wsl`
  - `DIALTONE:<pid>> [INFO] tunnel started for <name> -> <url>`
  - `DIALTONE:<pid>> [INFO] tunnel stopped for <name>`
  - `DIALTONE:<pid>> [INFO] tsnet endpoint announced at nats://...`
- Tests should assert user-visible outcome lines, not just raw JSON fragments.
- `subtone-list` and `subtone-log --pid` should be first-class test steps.

## Tests We Should Run

### T01. tmp bootstrap workspace is real
- Type: bootstrap
- Status: implemented
- Act:
  - run `curl .../install.sh | bash -s -- repl src_v3 test`
- Verify:
  - temp workspace is under OS temp
  - `dialtone.sh`, `src/dev.go`, and `env/dialtone.json` exist
  - workspace is not the developer checkout

### T02. CLI help surfaces work
- Type: direct CLI
- Status: implemented
- Act:
  - run `./dialtone.sh help`
  - run `./dialtone.sh repl src_v3 help`
- Verify:
  - help output matches current src_v3 behavior
- Notes:
  - this is intentionally not a REPL interaction test

### T03. leader and join startup are visible
- Type: user interaction
- Status: implemented in source, wrapper rerun still needed
- Act:
  - start leader
  - join as `llm-codex`
- Verify:
  - NATS endpoint is reachable
  - join event is visible
  - leader active line is visible
- Log checks:
  - `DIALTONE> [JOIN] llm-codex ...`
  - `DIALTONE> DIALTONE leader active`

### T04. `/help` works through the real prompt path
- Type: user interaction
- Status: implemented in source, assertion cleanup in progress
- Act:
  - type `/help`
- Verify:
  - the room shows `llm-codex> /help`
  - help text is returned
  - no subtone starts for this built-in command

### T05. `/ps` works through the real prompt path
- Type: user interaction
- Status: implemented in source, assertion cleanup in progress
- Act:
  - type `/ps`
- Verify:
  - empty-state response is correct before workload commands
- Log checks:
  - `llm-codex> /ps`
  - `DIALTONE> No active subtones.`

### T06. one real workload command is typed as the user
- Type: user interaction
- Status: partial
- Act:
  - run one transcript-driven real workload command from the REPL prompt
  - recommended example: a generic interactive host-add flow, not a special-case WSL flag path
- Example interaction:
  - `llm-codex> /repl src_v3 add-host`
  - `DIALTONE> Host name?`
  - `llm-codex> wsl`
  - `DIALTONE> Host address?`
  - `llm-codex> wsl.shad-artichoke.ts.net`
  - `DIALTONE> SSH user?`
  - `llm-codex> user`
  - `DIALTONE> Save host wsl to dialtone.json? [y/N]`
  - `llm-codex> y`
  - `DIALTONE> Host wsl saved`
- Verify:
  - the prompt-driven flow asks for each required field one step at a time
  - the resulting host entry is written to `env/dialtone.json`
  - `wsl` is only example data; the flow is generic for any host name
  - prompt path and state update both work together
- Log checks:
  - `llm-codex> /repl src_v3 add-host`
  - `DIALTONE> Host name?`
  - `DIALTONE> Host address?`
  - `DIALTONE> SSH user?`
  - `DIALTONE> Host wsl saved`
- Notes:
  - this is the minimum proof that a human user can do real configuration work from inside the REPL
  - `wsl` should be treated as sample test data, not a special command path
  - current source only covers `/repl src_v3 add-host --name ... --host ... --user ...` as a typed command, not a guided dialog
  - the real prompt-driven question flow still needs product work

### T07. transport add-host updates `env/dialtone.json`
- Type: transport
- Status: implemented in spirit, needs explicit wrapper rerun after tmp fix
- Act:
  - run `./dialtone.sh repl src_v3 inject --user llm-codex repl src_v3 add-host --name <name> --host <host> --user <user>`
- Verify:
  - `env/dialtone.json` contains the expected mesh host entry
  - values persisted match the command arguments
- Log checks:
  - request line attributed to `llm-codex`
  - standard subtone lifecycle lines
  - explicit file outcome log

### T08. subtone observability is testable
- Type: transport/inspection
- Status: implemented in source, wrapper rerun still needed
- Act:
  - run a real subtone command first
  - run `./dialtone.sh repl src_v3 subtone-list --count 20`
  - run `./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200`
- Verify:
  - recent PID appears in the list
  - log contains argv and outcome lines matching room output

### T09. SSH workload dispatch is real
- Type: transport
- Status: partial
- Act:
  - run `./dialtone.sh repl src_v3 inject --user llm-codex ssh src_v1 run --host wsl --cmd whoami`
- Verify:
  - command executes against the target
  - output or result proves remote execution occurred
  - the lifecycle is SSH-specific and not confused with NATS host routing
- Log checks:
  - standard subtone lifecycle lines
  - explicit host/result log if possible
 - Notes:
  - source now has an interactive ssh workload step
  - wrapper-level rerun and stronger remote-result proof are still needed

### T10. Cloudflare tunnel start/stop is real
- Type: user interaction + integration
- Status: partial
- Act:
  - run a transcript-driven prompt flow for tunnel creation/start
  - then run stop/cleanup
- Example interaction:
  - `llm-codex> /cloudflare src_v1 tunnel start`
  - `DIALTONE> Tunnel name?`
  - `llm-codex> repl-src-v3-test`
  - `DIALTONE> Target URL?`
  - `llm-codex> http://127.0.0.1:8080`
  - `DIALTONE> Token?`
  - `llm-codex> <test-token>`
  - `DIALTONE> Start tunnel repl-src-v3-test -> http://127.0.0.1:8080? [y/N]`
  - `llm-codex> y`
  - `DIALTONE> Tunnel repl-src-v3-test started`
- Verify:
  - tunnel actually starts
  - tunnel is reachable or Cloudflare reports active state
  - stop actually tears it down
  - subtone log proves real binary path, not only mock behavior
  - the prompts appear in the expected order before each answer is sent
- Log checks:
  - `llm-codex> /cloudflare src_v1 tunnel start`
  - `DIALTONE> Tunnel name?`
  - `DIALTONE> Target URL?`
  - `DIALTONE> Token?`
  - standard subtone lifecycle lines
  - `DIALTONE:<pid>> [INFO] tunnel started for <name> -> <url>`
  - `DIALTONE:<pid>> [INFO] tunnel stopped for <name>`
 - Notes:
  - source currently tests a command-style flow through the live prompt path
  - the real question/answer dialog flow still needs product work
  - real start/stop cleanup still needs wrapper-level rerun

### T11. tsnet endpoint test verifies routing, not just announcement
- Type: transport integration
- Status: partial
- Act:
  - start leader in real mode
  - wait for native tailscale skip or embedded tsnet endpoint announcement
  - if embedded tsnet is active, run a real follow-up command through the announced endpoint
- Verify:
  - native tailscale case is explicitly logged as a skip condition
  - embedded tsnet case proves real routing success
- Log checks:
  - `DIALTONE> [INFO] native tailscale already connected` or
  - `DIALTONE> [INFO] tsnet NATS endpoint active: nats://...`
  - normal subtone lifecycle for the follow-up command
 - Notes:
  - current source covers announcement / skip detection
  - real follow-up routed command still needs to be added

### T12. remote `--host` routing over REPL/NATS is verified
- Type: transport integration
- Status: not started
- Act:
  - run `./dialtone.sh go src_v1 version --host <target>`
- Verify:
  - execution occurs on the target REPL host
  - route is NATS/REPL, not SSH
  - fallback behavior is observable when tailnet is unavailable and LAN is used
- Log checks:
  - connection/routing line if available
  - subtone lifecycle lines on the target host

### T13. cleanup leaves no stale state
- Type: cleanup
- Status: partial
- Act:
  - run `process-clean`
  - run `test-clean`
  - stop Cloudflare tunnel if still active
- Verify:
  - no leftover test leader/join processes
  - no leftover managed subtones
  - no leftover unwanted temp directories
 - Notes:
  - `test-clean` and `process-clean` are in use now
  - final suite-level cleanup verification still needs a dedicated test assertion

## Recommended Execution Lanes

### Fast Local Lane
- T01
- T02
- T03
- T04
- T05
- T06
- T07
- T08
- T13

### Real Integration Lane
- T01
- T03
- T06
- T07
- T08
- T09
- T10
- T11
- T12
- T13

## Definition Of Done
- `inject` tests cover transport and subtone dispatch behavior.
- transcript-driven `join` tests cover the actual interactive user path.
- All guided setup flows use transcript-driven prompt/response tests.
- At least one real workload command is tested through the live REPL prompt as `llm-codex`.
- Every workload test validates a real state change or external effect.
- Every workload test checks the same subtone lifecycle log pattern.
- Cloudflare tunnel coverage includes real start and stop cleanup.
- tsnet coverage includes real routing when an embedded endpoint is announced.
- subtone-list and subtone-log are verified as part of the suite.
