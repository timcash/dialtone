# Plugin: chrome

The `chrome` plugin manages local Chrome/Chromium instances for Dialtone. It can verify connectivity, list running browser processes, launch new instances, and clean up Dialtone-started processes.

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
