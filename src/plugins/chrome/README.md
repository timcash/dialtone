# Chrome Plugin

`src/plugins/chrome` manages local Chrome/Chromium/Edge instances for Dialtone and follows the shared `logs` + `test` patterns.

## What It Supports

- detect existing Chrome processes and debug ports
- start headed/headless sessions with role tags (`dev`, `test`, etc.)
- reuse existing matching role/headless sessions
- configure GPU on/off
- set explicit `--user-data-dir`
- attach to existing browser via websocket and open new tabs
- clean up Dialtone-owned sessions

## CLI

```bash
./dialtone.sh chrome help
./dialtone.sh chrome src_v1 verify --port 9222
./dialtone.sh chrome src_v1 list --verbose
./dialtone.sh chrome src_v1 new https://example.com --role dev --gpu
./dialtone.sh chrome src_v1 new --headless --role test --user-data-dir ./.chrome_data/test-profile
./dialtone.sh chrome src_v1 kill all
./dialtone.sh chrome src_v1 test
```

Commands:
- `verify [--port N] [--debug]`
- `list [--headed|--headless] [--verbose|-v]`
- `new [URL] [--port N] [--gpu] [--headless] [--role NAME] [--reuse-existing] [--user-data-dir PATH] [--debug]`
- `kill [PID|all] [--all] [--windows]`
- `test`
- `install`

## Library Usage (`src_v1/go`)

Import:

```go
import chrome "dialtone/dev/plugins/chrome/src_v1/go"
```

Start session:

```go
session, err := chrome.StartSession(chrome.SessionOptions{
	RequestedPort: 0,
	GPU:           true,
	Headless:      false,
	Role:          "dev",
	ReuseExisting: true,
	UserDataDir:   ".chrome_data/dev-profile",
})
if err != nil {
	return err
}
defer chrome.CleanupSession(session)
```

Attach and create a new tab:

```go
ctx, cancel, err := chrome.AttachToWebSocket(session.WebSocketURL)
if err != nil {
	return err
}
defer cancel()

tabCtx, tabCancel := chrome.NewTabContext(ctx)
defer tabCancel()
```

Wait for debug readiness:

```go
if err := chrome.WaitForDebugPort(session.Port, 20*time.Second); err != nil {
	return err
}
```

## Tests (`src_v1`)

Run:

```bash
./dialtone.sh chrome src_v1 test
```

Layout:
- `src/plugins/chrome/src_v1/test/cmd/main.go`
- `src/plugins/chrome/src_v1/test/01_session_lifecycle/suite.go`
- `src/plugins/chrome/src_v1/test/02_example_library/suite.go`

Coverage includes:
- detect chrome binary/path
- launch headed `dev` role (GPU on) and verify debug port
- reuse running `dev` role session
- attach via websocket and open/navigate a new tab with chromedp
- launch headless `test` role (GPU off) with explicit `user-data-dir`
- verify process listing metadata: role/headless/gpu/debug port/command
- cleanup headless while preserving dev, then full cleanup

## Logs + Filters

Logs use the shared format:
- `[T+0000s|LEVEL|src/...:line] message`

Useful filters while running chrome tests:

```bash
./dialtone.sh logs src_v1 stream --topic 'logs.test.chrome-src-v1.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.level.error.chrome.>'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.pass.chrome'
./dialtone.sh logs src_v1 stream --topic 'logfilter.tag.fail.chrome'
```
