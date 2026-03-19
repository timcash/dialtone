# Chrome Test And Daemon Stabilization Plan

## Goal

Make `chrome src_v3` stable enough that:

- one daemon owns one browser per role
- one managed tab/session is reused across commands and tests
- WSL can control Windows `legion` entirely through REPL + NATS
- other plugins like `cad`, `ui`, and `robot` can depend on Chrome without forcing redeploys or browser churn
- `DIALTONE>` stays high-level and useful for LLMs, while subtone logs keep the full detail

## Current Problem

The original duplicate-browser bug was real, and the main causes are now known:

- browser-process counting and duplicate pruning were matching renderer/helper subprocesses, not just the top-level browser process
- the Windows fallback stop path could kill every `dialtone_chrome_v3.exe` instance, not just the requested role
- a stale-browser recovery path in `ensureBrowser()` could panic the daemon with a double unlock, which then showed up as `nats: no responders available for request`

The remaining open concern is now smaller:

- the suite now checks for recovery UI on the managed tab, but we still need to reduce noisy stale remote log blocks so the reports stay easier to read

## Current Progress

Passing through plain `./dialtone.sh` via REPL:

- `./dialtone.sh chrome src_v3 test --host legion --role dev --filter chrome-browser-actions-and-screenshot`
- `./dialtone.sh chrome src_v3 test --host legion --role dev`
- `./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke`

Passing focused Chrome stability checks:

- `chrome-browser-pid-stable-across-commands`
- `chrome-reset-does-not-restart-browser`
- `chrome-managed-tab-count-stays-bounded`

Passing focused Chrome stability checks:

- `chrome-browser-pid-stable-across-commands`
- `chrome-reset-does-not-restart-browser`
- `chrome-managed-tab-count-stays-bounded`
- `chrome-single-browser-process-per-role`
- `chrome-roles-do-not-share-browser-pid`
- `chrome-no-recovery-window-for-role`

Recent end-to-end passes through plain `./dialtone.sh` via REPL:

- `./dialtone.sh chrome src_v3 test --host legion --role dev`
- `./dialtone.sh chrome src_v3 test --host legion --role dev`
- `./dialtone.sh chrome src_v3 test --host legion --role dev --filter chrome-no-recovery-window-for-role`
- `./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke`
- `./dialtone.sh ui src_v1 test --attach legion --filter ui-build-and-go-serve`

Latest architecture progress:

- Chrome daemon commands are going over NATS through the REPL-managed path.
- REPL now exports a client-safe NATS URL (`127.0.0.1`) to subtones.
- Remote Chrome startup now waits on REPL service-registry state from `repl.registry.services` instead of only hammering `status` RPC during bootstrap.
- Remote Chrome startup now uses a hybrid wait:
  - REPL service-registry heartbeat first
  - short `status` probe fallback with a bounded timeout
- Role-specific ports/log files are active so one role does not steal another role’s daemon/browser.
- The Windows-native build path now resolves the synced repo copy on `legion` (`C:\Users\timca\dialtone`) instead of the broken Unix repo candidate.
- The daemon/browser duplicate-prune logic now targets top-level browser processes only:
  - requires the role-specific `--remote-debugging-port`
  - excludes child subprocesses with `--type=...`
- The Windows role stop fallback now only stops the requested role, instead of killing every `dialtone_chrome_v3.exe` process at the shared path.
- The `ensureBrowser()` stale-browser cleanup path no longer panics with a double unlock.
- The full suite log assertion now tolerates long-lived daemons whose current log tail no longer contains the original `daemon ready` line, while still failing on fatal stderr output.

## Current Hypothesis

The daemon/browser ownership model is now mostly behaving correctly for normal use:

- one daemon per role is reachable over manager NATS
- one top-level browser process per role is preserved across normal commands
- a second role now starts without taking down the first role

The next debugging focus is:

- reducing noisy/stale remote log blocks in the test report
- improving `DIALTONE>` summaries during deploy/start so the top-level shell transcript reflects build, sign, launch, and ready states more clearly
- broadening dependent-plugin verification beyond the focused `ui-build-and-go-serve` path when needed

## Desired Runtime Contract

For each role, for example `dev` or `cad-smoke`:

1. `./dialtone.sh chrome src_v3 service --host legion --mode start --role dev`
   starts one daemon if missing
2. that daemon starts or attaches to exactly one Chrome instance
3. the daemon keeps a live connection to that browser over the Chrome debug port
4. later commands:
   - `status`
   - `open`
   - `set-html`
   - `type-aria`
   - `click-aria`
   - `wait-log`
   - `screenshot`
   - plugin-driven browser usage
   all reuse the same daemon-owned browser
5. only explicit restart/stop commands may replace that browser

## What The Test Should Prove

We need Chrome tests that verify daemon correctness, not just browser actions.

### 1. Single Browser Per Role

Start the role once, then issue repeated commands and verify:

- the daemon `service_pid` stays the same
- the `browser_pid` stays the same
- the number of matching Chrome processes for that role does not increase

Test expectation:

- `status` before commands and after commands reports the same `browser_pid`

### 2. Single Managed Tab Reuse

Open a page, run multiple actions, then open another page and verify:

- the daemon reuses the same managed browser
- the tab count stays stable or returns to the intended single managed tab
- we do not accumulate orphan tabs

Test expectation:

- `status` or internal response should show one intended managed target
- repeated runs should not grow tab count without bound

### 3. No Browser Restart On Normal Commands

Verify that:

- `status` does not start a browser if the daemon is already ready
- `open` reuses the existing browser
- `reset` only resets session/tab state, not the full browser process
- filtered tests do not implicitly redeploy or restart the role

### 4. Recovery Warning Does Not Appear

This is now covered by an explicit suite step:

- open `about:blank`
- evaluate the managed page over NATS
- inspect `href`, `title`, and body text for recovery indicators such as:
  - `Chrome didn't shut down correctly`
  - `Restore pages`
  - `chrome-error://`

### 5. Role Isolation

Verify that `dev` and `cad-smoke` are isolated:

- one role does not steal the other role’s browser
- commands addressed to `cad-smoke` do not mutate `dev`
- each role keeps its own profile dir and browser pid

## Code Changes Needed

### Daemon

1. Persist stronger daemon state per role:
   - `service_pid`
   - `browser_pid`
   - `managed_target`
   - `profile_dir`
   - `chrome_port`
   - `nats_url`
   - `last_healthy_at`

2. Enforce one browser per role:
   - if `browser_pid` is alive, do not launch another browser
   - if connection is lost, reconnect to the same browser first
   - only launch a new browser if the stored pid is gone and no attachable browser exists for that role

3. Make `reset` tab-scoped only:
   - go to `about:blank`
   - clear managed target/session state
   - do not kill the browser

4. Detect duplicate browser launches:
   - if a second browser would be started for the same role, emit a warning and fail instead

### Chrome Test

Add or update tests in `src/plugins/chrome/src_v3/test/` to verify:

- browser pid stability across multiple commands
- role-specific status before and after actions
- no duplicate browser processes for one role
- no Chrome recovery page
- tab reuse behavior

### REPL / DIALTONE

Improve top-level `DIALTONE>` summaries for Chrome:

- `chrome service: ensuring daemon on legion role=dev`
- `chrome service: daemon ready on legion role=dev browser_pid=5424`
- `chrome open: reusing managed browser on legion role=dev`
- `chrome reset: clearing managed tab state on legion role=dev`
- `chrome service: refused duplicate browser launch for role=dev`

Keep raw details in subtone logs:

- exact daemon logs
- tab lists
- remote stdout/stderr
- NATS subject/debug output

## TDD Order

1. Add a Chrome test step for pid stability:
   - start service
   - read `browser_pid`
   - run several actions
   - read `browser_pid` again
   - require equality

2. Add a test step for duplicate browser detection:
   - ensure role
   - issue repeated `open`/`status`/`reset`
   - verify only one intended browser instance remains for the role

3. Add a test step for tab reuse:
   - open page A
   - open page B
   - verify managed tab count stays bounded

4. Only after Chrome tests are stable, rerun dependent plugin tests:
   - `cad src_v1 test --attach legion --filter cad-ui-browser-smoke`
   - `ui src_v1 test --attach legion ...`

## Current TDD Status

Done:

1. pid stability across normal commands
2. reset does not restart browser
3. managed tab count stays bounded
4. single browser process per role
5. roles do not share browser pid
6. explicit recovery-warning detection on the managed tab
7. reran `cad src_v1` browser smoke against the Chrome daemon path
8. reran full `chrome src_v3 test --host legion --role dev` with the recovery step included
9. reran `ui src_v1 test --attach legion --filter ui-build-and-go-serve` against the same Chrome daemon path

Next:

10. tighten `DIALTONE>` deploy/start summaries
11. reduce stale CLIXML / old stderr noise in remote log blocks

## Current Blockers

1. `deploy --service` can still keep the foreground subtone alive longer than the daemon’s actual ready point.
2. The top-level `DIALTONE>` deploy/start transcript still needs clearer phase summaries so an LLM can tell whether it is:
   - syncing code
   - building
   - signing
   - launching daemon
   - waiting for first heartbeat
3. Remote daemon log tails still include old PowerShell CLIXML noise and stale stderr blocks, which makes test reports harder to read than they should be.

## Suggested Debugging Commands

```bash
# Clean the REPL/chrome runtime, then let a normal command autostart the leader.
./dialtone.sh repl src_v3 process-clean --include-chrome

# Start or reuse the dev role.
./dialtone.sh chrome src_v3 service --host legion --mode start --role dev

# Check the role state; browser_pid should stay stable across later commands.
./dialtone.sh chrome src_v3 status --host legion --role dev

# Run the focused browser action test through the REPL.
./dialtone.sh chrome src_v3 test --host legion --role dev --filter chrome-browser-actions-and-screenshot

# Re-check status after the test and compare browser_pid/tab count.
./dialtone.sh chrome src_v3 status --host legion --role dev

# Inspect the exact subtone log if the run looked wrong in DIALTONE>.
./dialtone.sh repl src_v3 subtone-list --count 20
./dialtone.sh repl src_v3 subtone-log --pid <pid> --lines 200
```

## Success Criteria

We are done when:

- repeated `chrome src_v3 test` runs do not spawn extra browser windows
- no Chrome recovery warning appears
- the same `browser_pid` is reused for normal commands within a role
- filtered tests still work correctly
- `cad src_v1` and other plugin tests can rely on the daemon without special-case restarts
