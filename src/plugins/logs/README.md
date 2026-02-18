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
| **CLI (stream/tail)** | Subscribe to NATS subject(s), print each message. | `--stdout` (default when running `logs stream` locally). Optional `--subject logs.dag.>` etc. |
| **File** | Logs lib starts a **NATS listener** that subscribes to the subject and writes each message to the given path. | e.g. `logs.ListenToFile("logs.dag.my-test", "/path/to/file.log")` or CLI `--file /path/to/file.log` that starts the listener and then runs the app. |
| **Browser** | UI subscribes to NATS (e.g. WebSocket), receives messages, appends to xterm or other widget. | Use `logger.ts` `createLogStream('nats', { subject, natsUrl })` and `attachToTerminal(term, stream)`. |

So: **readers** attach to NATS; the library can help by running a listener that writes to a file or by exposing a stream for stdout/UI.

### 5. CLI options (summary)

- **`logs stream`** / **`logs tail`**  
  - Subscribe to NATS and show logs.  
  - **`--stdout`** (default for local): print each message to stdout.  
  - **`--file <path>`**: start a listener that subscribes to the chosen subject and writes to `<path>`; the CLI can also print to stdout if you pass both.  
  - **`--subject <subject>`**: e.g. `logs.dag.>`, `logs.>` (default can be `logs.>` or from env).  
  - **`--remote`**: same as today (SSH to robot and tail there); on the robot, logs still go through NATS; remote tail is a consumer on that host.

- **Other CLI commands** (install, dev, test, serve, etc.)  
  - Can support **`--stdout`** so that when they run, their own log output is also printed to stdout (i.e. the process subscribes to its own subject and prints, or the lib can tee to stdout when `--stdout` is set).  
  - **`--file <path>`**: before starting the app, start a listener that subscribes to this run’s subject and writes to `<path>`.

So: by default logs only go to NATS; if you want stdout or a file, you explicitly ask for it via CLI flags or the lib’s listener API.

### 6. File writing via listener

- **If you want logs in a file**, you don’t configure the producer to write to that file.  
- You run a **listener**: the logs library subscribes to the right NATS subject and writes each message to the file you specify.  
- One listener per file (or one listener per subject that writes to one file).  
- The producer stays NATS-only; the listener is a separate consumer that happens to write to disk.

API idea (Go): `logs.ListenToFile(ctx, subject, filePath)` that subscribes to `subject` and appends each message to `filePath`.  
CLI: `logs stream --file /var/log/myapp.log --subject logs.myplugin.>` runs the listener (and optionally also prints to stdout).

---

## Plugin author guide (using the logs lib)

The **src_v1** libs are built so other plugin authors can rely on NATS-only logging and optional stdout/file with minimal code.

### Go (backend)

1. **Dependency**: Use the logs plugin’s Go package (e.g. `logs_v1` or as wired in your plugin).
2. **Init**: Create a logger bound to your plugin and run id:
   - `logger := logs.New(pluginName, runID)`  
   - This sets the NATS subject to `logs.<pluginName>.<runID>`.
3. **Log**: Call `logger.Info()`, `logger.Warn()`, `logger.Error()`, etc. Each call **publishes one message to NATS**. No file, no stdout, unless you add them.
4. **Optional – stdout for local dev**: Either use the CLI with `--stdout` when you run your plugin, or call `logger.AlsoStdout(true)` (or equivalent) so the lib tees to stdout in addition to NATS.
5. **Optional – file**: Don’t write from the producer. Run a listener: `logs.ListenToFile(ctx, "logs.<plugin>.<run>", path)` in a goroutine or separate process, or use the CLI `--file` to start the listener.

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
- **No direct file write from app**: Prefer “NATS only” from the app; add files only via listeners or CLI `--file`.
- **CLI for humans**: Use `--stdout` when you want to watch logs in the terminal; use `--file` when you want a persistent file, with the lib running the listener.

---

## Quick reference

| Role | Action | Default |
|------|--------|--------|
| **Producer (plugin)** | Publish log lines to NATS | NATS only (no file, no stdout) |
| **CLI stream** | Subscribe, show logs | `--stdout` to print; `--file <path>` to run listener writing to file |
| **File** | Get logs into a file | Logs lib starts listener: subscribe to subject, write to file |
| **Browser** | Show logs in UI | Subscribe via NATS WS, attach stream to xterm |

**Subject**: `logs.<plugin>.<run>`  
**Payload**: One line per message, or JSON `{ "ts", "level", "source", "message" }`.

---

## Repo layout

- **README.md** (this file): NATS-first design, how it works, plugin author guide.
- **src_v1/**: Implementation and versioned API (Go + TS libs, CLI, UI). See [src_v1/README.md](src_v1/README.md) for CLI commands, test, and dev flow.

Every plugin must include a `README.md` at its plugin root; the detailed design and implementation notes live in this file and the versioned README.
