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
DIALTONE> Spawning subtone subprocess via PID 51020...
DIALTONE> Streaming stdout/stderr from subtone PID 51020.
DIALTONE:51020:> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:51020:> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:51020:> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:51020:> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 51020 exited with code 0.
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
DIALTONE> Spawning subtone subprocess via PID 51078...
DIALTONE> Streaming stdout/stderr from subtone PID 51078.
DIALTONE:51078:> >> [Robot] Install: src_v1
DIALTONE:51078:> >> [Robot] Checking local prerequisites...
DIALTONE:51078:> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:51078:> bun install v1.2.22 (6bafe260)
DIALTONE:51078:> 
DIALTONE:51078:> Checked 27 installs across 74 packages (no changes) [24.00ms]
DIALTONE:51078:> >> [Robot] Install complete: src_v1
DIALTONE> Process 51078 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

