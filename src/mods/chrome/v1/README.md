# chrome/v1

`chrome/v1` is a small Chrome control service with a narrow contract:

- one background service process
- one long-lived Chrome browser connection
- one managed `main` tab for the lifetime of the service
- optional extra named tabs
- NATS request/reply as the CLI control plane

The implementation has been validated locally with the CLI and integration tests in this repo.

## Command Shape

All commands run through:

```bash
./dialtone_mod chrome v1 <group> <action> [flags]
```

## Commands

### Tooling

- `./dialtone_mod chrome v1 install`
- `./dialtone_mod chrome v1 build`
- `./dialtone_mod chrome v1 format`
- `./dialtone_mod chrome v1 test`
- `./dialtone_mod chrome v1 test --integration`
- `./dialtone_mod chrome v1 test --filter <name>`

### Service

- `./dialtone_mod chrome v1 service start [--host HOST] [--port PORT] [--nats-url URL] [--nats-prefix PREFIX] [--chrome-debug-port PORT] [--headless] [--initial-url URL]`
- `./dialtone_mod chrome v1 service stop`
- `./dialtone_mod chrome v1 service status`

Defaults:

- host: `127.0.0.1`
- port: `7788`
- NATS URL: `nats://127.0.0.1:4222`
- NATS prefix: `chrome.v1`
- `--embedded-nats` is on by default
- `--headless` is off by default
- initial URL: `about:blank`

### Tab

- `./dialtone_mod chrome v1 tab open [--tab NAME] [--url URL]`
- `./dialtone_mod chrome v1 tab close [--tab NAME]`
- `./dialtone_mod chrome v1 tab goto [--tab NAME] --url URL`
- `./dialtone_mod chrome v1 tab list`

Defaults:

- default tab name: `main`
- `main` is created during service startup and should stay open for the life of the service
- `main` cannot be closed through the CLI

## Verified Behavior

The current service behavior is:

- startup creates and manages a single `main` tab
- the `service start` wrapper waits for `/healthz` and tolerates slower headed launches
- `tab list` reports stable tab names plus `pid`, `headless`, and `target_id`
- `tab goto --tab main` reuses the same `main` target in the normal case
- opening and closing a second tab does not disrupt `main`
- service health is exposed at `http://<host>:<port>/healthz`
- the embedded web UI is served from `http://<host>:<port>/`

The implementation uses the root chromedp page as managed `main`, then creates additional named tabs as child contexts. That matches the behavior validated by the integration tests and avoids the target-creation issues seen in earlier iterations.

Headed startup has been verified locally with the CLI. On this machine it can take more than 15 seconds, so the wrapper now waits up to 45 seconds for `/healthz` before declaring startup failure.

## Smoke Test Workflow

### 1. Clean state

```bash
./dialtone_mod chrome v1 service stop
```

If the service is not running, that is fine.

### 2. Format and test

```bash
./dialtone_mod chrome v1 format
./dialtone_mod chrome v1 test
```

### 3. Start the service

Headed:

```bash
./dialtone_mod chrome v1 service start --initial-url https://example.com
./dialtone_mod chrome v1 service status
```

Headless:

```bash
./dialtone_mod chrome v1 service start --headless --initial-url https://example.com
./dialtone_mod chrome v1 service status
```

Expected:

- Chrome starts
- `service status` reports running
- in headed mode, allow for a slower startup than headless mode

### 4. Verify the managed tab

```bash
./dialtone_mod chrome v1 tab list
```

Expected:

- exactly one tab named `main`
- response includes `target_id`

### 5. Drive `main`

```bash
./dialtone_mod chrome v1 tab goto --tab main --url https://example.com
./dialtone_mod chrome v1 tab goto --tab main --url http://127.0.0.1:7788/
./dialtone_mod chrome v1 tab list
```

Expected:

- the same `main` tab navigates between pages
- `target_id` for `main` stays the same under normal operation

### 6. Open and close a second tab

```bash
./dialtone_mod chrome v1 tab open --tab docs --url https://example.org
./dialtone_mod chrome v1 tab list
./dialtone_mod chrome v1 tab close --tab docs
./dialtone_mod chrome v1 tab list
```

Expected:

- `docs` appears in the tab list
- `docs` can be closed cleanly
- `main` remains open with the same `target_id`

### 7. Stop the service

```bash
./dialtone_mod chrome v1 service stop
```

## Verified Headed Demo

The following headed run has been exercised successfully from the CLI:

```bash
./dialtone_mod chrome v1 service stop
./dialtone_mod chrome v1 service start --host 127.0.0.1 --port 7788 --nats-url nats://127.0.0.1:4222 --nats-prefix chrome.v1.headed --chrome-debug-port 9224 --initial-url https://example.com
./dialtone_mod chrome v1 service status
./dialtone_mod chrome v1 tab list --nats-prefix chrome.v1.headed
./dialtone_mod chrome v1 tab goto --nats-prefix chrome.v1.headed --tab main --url http://127.0.0.1:7788/
./dialtone_mod chrome v1 tab list --nats-prefix chrome.v1.headed
./dialtone_mod chrome v1 tab open --nats-prefix chrome.v1.headed --tab docs --url https://example.org
./dialtone_mod chrome v1 tab list --nats-prefix chrome.v1.headed
./dialtone_mod chrome v1 tab close --nats-prefix chrome.v1.headed --tab docs
./dialtone_mod chrome v1 tab list --nats-prefix chrome.v1.headed
./dialtone_mod chrome v1 service stop
```

Observed results:

- `service start` succeeded in headed mode
- initial `tab list` showed only `main`
- `main` kept the same `target_id` before and after `tab goto`
- `docs` opened as a second tab and closed cleanly
- final `tab list` again showed only `main`

## Integration Tests

The Go integration tests are opt-in.

Run all integration tests:

```bash
./dialtone_mod chrome v1 test --integration
```

Run individual flows:

```bash
./dialtone_mod chrome v1 test --integration --filter new-headed
./dialtone_mod chrome v1 test --integration --filter new-headless
./dialtone_mod chrome v1 test --integration --filter tab-flow
./dialtone_mod chrome v1 test --integration --filter headed-tab-count
```

## NATS Contract

Default prefix: `chrome.v1`

Subjects:

- `<prefix>.tab.open`
- `<prefix>.tab.close`
- `<prefix>.tab.goto`
- `<prefix>.tab.list`

## Implementation Notes

Relevant files:

- [service_manager.go](/Users/user/dialtone/src/mods/chrome/v1/cli/service_manager.go)
- [service_server.go](/Users/user/dialtone/src/mods/chrome/v1/cli/service_server.go)
- [service_lifecycle.go](/Users/user/dialtone/src/mods/chrome/v1/cli/service_lifecycle.go)
- [service_integration_test.go](/Users/user/dialtone/src/mods/chrome/v1/cli/service_integration_test.go)
- [chromedp_readme.md](/Users/user/dialtone/src/mods/chrome/v1/chromedp_readme.md)

The most important invariants are:

- keep one browser websocket connection open for the service lifetime
- keep one managed `main` tab open for the service lifetime
- identify tabs by stable names, not window ordering
- reuse the existing tab context for `tab goto` when possible
- close the underlying CDP target for non-`main` tabs on `tab close`
