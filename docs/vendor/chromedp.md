# Chromedp Integration

Dialtone uses `chromedp` to perform headless browser automation and remote verification of the Robot Web UI.

## Why Chromedp?

- **Zero-Dependency (almost)**: It uses the Chrome DevTools Protocol (CDP) and doesn't require Selenium or WebDriver.
- **Go-Native**: Integrates seamlessly with our Go CLI and plugins.
- **Remote Verification**: Allows the `diagnostic` tool to "see" the Web UI from the local development machine to confirm successful deployment.

## Browser Discovery

The `chrome` plugin and `diagnostic` tool use a multi-stage discovery process:

1. **Linux Native**: Checks `/usr/bin/google-chrome` (stable/beta/unstable), `/usr/bin/chromium-browser`, `/usr/bin/chromium`.
2. **macOS Native**: Checks `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` and Canary versions.
3. **Windows Native**: Checks `%ProgramFiles%` and `%LocalAppData%`.
4. **WSL (Windows Host)**: If running on WSL and no Linux browser is found, it automatically looks for Chrome on the Windows host at `/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`.

## Usage

### Testing Connectivity
```bash
./dialtone.sh chrome
```

### Verbose Debugging
To see exactly which paths are checked and which flags are used:
```bash
./dialtone.sh chrome --debug
```

### Custom Port
You can specify a custom debugging port:
```bash
./dialtone.sh chrome --port 9222
```

## Troubleshooting

### "executable file not found in $PATH"
If `chromedp` fails to find a browser:
- **Linux/WSL**: Install Chromium (`sudo apt install chromium-browser`).
- **WSL**: Ensure you have Chrome installed on Windows.
- **macOS**: Ensure Chrome is in your `/Applications` folder.

## Technical Details

Dialtone configures the browser with the following default flags for stability:
- `--headless`
- `--no-first-run`
- `--no-default-browser-check`
- `--remote-debugging-port=9222` (configurable)
