# Test Report: chrome-src-v3

- **Date**: Sun, 08 Mar 2026 10:17:55 PDT
- **Total Duration**: 975.540312ms

## Summary

- **Steps**: 1 / 1 passed
- **Status**: PASSED

## Details

### 1. ✅ chrome-deploy-and-start

- **Duration**: 975.538005ms
- **Report**: chrome src_v3 deployed and service started on legion (service_pid=19848 browser_pid=22548)

#### Logs

```text
INFO: service ready host=legion role=dev service_pid=19848 browser_pid=22548 chrome_port=19464 nats_port=19465 unhealthy=false
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:63] chrome src_v3 daemon ready role=dev nats=19465 chrome=19464
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0002s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0002s|INFO|src/plugins/chrome/src_v3/browser.go:78] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/browser.go:97] chrome src_v3 refined browser PID: 22548
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:102] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0003s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0004s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0011s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0011s|INFO|src/plugins/chrome/src_v3/daemon.go:83] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT_END
INFO: report: chrome src_v3 deployed and service started on legion (service_pid=19848 browser_pid=22548)
PASS: [TEST][PASS] [STEP:chrome-deploy-and-start] report: chrome src_v3 deployed and service started on legion (service_pid=19848 browser_pid=22548)
```

#### Browser Logs

```text
<empty>
```

---

