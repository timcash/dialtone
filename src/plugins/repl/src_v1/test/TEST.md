# Test 1: REPL Startup
1. `DIALTONE>` should introduce itself and print the help command

### Results
```text
DIALTONE> Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
USER-1> DIALTONE> Goodbye.

```

# Test 2: dev install
1. `USER-1>` should request the install of the latest stable Go runtime at the `env/.env` DIALTONE_ENV path... 
2. `DIALTONE>` should run that command on a subtone

### Results
```text
DIALTONE> Virtual Librarian online.
I can bootstrap dev tools, route commands through dev.go, and help install plugins.
Type 'help' for commands, or 'exit' to quit.
USER-1> DIALTONE> dev install
DIALTONE> Signatures verified. Spawning subtone subprocess via PID 49329...
DIALTONE> Streaming stdout/stderr from subtone PID 49329.
DIALTONE:subtone:> DIALTONE> Installing latest Go runtime for managed ./dialtone.sh go commands...
DIALTONE:subtone:> Go 1.26.0 already installed at /Users/dev/dialtone_dependencies/go/bin/go
DIALTONE:subtone:> DIALTONE> Bootstrap complete. Initializing dev.go scaffold...
DIALTONE:subtone:> DIALTONE> Ready. You can now run plugin commands (install/build/test) via DIALTONE.
DIALTONE> Process 49329 exited with code 0.
USER-1> DIALTONE> Goodbye.

```

