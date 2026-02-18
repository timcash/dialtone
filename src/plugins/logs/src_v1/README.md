# Logs Plugin (`logs` / `src_v1`)

Versioned logs plugin development lives under `src/plugins/logs/src_vN`. The current active version is `src_v1`.

This plugin standardizes logs across Dialtone plugins and services (local, dev, prod), and unifies browser and backend logging with a single API. It supports publishing and subscribing to logs over **NATS** (e.g. `plugin-name.test-name`) so the browser can receive unified logs in real time and display them in an **xterm**-based log view.

This plugin is both a **shared library** (for other plugins to use) and a **CLI** (for developing and testing the plugin itself). It is tested with its own CLI tools and `src_v1/ui`.

This README follows the [Dialtone README](../../../README.md) and the DAG plugin as source of truth for UI and test flow.

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
- **`./dialtone.sh logs stream`** / **`./dialtone.sh logs tail`** – Stream logs (e.g. `--remote` from robot)

Default version directory is the latest `src_vN` under `src/plugins/logs/`.

---

## Goals

- **Same format everywhere**: One log line format for all runtimes (Go, Node, browser).
- **Unified API**: One TypeScript API for both browser and backend so plugins write logs the same way.
- **Real-time in the browser**: Backend and browser logs can be published to NATS; the browser subscribes and streams them into an xterm (like the DAG log component).
- **Fallback**: Support HTTP polling (e.g. `/api/test-log`) where NATS is not available, so existing DAG-style log UIs keep working.

---

## What the Current Go Logger Does (Must Match or Exceed)

The existing logger lives at `src/core/logger/logger.go`. This plugin must support everything it does and add the features below.

| Capability | Current `logger.go` | `logs` plugin (`src_v1`) |
|------------|----------------------|---------------------------|
| **File + stdout** | Writes to `dialtone.log` and stdout | Same; configurable path and streams |
| **Format** | `[timestamp \| LEVEL \| file:function:line] message` | Same format; optional structured (JSON) for NATS |
| **Levels** | INFO, ERROR, WARN, DEBUG, FATAL, TRACE | Same + CLI levels: `--quiet`, `--verbose`, `--debug` |
| **Redirect std log** | Go `log` package output goes through logger | Same |
| **NATS server logger** | `LoggerWriter` implements NATS server Logger (Noticef, Fatalf, Errorf, Warnf, Debugf, Tracef); `GetNATSLogger()` | Same; plus **publish** log lines to NATS subjects |
| **Depth / caller** | `LogMsgWithDepth(depth, level, format, args...)` | Same for Go; TS uses stack/caller where available |

So in addition to file/stdout and NATS-server logging, this plugin adds:

- Publishing log lines to NATS subjects (e.g. `dag.my-test`).
- A TypeScript library that uses the same API in browser and Node.
- Browser subscription to those NATS topics and streaming into an xterm.

---

## Log Levels

- **CLI / operator**: `--quiet` (minimal), `--verbose` (operator-friendly), `--debug` (full diagnostic).
- **API**: `INFO`, `WARN`, `ERROR`, `DEBUG`, `FATAL`, `TRACE` (same as current logger).

Mapping: `--quiet` ≈ ERROR and above; `--verbose` ≈ INFO and above; `--debug` ≈ DEBUG/TRACE.

---

## NATS Topics for Unified Logs

Log streams are identified by **plugin** and **test (or run)** so the browser can subscribe to the right stream.

- **Subject pattern**: `logs.<plugin-name>.<test-or-run-name>`
  - Examples: `logs.dag.my-test`, `logs.template.smoke`, `logs.robot.telemetry`
- **Payload**: One log line per message (same format as file/stdout), or JSON `{ "ts", "level", "source", "message" }` for structured consumers.
- **Who publishes**: Go backend (and optionally Node) when a test or run starts; browser `logger.ts` can also publish to the same subject so all logs are unified.
- **Who subscribes**: Browser via the `logger.ts` library, over NATS (e.g. WebSocket connection to NATS). The library feeds received lines into an xterm or a callback.

---

## TypeScript Library (`logger.ts`)

A single library under this plugin used by both browser and backend (Node) to:

1. **Unified API**
   - Same methods: `info()`, `warn()`, `error()`, `debug()`, `trace()`, `fatal()`.
   - Same options: level filter, prefix (e.g. plugin name), optional structured output.

2. **Backend (Node)**
   - Write to stdout/stderr and optionally to a file.
   - Optionally **publish** each log line to a NATS subject `logs.<plugin>.<test>`.

3. **Browser**
   - Write to `console` and/or to an in-memory buffer.
   - **Subscribe** to NATS subject(s) `logs.<plugin>.<test>` (e.g. over NATS WebSocket) and receive unified backend + browser logs.
   - **Stream into xterm**: Provide a sink that writes received lines into an xterm instance (same pattern as the DAG log component: append lines to the terminal buffer).

4. **Transport**
   - Prefer **NATS** when configured (publish/subscribe).
   - Fallback: **HTTP polling** (e.g. `GET /api/test-log?offset=...` returning `{ offset, lines }`) so existing UIs (DAG log) work without NATS. The library can support both: try NATS first, fall back to HTTP polling.

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
| **Go logger** | Same as current `logger.go` (file, stdout, levels, redirect std log, NATS-server Logger) **plus** publish to NATS subjects `logs.<plugin>.<test>`. |
| **TypeScript library** | One API for browser and backend: same methods, optional NATS publish (backend) and subscribe (browser), optional HTTP polling fallback. |
| **NATS topics** | `logs.<plugin-name>.<test-name>` for unified log streams. |
| **Browser xterm** | Library can attach a log stream (NATS or HTTP) to an xterm instance so the DAG-style log component can show unified logs from NATS or from `/api/test-log`. |

---

## Planned Layout

```
src/plugins/logs/
  src_v1/
    README.md           (this file)
    logger.go           (or refactor to wrap core/logger + NATS publish)
    ts/
      logger.ts         (unified API: browser + Node)
      transport-nats.ts  (NATS subscribe/publish)
      transport-http.ts  (HTTP polling fallback)
      xterm-sink.ts     (attach stream to xterm)
```

This keeps the design in one place and makes it clear that the DAG log component's xterm can be fed by either NATS (new) or HTTP (current) via the same `logger.ts` API.
