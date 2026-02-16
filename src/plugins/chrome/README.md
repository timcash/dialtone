# Plugin: chrome

The `chrome` plugin manages local Chrome/Chromium instances for Dialtone. It can verify connectivity, list running browser processes, launch new instances, and clean up Dialtone-started processes. It has full support for **WSL 2**, allowing you to manage Chrome instances running on the Windows host directly from your Linux terminal.

## Usage
```shell
./dialtone.sh chrome <command> [arguments]
```

## Commands
```shell
./dialtone.sh chrome verify    # Verify Chrome/Chromium connectivity on a remote debugging port.
./dialtone.sh chrome list      # List detected Chrome/Chromium processes with optional filters.
./dialtone.sh chrome new       # Launch a new headed Chrome instance linked to Dialtone.
./dialtone.sh chrome kill      # Kill Dialtone-originated processes (or all with --all).
./dialtone.sh chrome install   # No-op (Chrome is detected locally).
```

## Flags for `list`
- `--headed`: Filter for headed instances only.
- `--headless`: Filter for headless instances only.
- `--verbose`, `-v`: Show full command line report, including PPID and platform.

## Flags for `kill`
- `all`: (Position) Target all Dialtone-originated browsers.
- `<PID>`: (Position) Target a specific process ID.
- `--all`: Kill EVERY Chrome/Edge process on the system (use with caution).
- `--windows`: Force use of Windows `taskkill` (usually auto-detected in WSL).

## WSL 2 Support
When running in WSL, Dialtone automatically detects if a Chrome process is running on the Windows host. 
- `chrome list` will show Windows processes with the platform marked as `Windows`.
- `chrome kill` will use `taskkill.exe` via interop to terminate Windows-side browsers.
- `chrome new` will launch the Windows version of Chrome if no native Linux Chrome is found.

## Examples
```shell
./dialtone.sh chrome verify --port 9222
./dialtone.sh chrome list --headed
./dialtone.sh chrome list --verbose
./dialtone.sh chrome new https://example.com --gpu
./dialtone.sh chrome kill all
./dialtone.sh chrome kill 12345
./dialtone.sh chrome kill all --all
```

## Tests
```shell
./dialtone.sh chrome test
```
