# Logs Plugin – NATS-First Logging

All logs go through **NATS** by default. Producers (plugins, services) publish only to NATS; no file, no stdout from the app. Readers (CLI, file writer, browser) subscribe to NATS and can attach stdout, files, or UIs as they want. The CLI can output to stdout (`--stdout`); if you want a file, the logs lib starts a listener that subscribes to the NATS topic and writes to the path you specify.

---

## How It Works

### 1. Single bus: NATS

- **By default, logs only go to NATS.** No file, no stdout, from the producer side.
- **Producers** use the logs library and publish each log line to a NATS subject. They do not write to files or stdout unless you explicitly add those as extra sinks (e.g. for local dev).
- **Consumers** attach to NATS and do whatever they want with the stream:
  - **CLI** can subscribe and print to **stdout** (e.g. `--stdout`).
  - **File**: the logs lib can start a **listener** that subscribes to a NATS topic and writes to a **file you specify**.
  - **Browser** subscribes over NATS (e.g. WebSocket) and displays in an xterm or other UI.

So: one write path (NATS); many read paths (stdout, file, browser, etc.).

### 2. Subject naming

- **Pattern**: `logs.<plugin-name>.<run-or-test-name>`
- **Examples**: `logs.dag.my-test`, `logs.template.smoke`, `logs.robot.telemetry`, `logs.logs.src_v1`
- **Wildcards** (subscribers only):
  - `logs.>` – all log streams
  - `logs.dag.>` – all DAG runs
  - `logs.dag.my-test` – single stream

One log line = one NATS message. Payload can be a single line (same format as today) or JSON `{ "ts", "level", "source", "message" }` for structured use.

### 3. Producer flow (plugin / service code)

- Your code uses the **logs library** (Go or TypeScript).
- You configure it with **plugin name** and **run id** (e.g. test name, job id).
- You call `Info()`, `Warn()`, `Error()`, etc.; the library **publishes to NATS** on `logs.<plugin>.<run>`.
- **Default: no file, no stdout.** If you want a file, you run a **listener** (see below). If you want stdout for a CLI, use the CLI’s `--stdout` option.

### 4. Consumer flows

| Consumer | How | Option / API |
|----------|-----|----------------|
| **CLI (stream/tail)** | Subscribe to NATS subject(s), print each message. | `--topic` / `--stream` + `--stdout` (default). |
| **File listener** | Logs lib starts a **NATS listener** and writes each message to a file path. | `logs.ListenToFile(conn, subject, filePath)` |
| **Browser** | UI subscribes to NATS (e.g. WebSocket), receives messages, appends to xterm or other widget. | Use `logger.ts` `createLogStream('nats', { subject, natsUrl })` and `attachToTerminal(term, stream)`. |

So: **readers** attach to NATS; the library can help by running a listener that writes to a file or by exposing a stream for stdout/UI.

### 5. CLI options (summary)

- **`logs stream`** / **`logs tail`**  
  - Subscribe to NATS and show logs.  
  - **`--topic <topic>`**: topic alias (e.g. `logs.error.topic`, `*` for all logs).  
  - **`--stream <subject>`**: raw NATS subject (same purpose as `--topic`).  
  - **`--nats-url <url>`**: NATS URL (default `nats://127.0.0.1:4222`).  
  - **`--stdout`** (default for local): print each message to stdout.  
  - **Auto start behavior**: if local NATS is unreachable, `logs stream` auto-starts an embedded NATS daemon and connects.  
  - **`--embedded`**: force an in-process embedded broker for this stream process.  
  - **`--remote`**: same as today (SSH to robot and tail there); on the robot, logs still go through NATS; remote tail is a consumer on that host.

- **`logs pingpong`**  
  - Process-level topic verification utility.  
  - Intended for integration tests and manual topic round-trip validation.

So: by default logs go to NATS; consumers subscribe explicitly (CLI stream, listener-to-file, browser stream).

### Stream from a topic (CLI)

Use versioned stream syntax:

`./dialtone.sh logs stream src_v1 --stream logs.>`

Examples:
- `./dialtone.sh logs stream src_v1 --topic '*'`
- `./dialtone.sh logs stream src_v1 --stream logs.error.topic`
- `./dialtone.sh logs stream src_v1 --stream logs.> --nats-url nats://127.0.0.1:4222`
- `./dialtone.sh logs stream src_v1 --topic '*' --embedded`

Behavior:
- `logs stream` now auto-starts a local embedded NATS daemon if none is reachable at `--nats-url` (default `nats://127.0.0.1:4222`).

### Command Cookbook (`src_v1`)

- Start local daemon:
  - `./dialtone.sh logs nats-start src_v1`
- Check daemon status:
  - `./dialtone.sh logs nats-status src_v1`
- Stop tracked daemon:
  - `./dialtone.sh logs nats-stop src_v1`
- Stream all logs:
  - `./dialtone.sh logs stream src_v1 --topic '*'`
- Stream specific topic:
  - `./dialtone.sh logs stream src_v1 --topic logs.error.topic`
- Force in-process embedded server for this stream process:
  - `./dialtone.sh logs stream src_v1 --topic '*' --embedded`
- Two-process ping/pong verification:
  - Terminal A: `./dialtone.sh logs pingpong src_v1 --id alpha --peer beta --topic logs.pingpong --rounds 3`
  - Terminal B: `./dialtone.sh logs pingpong src_v1 --id beta --peer alpha --topic logs.pingpong --rounds 3`

### Ping/Pong Utility

For process-level topic verification:

`./dialtone.sh logs pingpong src_v1 --id alpha --peer beta --topic logs.pingpong --rounds 3`

### Example "Other Plugin" Pattern (in logs test folder)

`src/plugins/logs/src_v1/test/05_example_plugin/main.go` is a buildable example binary that imports the logs library and demonstrates:
- connect to an existing NATS server if available
- otherwise start embedded NATS automatically
- publish messages on a topic with `NewNATSLogger`
- subscribe/write messages to file with `ListenToFile`

This is verified by `./dialtone.sh logs test src_v1` step:
- `05 Example plugin binary imports logs library`

### 6. File writing via listener

- **If you want logs in a file**, you don’t configure the producer to write to that file.  
- You run a **listener**: the logs library subscribes to the right NATS subject and writes each message to the file you specify.  
- One listener per file (or one listener per subject that writes to one file).  
- The producer stays NATS-only; the listener is a separate consumer that happens to write to disk.

Go API: `logs.ListenToFile(conn, subject, filePath)` subscribes to `subject` and appends each message to `filePath`.

---

## Plugin author guide (using the logs lib)

The **src_v1** libs are built so other plugin authors can rely on NATS-only logging and optional stdout/file with minimal code.

### Go (backend)

1. **Dependency**: Use the logs plugin’s Go package (e.g. `logs_v1` or as wired in your plugin).
2. **Init**: Create a logger bound to your plugin and run id:
   - `logger := logs.New(pluginName, runID)`  
   - This sets the NATS subject to `logs.<pluginName>.<runID>`.
3. **Log**: Call `logger.Info()`, `logger.Warn()`, `logger.Error()`, etc. Each call **publishes one message to NATS**. No file, no stdout, unless you add them.
4. **Optional – stdout for local dev**: use `./dialtone.sh logs stream src_v1 --topic ...`.
5. **Optional – file**: run a listener: `logs.ListenToFile(conn, "logs.<plugin>.<run>", path)`.

### TypeScript (Node / browser)

1. **Dependency**: Use the logs plugin’s TS package (e.g. from `src/plugins/logs/src_v1/ui/...` or bundled lib).
2. **Init**: Create a logger with plugin name and run id; it publishes to `logs.<plugin>.<run>` over NATS (Node: TCP; browser: WebSocket).
3. **Log**: `logger.info()`, `logger.warn()`, `logger.error()`, etc. → each publishes to NATS. No file, no console, by default (or console in dev if you opt in).
4. **Optional – stdout / console**: Config option to tee to stdout (Node) or console (browser).
5. **Optional – file**: Same as Go: use a listener (Node can run the listener that writes to file; browser doesn’t write files).

### Subject and run id

- **Plugin name**: Your plugin, e.g. `dag`, `template`, `robot`, `logs`.  
- **Run id**: Test name, job id, or session id so multiple runs don’t mix. Examples: `my-test`, `smoke`, `src_v1`, `deploy-42`.  
- Full subject: `logs.<plugin>.<run>`.

---

## Workflow (best practices)

- **One producer, many consumers**: One process publishes to one (or a few) subjects; many processes can subscribe (stdout, file listener, browser, metrics, etc.).
- **Subject hierarchy**: Use `logs.<plugin>.<run>` so subscribers can use wildcards (`logs.>`, `logs.dag.>`).
- **No direct file write from app**: Prefer “NATS only” from the app; add files only via listeners.
- **CLI for humans**: Use `logs stream` with `--topic`/`--stream` to watch logs in the terminal.

## Troubleshooting

- NATS not reachable:
  - Run `./dialtone.sh logs nats-status src_v1`
  - Start it explicitly: `./dialtone.sh logs nats-start src_v1`
- Stream exits with connection errors:
  - Retry with force-embedded mode: `./dialtone.sh logs stream src_v1 --topic '*' --embedded`
- Stop local tracked daemon:
  - `./dialtone.sh logs nats-stop src_v1`
  - Note: if status still shows UP, another NATS process is likely bound to the same URL.
- Inspect daemon logs:
  - `tail -f .dialtone/logs/logs-nats-daemon.log`

---

## Quick reference

| Role | Action | Default |
|------|--------|--------|
| **Producer (plugin)** | Publish log lines to NATS | NATS only (no file, no stdout) |
| **CLI stream** | Subscribe, show logs | `--topic` / `--stream`; `*` maps to `logs.>` |
| **File** | Get logs into a file | `logs.ListenToFile(conn, subject, filePath)` |
| **Browser** | Show logs in UI | Subscribe via NATS WS, attach stream to xterm |

**Subject**: `logs.<plugin>.<run>`  
**Payload**: One line per message, or JSON `{ "ts", "level", "source", "message" }`.

---

## Repo layout

- **README.md** (this file): NATS-first design, how it works, plugin author guide.
- **src_v1/**: Implementation and versioned API (Go + TS libs, CLI, UI). See [src_v1/README.md](src_v1/README.md) for CLI commands, test, and dev flow.

Every plugin must include a `README.md` at its plugin root; the detailed design and implementation notes live in this file and the versioned README.
