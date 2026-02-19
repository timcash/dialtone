# Test 1: REPL Startup
1. `DIALTONE>` should introduce itself and print the help command

### Results
```text
DIALTONE> Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
USER-1> exit
DIALTONE> Goodbye.

```

# Test 2: dev install
1. `USER-1>` should request the install of the latest stable Go runtime at the `env/.env` DIALTONE_ENV path... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
USER-1> @DIALTONE dev install
DIALTONE> Request received. Spawning subtone for dev install...
DIALTONE> Spawning subtone subprocess via PID 51353...
DIALTONE> Streaming stdout/stderr from subtone PID 51353.
DIALTONE:51353> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:51353> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:51353> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:51353> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 51353 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

# Test 3: robot install src_v1
1. `USER-1>` should request robot install... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
USER-1> @DIALTONE robot install src_v1
DIALTONE> Request received. Spawning subtone for robot install...
DIALTONE> Spawning subtone subprocess via PID 51409...
DIALTONE> Streaming stdout/stderr from subtone PID 51409.
DIALTONE:51409> >> [Robot] Install: src_v1
DIALTONE:51409> >> [Robot] Checking local prerequisites...
DIALTONE:51409> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:51409> bun install v1.2.22 (6bafe260)
DIALTONE:51409> 
DIALTONE:51409> Checked 27 installs across 74 packages (no changes) [26.00ms]
DIALTONE:51409> >> [Robot] Install complete: src_v1
DIALTONE> Process 51409 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

