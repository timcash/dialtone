# Swarm Plugin Cleanup Plan

## Goal

- `./dialtone.sh swarm test` runs **Go + chromedp** tests in `src/plugins/swarm/test/test.go`: start Chrome via CLI, clean up, attach chromedp, load the swarm dashboard (or test page), capture `console.log` and errors, and pass/fail based on browser output.
- **Data structure and holepunch tests** (autobase, hyperswarm, hyperbee, etc.) live in **`src/plugins/swarm/app/test.js`**. These run under Pear (Node-like), not in the browser.
- **`src/plugins/swarm/test/`** contains only **Go + chromedp** code. All TypeScript/Node E2E and KV tests are migrated out.

---

## Plan

### 1. `./dialtone.sh swarm test` behavior

- **Entry:** `cli/swarm.go` → `runSwarmTest()` calls `swarm_test.RunAll()` (or a single chromedp test).
- **test.go** (in `test/`):
  - Follow **www** pattern: cleanup ports, `./dialtone.sh chrome kill all`, start **swarm dashboard** (serves `http://127.0.0.1:4000`), wait for port, launch Chrome via `./dialtone.sh chrome new --gpu`, parse WebSocket URL, attach **chromedp** with `NewRemoteAllocator` + `NewContext`.
  - Subscribe to **console** and **exceptions** (`runtime.EventConsoleAPICalled`, `runtime.EventExceptionThrown`), format and collect logs.
  - Navigate to `http://127.0.0.1:4000`, run minimal page checks (e.g. title or key elements).
  - After run: print collected console logs, **fail if** `[error]` or `[EXCEPTION]` detected; then `./dialtone.sh chrome kill all` for cleanup.

### 2. Where tests live

| Current location | Target | Notes |
|------------------|--------|--------|
| `test/test.go` | `test/test.go` | Rewritten to chromedp-only (dashboard load + console/error capture). |
| `test/kv.ts` | `app/test.js` | Autobee/hyperbee/autobase **logic and tests** moved into `app/test.js` (Pear runtime). |
| `test/swarm_orchestrator.ts` | **Removed** | Behavior replaced by chromedp in `test/test.go` (start dashboard, open page, assert). |
| `test/test.ts` | **Removed** | Empty. |
| Multi-peer test | `app/test.js` | Already in `app/test.js`; keep as-is (e.g. `pear run ./test.js peer-a topic`). |

### 3. `app/test.js` layout

- **Hyperswarm multi-peer test** (existing): when run with args like `pear run ./test.js peer-a test-topic`, keep current behavior.
- **KV / Autobee test** (from `kv.ts`): add a mode (e.g. first arg `kv` or a separate export/script) that runs the ephemeral Autobase + Hyperbee test (sequential write/read, concurrent writes, convergence). Use same deps already in `app/package.json` (autobase, hyperbee, hypercore, b4a, bare-fs, etc.).

### 4. Remove from `test/`

- Delete: `kv.ts`, `swarm_orchestrator.ts`, `test.ts`, `types/` (e.g. `autobase.d.ts`), `types.d.ts`, `tsconfig.json`.
- No Bun/Puppeteer or TypeScript in `test/`; no `writeSwarmTestPackage` for a test dir package.json that referenced puppeteer.

### 5. CLI updates (`cli/swarm.go`)

- **`runSwarmTest`**: call `swarm_test.RunAll()` so it runs the chromedp-based test(s) in `test/test.go`. Remove any direct call to `RunMultiPeerConnection` from here if `RunAll()` becomes the single entry that runs the browser test.
- **`runSwarmE2E`**: remove or repurpose. If E2E is fully replaced by chromedp in `test/test.go`, remove the Bun + `swarm_orchestrator.ts` path and optionally the `test-e2e` subcommand (or make it an alias to `test`).
- **`runSwarmInstall`**: stop creating/linking `src/plugins/swarm/test` as a separate Node/Bun project (no `writeSwarmTestPackage(envTestDir)`, no `testDir`/`envTestDir` install steps for the test folder). Install only applies to `app/`.

### 6. Summary

- **test/** = Go + chromedp only; starts dashboard, launches Chrome, attaches, captures console/errors, asserts.
- **app/test.js** = Pear-run data structure and holepunch tests (hyperswarm multi-peer + Autobee/KV from kv.ts).
- **CLI** = `swarm test` runs Go chromedp suite; install and E2E simplified as above.
