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
DIALTONE> Spawning subtone subprocess via PID 57862...
DIALTONE> Streaming stdout/stderr from subtone PID 57862.
DIALTONE:57862> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:57862> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:57862> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:57862> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 57862 exited with code 0.
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
DIALTONE> Spawning subtone subprocess via PID 57894...
DIALTONE> Streaming stdout/stderr from subtone PID 57894.
DIALTONE:57894> >> [Robot] Install: src_v1
DIALTONE:57894> >> [Robot] Checking local prerequisites...
DIALTONE:57894> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:57894> bun install v1.2.22 (6bafe260)
DIALTONE:57894> 
DIALTONE:57894> Checked 27 installs across 74 packages (no changes) [30.00ms]
DIALTONE:57894> >> [Robot] Install complete: src_v1
DIALTONE> Process 57894 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

