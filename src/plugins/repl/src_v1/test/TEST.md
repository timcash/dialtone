# Test 1: REPL Startup

### Files
- `src/plugins/repl/src_v1/test/01.go`
- `dialtone.sh`

### Conditions
1. `DIALTONE>` should introduce itself and print the help command

### Results
```text
DIALTONE> Virtual Librarian online.
DIALTONE> Type 'help' for commands, or 'exit' to quit.
USER-1> help
DIALTONE> Help

### Bootstrap
`@DIALTONE dev install`
Install latest Go and bootstrap dev.go command scaffold

### Plugins
`@DIALTONE robot install src_v1`
Install robot src_v1 dependencies

`@DIALTONE dag install src_v3`
Install dag src_v3 dependencies

### System
`<any command>`
Forward to @./dialtone.sh <command>
USER-1> exit
DIALTONE> Goodbye.

```

# Test 2: dev install

### Files
- `src/plugins/repl/src_v1/test/02.go`
- `src/plugins/go/install.sh`
- `dialtone.sh`

### Conditions
1. `USER-1>` should request the install of the latest stable Go runtime at the `env/.env` DIALTONE_ENV path... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
DIALTONE> Type 'help' for commands, or 'exit' to quit.
USER-1> @DIALTONE dev install
DIALTONE> Request received. Spawning subtone for dev install...
DIALTONE> Spawning subtone subprocess via PID 87785...
DIALTONE> Streaming stdout/stderr from subtone PID 87785.
DIALTONE:87785> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:87785> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:87785> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:87785> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 87785 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

# Test 3: robot install src_v1

### Files
- `src/plugins/repl/src_v1/test/03.go`
- `src/plugins/robot/ops.go`
- `dialtone.sh`

### Conditions
1. `USER-1>` should request robot install... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
DIALTONE> Type 'help' for commands, or 'exit' to quit.
USER-1> @DIALTONE robot install src_v1
DIALTONE> Request received. Spawning subtone for robot install...
DIALTONE> Spawning subtone subprocess via PID 87817...
DIALTONE> Streaming stdout/stderr from subtone PID 87817.
DIALTONE:87817> >> [Robot] Install: src_v1
DIALTONE:87817> >> [Robot] Checking local prerequisites...
DIALTONE:87817> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:87817> bun install v1.2.22 (6bafe260)
DIALTONE:87817> 
DIALTONE:87817> Checked 27 installs across 74 packages (no changes) [37.00ms]
DIALTONE:87817> >> [Robot] Install complete: src_v1
DIALTONE> Process 87817 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

# Test 4: dag install src_v3

### Files
- `src/plugins/repl/src_v1/test/04.go`
- `src/plugins/dag/cli/install.go`
- `dialtone.sh`

### Conditions
1. `USER-1>` should request dag install... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
DIALTONE> Type 'help' for commands, or 'exit' to quit.
USER-1> @DIALTONE dag install src_v3
DIALTONE> Request received. Spawning subtone for dag install...
DIALTONE> Spawning subtone subprocess via PID 87856...
DIALTONE> Streaming stdout/stderr from subtone PID 87856.
DIALTONE:87856> Orchestrator error: exit status 1
DIALTONE:87856> # dialtone/dev/plugins/chrome/app
DIALTONE:87856> plugins/chrome/app/chrome.go:189:32: undefined: browser
DIALTONE:87856> plugins/chrome/app/chrome.go:234:46: undefined: browser
DIALTONE:87856> plugins/chrome/app/chrome.go:289:10: undefined: browser
DIALTONE:87856> plugins/chrome/app/chrome.go:320:32: undefined: browser
DIALTONE:87856> plugins/chrome/app/chrome.go:421:32: undefined: browser
DIALTONE:87856> plugins/chrome/app/helpers.go:4:2: "context" imported and not used
DIALTONE:87856> exit status 1
DIALTONE> Process 87856 exited with code 1.
USER-1> exit
DIALTONE> Goodbye.

ERROR: missing expected output: ">> [DAG] Install: src_v3"
```

