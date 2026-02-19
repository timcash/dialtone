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
DIALTONE> dev install
DIALTONE> Signatures verified. Spawning subtone subprocess via PID 50569...
DIALTONE> Streaming stdout/stderr from subtone PID 50569.
DIALTONE:50569:> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:50569:> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:50569:> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:50569:> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 50569 exited with code 0.
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
DIALTONE> Signatures verified. Spawning subtone subprocess via PID 50625...
DIALTONE> Streaming stdout/stderr from subtone PID 50625.
DIALTONE:50625:> >> [Robot] Install: src_v1
DIALTONE:50625:> >> [Robot] Checking local prerequisites...
DIALTONE:50625:> >> [Robot] Installing UI dependencies (bun install)
DIALTONE:50625:> bun install v1.2.22 (6bafe260)
DIALTONE:50625:> 
DIALTONE:50625:> Checked 27 installs across 74 packages (no changes) [34.00ms]
DIALTONE:50625:> >> [Robot] Install complete: src_v1
DIALTONE> Process 50625 exited with code 0.
USER-1> exit
DIALTONE> Goodbye.

```

