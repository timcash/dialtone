# Logs Plugin (`logs` / `src_v1`)

Versioned logs plugin development lives under `src/plugins/logs/src_vN`. The current active version is `src_v1`.

**NATS-first:** By default all logs go through NATS only (no file, no stdout from the producer). Other readers attach and write to stdout or files if they want. See the main [logs README](../../README.md) for the full design.

This plugin is both a **shared library** (for other plugin authors) and a **CLI** (for developing and testing the plugin itself). The src_v1 libs make it easy for plugin code to publish to NATS and, when needed, to run a listener that writes to a file or to use CLI `--stdout`.

This README follows the [Dialtone README](../../../README.md) and the [logs README](../../README.md) for NATS-first behavior and plugin author guide.

---

## How to Test (Primary)

```bash
./dialtone.sh logs test src_v1
```

Use this command as the default verification entrypoint for logs plugin changes.

## CLI (same tools as DAG)

The logs plugin uses the same CLI pattern as DAG and template:

- **`./dialtone.sh logs install src_v1`** – Install Go/Bun and UI deps
- **`./dialtone.sh logs fmt src_v1`** – Run go fmt
- **`./dialtone.sh logs format src_v1`** – Run UI format checks
- **`./dialtone.sh logs vet src_v1`** – Run go vet
- **`./dialtone.sh logs go-build src_v1`** – Run go build
- **`./dialtone.sh logs lint src_v1`** – Run TypeScript lint
- **`./dialtone.sh logs dev src_v1`** – Start Vite + debug browser
- **`./dialtone.sh logs ui-run src_v1`** – Run UI dev server only
- **`./dialtone.sh logs serve src_v1`** – Run plugin Go server (health, /api/test-log, static UI)
- **`./dialtone.sh logs build src_v1`** – Build UI assets
- **`./dialtone.sh logs test src_v1`** – Run automated tests (preflight + browser steps, write TEST.md)
- **`./dialtone.sh logs test src_v1 --attach`** – Run tests attaching to a running headed dev browser
- **`./dialtone.sh logs stream`** / **`./dialtone.sh logs tail`** – Subscribe to NATS and show logs. **`--stdout`** (default for local): print each message to stdout. **`--file <path>`**: start a NATS listener that subscribes to the subject and writes to `<path>`. **`--subject <subject>`**: e.g. `logs.dag.>`, `logs.>`. **`--remote`**: stream from remote robot (SSH tail; on robot logs still go through NATS).

Default version directory is the latest `src_vN` under `src/plugins/logs/`.

---

## Goals

- **NATS-only by default**: Producers publish only to NATS; no file, no stdout from the app unless explicitly requested (CLI `--stdout` or a listener for file).
- **Same format everywhere**: One log line format for all runtimes (Go, Node, browser).
- **Unified API**: One API (Go + TS) so plugin authors call `Info`/`Warn`/`Error` and the lib publishes to NATS.
- **Readers attach**: CLI can print to stdout (`--stdout`); if someone wants a file, the logs lib starts a NATS listener that writes to the path they specify; browser subscribes and shows in xterm.
- **Fallback**: HTTP polling (e.g. `/api/test-log`) where NATS is not available, so existing DAG-style log UIs keep working.

---

## What the Current Go Logger Does (Must Match or Exceed)

The existing logger lives at `src/core/logger/logger.go`. This plugin moves to NATS-first and adds listeners for file/stdout.

| Capability | Current `logger.go` | `logs` plugin (`src_v1`) |
|------------|----------------------|---------------------------|
| **Output** | File + stdout | **NATS only by default**; file/stdout via listener or CLI `--stdout` / `--file` |
| **Format** | `[timestamp \| LEVEL \| file:function:line] message` | Same format; optional structured (JSON) for NATS |
| **Levels** | INFO, ERROR, WARN, DEBUG, FATAL, TRACE | Same + CLI levels: `--quiet`, `--verbose`, `--debug` |
| **Redirect std log** | Go `log` package output goes through logger | Same; redirect can publish to NATS |
| **NATS server logger** | `LoggerWriter` implements NATS server Logger; `GetNATSLogger()` | Same; plus **publish** log lines to NATS subjects |
| **Depth / caller** | `LogMsgWithDepth(depth, level, format, args...)` | Same for Go; TS uses stack/caller where available |
| **File** | Writes to `dialtone.log` | **Listener**: lib subscribes to NATS and writes to a path you specify (or CLI `--file`) |
| **Stdout** | Prints to stdout | **CLI `--stdout`**: subscribe to NATS and print; or opt-in tee from lib |

So: producers publish only to NATS; readers (CLI with `--stdout`, listener for file, browser) subscribe and attach stdout/file/UI.

---

## Log Levels

- **CLI / operator**: `--quiet` (minimal), `--verbose` (operator-friendly), `--debug` (full diagnostic).
- **API**: `INFO`, `WARN`, `ERROR`, `DEBUG`, `FATAL`, `TRACE` (same as current logger).

Mapping: `--quiet` ≈ ERROR and above; `--verbose` ≈ INFO and above; `--debug` ≈ DEBUG/TRACE.

---

## NATS Topics for Unified Logs

Log streams are identified by **plugin** and **run** (test name, job id, etc.). By default only NATS is used; readers attach as needed.

- **Subject pattern**: `logs.<plugin-name>.<run-name>`
  - Examples: `logs.dag.my-test`, `logs.template.smoke`, `logs.robot.telemetry`, `logs.logs.src_v1`
- **Wildcards** (subscribers only): `logs.>`, `logs.dag.>`, etc.
- **Payload**: One log line per message (same format as before), or JSON `{ "ts", "level", "source", "message" }` for structured consumers.
- **Who publishes**: Plugin code via the logs lib (Go or TS); each `Info`/`Warn`/`Error` call publishes one message to NATS. No file, no stdout by default.
- **Who subscribes**: CLI (with `--stdout` or `--file`), file listener (lib subscribes and writes to path), browser (subscribe via NATS WS and show in xterm).

---

## TypeScript Library (`logger.ts`)

A single library under this plugin used by both browser and backend (Node). **Default: publish only to NATS.** Optional stdout/console or file via listener/CLI.

1. **Unified API**
   - Same methods: `info()`, `warn()`, `error()`, `debug()`, `trace()`, `fatal()`.
   - Same options: level filter, prefix (e.g. plugin name), optional structured output.
   - **By default**: each call **publishes to NATS** on `logs.<plugin>.<run>`; no file, no stdout/console unless opted in.

2. **Backend (Node)**
   - **Publish** each log line to NATS subject `logs.<plugin>.<run>` (default).
   - Optional: tee to stdout (e.g. for local dev or when CLI runs with `--stdout`).
   - File: not written by the producer; use a listener (lib or CLI `--file`) that subscribes to NATS and writes to a path.

3. **Browser**
   - **Publish** to NATS (e.g. over WebSocket) so backend and browser logs are on the same subject.
   - **Subscribe** to NATS subject(s) `logs.<plugin>.<run>` and receive unified logs.
   - **Stream into xterm**: `attachToTerminal(term, stream)` writes each received line to the terminal (same pattern as the DAG log component).

4. **Transport**
   - **Primary**: NATS (publish and subscribe).
   - **Fallback**: HTTP polling (`GET /api/test-log?offset=...`) when NATS is not available so existing UIs keep working.

---

## Browser: Pulling Unified Logs into an xterm

The DAG plugin log UI (`src/plugins/dag/src_v3/ui/src/components/log/index.ts`) today:

- Mounts an **xterm** in a container (`aria-label='Log Terminal'`).
- **Does not use NATS.** It polls **HTTP**: `GET /api/test-log?offset=<offset>` every 500ms; response `{ offset, lines }`; appends `lines` to the terminal.
- Supports cursor/select/command modes, copy, and a command input.

This plugin will:

- **Keep that xterm UX** (same component pattern: mount xterm, mode form, thumb buttons, etc.).
- **Add a transport layer** that can:
  - **Option A – NATS**: Subscribe to `logs.<plugin>.<test>` and push each received message as a line into the xterm (no polling).
  - **Option B – HTTP**: Same as today: poll `/api/test-log?offset=...` and append lines to the xterm.
- The **`logger.ts`** library will expose:
  - `createLogStream(transport: 'nats' | 'http', options)` that returns an async iterable or callback of log lines.
  - `attachToTerminal(term, stream)` that writes each line to `term.writeln(line)` (and optionally handles resize/fit like the DAG component).

So: unified logs (backend + browser) are delivered via NATS when available; otherwise via HTTP. In both cases the same xterm component can display them.

---

## Current DAG Log Behavior (Reference)

- **Transport**: HTTP only. Polls `/api/test-log?offset=<offset>`; backend reads from a log file via `readLogDelta(logPath, offset)` and returns `{ offset, lines }`.
- **UI**: xterm (Terminal + FitAddon), cursor/select/command modes, copy selection, command input, status lines like `[DAG LOG] tailing /api/test-log ...`.
- **No NATS** in this path today. This plugin does not replace this; it adds NATS as an option and unifies the log format and API so the same xterm can be fed by either NATS or HTTP.

---

## Summary

| Area | Content |
|------|--------|
| **Producers** | Logs lib (Go/TS): publish only to NATS by default; no file, no stdout unless listener or CLI `--stdout` / `--file`. |
| **CLI** | `logs stream` / `logs tail`: subscribe to NATS; `--stdout` to print; `--file <path>` to run listener that writes to file; `--subject` for topic. |
| **File** | Logs lib starts a listener: subscribe to NATS subject, write each message to the path you specify (or use CLI `--file`). |
| **NATS topics** | `logs.<plugin-name>.<run-name>`; wildcards `logs.>`, `logs.dag.>` for subscribers. |
| **Browser xterm** | Subscribe via NATS (or HTTP fallback); attach stream to xterm. |
| **Plugin authors** | Use logs lib with plugin name + run id; call Info/Warn/Error; logs go to NATS; add file/stdout via listener or CLI. See main [logs README](../../README.md). |

---

## Planned Layout

```
src/plugins/logs/
  README.md             (NATS-first design, plugin author guide)
  src_v1/
    README.md           (this file: CLI, test, dev)
    go/
      logger.go         (publish to NATS only; optional tee; ListenToFile)
      listener.go       (subscribe to subject, write to file or io.Writer)
    cmd/
      main.go           (serve: health, /api/test-log fed by listener or HTTP fallback)
    ts/
      logger.ts         (unified API: publish to NATS; optional tee)
      transport-nats.ts (NATS subscribe/publish)
      transport-http.ts (HTTP polling fallback)
      listener.ts       (subscribe to subject, write to file or callback)
      xterm-sink.ts     (attach stream to xterm)
    ui/                 (log section UI; subscribes via NATS or HTTP)
    test/
```

Producers use the lib and publish to NATS. Readers use the listener (file/stdout) or subscribe in the browser; the same `logger.ts` API and xterm sink work with NATS or HTTP fallback.
