# Test Report: chrome-src-v3

- **Date**: Thu, 19 Mar 2026 15:49:41 PDT
- **Total Duration**: 25.515940568s

## Summary

- **Steps**: 9 / 9 passed
- **Status**: PASSED

## Details

### 1. ✅ chrome-deploy-and-start

- **Duration**: 845.758681ms
- **Report**: chrome src_v3 deployed and service started on legion (service_pid=3552 browser_pid=14156)

#### Logs

```text
INFO: service ready host=legion role=dev service_pid=3552 browser_pid=14156 chrome_port=19464 nats_port=47222 unhealthy=false
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0127s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0127s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0128s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0128s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0129s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0130s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0130s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0131s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0131s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0132s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0133s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0133s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0134s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0135s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0136s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0138s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0140s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0142s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0143s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0144s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0145s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0147s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 deployed and service started on legion (service_pid=3552 browser_pid=14156)
PASS: [TEST][PASS] [STEP:chrome-deploy-and-start] report: chrome src_v3 deployed and service started on legion (service_pid=3552 browser_pid=14156)
```

#### Browser Logs

```text
<empty>
```

---

### 2. ✅ chrome-browser-pid-stable-across-commands

- **Duration**: 3.312220307s
- **Report**: chrome src_v3 reused browser pid 14156 across normal commands on legion

#### Logs

```text
INFO: initial browser pid=14156 service_pid=3552
INFO: pid-check command=open browser_pid=14156 service_pid=3552 tabs=1
INFO: pid-check command=set-html browser_pid=14156 service_pid=3552 tabs=1
INFO: pid-check command=wait-log browser_pid=14156 service_pid=3552 tabs=1
INFO: pid-check command=status browser_pid=14156 service_pid=3552 tabs=1
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0130s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0131s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0131s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0132s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0133s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0133s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0134s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0135s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0136s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0138s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0140s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0142s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0143s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0144s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0145s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0147s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 reused browser pid 14156 across normal commands on legion
PASS: [TEST][PASS] [STEP:chrome-browser-pid-stable-across-commands] report: chrome src_v3 reused browser pid 14156 across normal commands on legion
```

#### Browser Logs

```text
<empty>
```

---

### 3. ✅ chrome-reset-does-not-restart-browser

- **Duration**: 2.851152954s
- **Report**: chrome src_v3 reset reused browser pid 14156 on legion

#### Logs

```text
INFO: before reset browser_pid=14156 service_pid=3552
INFO: after reset browser_pid=14156 service_pid=3552
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0134s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0135s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0136s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0138s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0139s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0140s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0142s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0143s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0144s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0145s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0147s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 reset reused browser pid 14156 on legion
PASS: [TEST][PASS] [STEP:chrome-reset-does-not-restart-browser] report: chrome src_v3 reset reused browser pid 14156 on legion
```

#### Browser Logs

```text
<empty>
```

---

### 4. ✅ chrome-managed-tab-count-stays-bounded

- **Duration**: 2.94499331s
- **Report**: chrome src_v3 kept a single managed tab across repeated opens on legion

#### Logs

```text
INFO: open cycle=1 browser_pid=14156 tabs=1 current_url=about:blank
INFO: open cycle=2 browser_pid=14156 tabs=1 current_url=data:text/html,<html><body><h1>one</h1></body></html>
INFO: open cycle=3 browser_pid=14156 tabs=1 current_url=data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0141s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0142s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0143s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0144s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0145s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0147s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 kept a single managed tab across repeated opens on legion
PASS: [TEST][PASS] [STEP:chrome-managed-tab-count-stays-bounded] report: chrome src_v3 kept a single managed tab across repeated opens on legion
```

#### Browser Logs

```text
<empty>
```

---

### 5. ✅ chrome-single-browser-process-per-role

- **Duration**: 3.19773994s
- **Report**: chrome src_v3 kept exactly one chrome process for role dev on legion

#### Logs

```text
INFO: process-count command=open browser_pid=14156 count=1
INFO: process-count command=status browser_pid=14156 count=1
INFO: process-count command=reset browser_pid=14156 count=1
INFO: process-count command=open browser_pid=14156 count=1
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0195s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0196s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 kept exactly one chrome process for role dev on legion
PASS: [TEST][PASS] [STEP:chrome-single-browser-process-per-role] report: chrome src_v3 kept exactly one chrome process for role dev on legion
```

#### Browser Logs

```text
<empty>
```

---

### 6. ✅ chrome-no-recovery-window-for-role

- **Duration**: 2.166183304s
- **Report**: chrome src_v3 showed no recovery UI for role dev on legion

#### Logs

```text
INFO: recovery-probe href="about:blank" title="" text=""
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0197s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0198s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 showed no recovery UI for role dev on legion
PASS: [TEST][PASS] [STEP:chrome-no-recovery-window-for-role] report: chrome src_v3 showed no recovery UI for role dev on legion
```

#### Browser Logs

```text
<empty>
```

---

### 7. ✅ chrome-roles-do-not-share-browser-pid

- **Duration**: 3.037287129s
- **Report**: chrome src_v3 kept role browser isolation between dev and dev-isolated on legion

#### Logs

```text
INFO: primary role=dev browser_pid=14156 service_pid=3552
INFO: secondary role=dev-isolated browser_pid=4296 service_pid=2800
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0199s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0200s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0201s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0202s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0086s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:100] chrome src_v3 daemon ready role=dev-isolated host=legion nats=nats://127.0.0.1:47222 chrome=22602
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev-isolated host=legion nats=nats://127.0.0.1:47222 chrome=22602
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev-isolated host=legion nats=nats://127.0.0.1:47222 chrome=22602
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0039s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0097s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0099s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0099s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=22602 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev-isolated --dialtone-role=dev-isolated --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev-isolated --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0101s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 4296
INFO: REMOTE_STDOUT [T+0101s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>secondary</h1></body></html>
INFO: REMOTE_STDOUT [T+0120s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0121s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0121s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>secondary</h1></body></html>
INFO: REMOTE_STDOUT [T+0225s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0226s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0226s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>secondary</h1></body></html>
INFO: REMOTE_STDOUT [T+0298s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0299s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0299s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>secondary</h1></body></html>
INFO: REMOTE_STDOUT [T+0753s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0754s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0754s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>secondary</h1></body></html>
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 kept role browser isolation between dev and dev-isolated on legion
PASS: [TEST][PASS] [STEP:chrome-roles-do-not-share-browser-pid] report: chrome src_v3 kept role browser isolation between dev and dev-isolated on legion
```

#### Browser Logs

```text
<empty>
```

---

### 8. ✅ chrome-browser-actions-and-screenshot

- **Duration**: 6.337763036s
- **Report**: chrome src_v3 action flow passed on legion with screenshot capture

#### Logs

```text
INFO: command=open ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: command=set-html ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: command=wait-log ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: remote-console: "page-ready:1773960574583836730"
INFO: command=type-aria ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: command=wait-log ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: remote-console: "page-ready:1773960574583836730"
INFO: remote-console: "typed:dialtone:1773960574583836730"
INFO: command=click-aria ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: command=wait-log ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: remote-console: "page-ready:1773960574583836730"
INFO: remote-console: "typed:dialtone:1773960574583836730"
INFO: remote-console: "clicked:1773960574583836730"
INFO: command=screenshot ok service_pid=3552 browser_pid=14156 current_url=about:blank tabs=1
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0086s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0089s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0090s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0090s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0091s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0091s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0092s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0093s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0093s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0094s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0095s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 action flow passed on legion with screenshot capture
PASS: [TEST][PASS] [STEP:chrome-browser-actions-and-screenshot] report: chrome src_v3 action flow passed on legion with screenshot capture
```

#### Browser Logs

```text
<empty>
```

#### Screenshots

![chrome_src_v3_actions_legion.png](screenshots/chrome_src_v3_actions_legion.png)

---

### 9. ✅ chrome-logs-and-status

- **Duration**: 822.797352ms
- **Report**: chrome src_v3 logs captured and service remains healthy on legion (browser_pid=14156)

#### Logs

```text
INFO: REMOTE_STDOUT_BEGIN
INFO: REMOTE_STDOUT #< CLIXML
INFO: REMOTE_STDOUT [T+0203s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0204s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0205s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0206s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0207s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0208s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0209s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0211s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0212s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0213s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0214s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0215s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0216s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0217s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0527s|INFO|src/plugins/chrome/src_v3/browser.go:392] chrome src_v3 closing browser intentionally
INFO: REMOTE_STDOUT [T+0547s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: shutdown
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:105] chrome src_v3 daemon ready role=dev host=legion nats=nats://127.0.0.1:47222 chrome=19464
INFO: REMOTE_STDOUT [T+0000s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0001s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0020s|INFO|src/plugins/chrome/src_v3/browser.go:123] chrome src_v3 starting browser: C:\Program Files\Google\Chrome\Application\chrome.exe [--remote-debugging-port=19464 --remote-debugging-address=127.0.0.1 --remote-allow-origins=* --user-data-dir=C:\Users\timca\.dialtone\chrome-v3\dev --dialtone-role=dev --dialtone-managed-profile=C:\Users\timca\.dialtone\chrome-v3\dev --no-first-run --no-default-browser-check --disable-gpu about:blank]
INFO: REMOTE_STDOUT [T+0022s|INFO|src/plugins/chrome/src_v3/browser.go:144] chrome src_v3 refined browser PID: 14156
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0023s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0071s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0072s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0073s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0074s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0075s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0076s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0077s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0078s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0079s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>one</h1></body></html>
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0080s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>two</h1></body></html>
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0081s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0082s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: reset
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0083s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0084s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0085s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: eval
INFO: REMOTE_STDOUT [T+0086s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0087s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: data:text/html,<html><body><h1>primary</h1></body></html>
INFO: REMOTE_STDOUT [T+0089s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT [T+0090s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: open
INFO: REMOTE_STDOUT [T+0090s|INFO|src/plugins/chrome/src_v3/daemon.go:145] chrome src_v3 daemon navigating to: about:blank
INFO: REMOTE_STDOUT [T+0091s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: set-html
INFO: REMOTE_STDOUT [T+0091s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0092s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: type-aria
INFO: REMOTE_STDOUT [T+0093s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0093s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: click-aria
INFO: REMOTE_STDOUT [T+0094s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: wait-log
INFO: REMOTE_STDOUT [T+0095s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: screenshot
INFO: REMOTE_STDOUT [T+0096s|INFO|src/plugins/chrome/src_v3/daemon.go:126] chrome src_v3 daemon handle: status
INFO: REMOTE_STDOUT <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDOUT_END
INFO: REMOTE_STDERR_BEGIN
INFO: REMOTE_STDERR #< CLIXML
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7849000, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1dc?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x7ff70c69dde0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc000134ca0, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc000134c88, {0xc000200000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc000134c88, {0xc000200000?, 0xa007ff70c72819b?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116338, {0xc000200000?, 0xd14aac0?, 0xc000280008?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0001f6e80)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 23 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR goroutine 5 [sync.Cond.Wait]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0050, 0xb)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0xc0000c4690?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).waitForMsgs(0xc000147108, 0xc0000a6000)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3304 +0xc9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).subscribeLocked in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:4663 +0x419
INFO: REMOTE_STDERR goroutine 7 [chan receive]:
INFO: REMOTE_STDERR dialtone/dev/plugins/chrome/src_v3.publishServiceHeartbeat(0xc000115200, 0xc000147108)
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:554 +0xf5
INFO: REMOTE_STDERR created by dialtone/dev/plugins/chrome/src_v3.runDaemon in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/dialtone/src/plugins/chrome/src_v3/daemon.go:104 +0x8ca
INFO: REMOTE_STDERR goroutine 8 [sync.Cond.Wait, 3 minutes]:
INFO: REMOTE_STDERR sync.runtime_notifyListWait(0xc0000a0150, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/sema.go:606 +0x15d
INFO: REMOTE_STDERR sync.(*Cond).Wait(0x0?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/sync/cond.go:71 +0x73
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*asyncCallbacksHandler).asyncCBDispatcher(0xc000092040)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3185 +0xdc
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.Options.Connect in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:1725 +0x36e
INFO: REMOTE_STDERR goroutine 9 [IO wait]:
INFO: REMOTE_STDERR internal/poll.runtime_pollWait(0x1a7d7848e00, 0x72)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/runtime/netpoll.go:351 +0x85
INFO: REMOTE_STDERR internal/poll.(*pollDesc).wait(0x1f8?, 0x7ff70c69c786?, 0x0)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_poll_runtime.go:84 +0x27
INFO: REMOTE_STDERR internal/poll.waitIO(0x1f8?)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:187 +0x65
INFO: REMOTE_STDERR internal/poll.execIO(0xc0000ca020, 0x7ff70d204048)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:240 +0x13a
INFO: REMOTE_STDERR internal/poll.(*FD).Read(0xc0000ca008, {0xc0000bc000, 0x8000, 0x8000})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/internal/poll/fd_windows.go:533 +0x1ee
INFO: REMOTE_STDERR net.(*netFD).Read(0xc0000ca008, {0xc0000bc000?, 0xa0000c0000adf20?, 0x7ff70c70760c?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/fd_posix.go:68 +0x25
INFO: REMOTE_STDERR net.(*conn).Read(0xc000116360, {0xc0000bc000?, 0xd14aac0?, 0xc0001dc608?})
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/net/net.go:196 +0x45
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*natsReader).Read(0xc0000a0180)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2121 +0x89
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).readLoop(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3260 +0xef
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2556 +0x2b3
INFO: REMOTE_STDERR goroutine 10 [chan receive]:
INFO: REMOTE_STDERR github.com/nats-io/nats%2ego.(*Conn).flusher(0xc0000b8008)
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:3702 +0xe9
INFO: REMOTE_STDERR created by github.com/nats-io/nats%2ego.(*Conn).processConnectInit in goroutine 1
INFO: REMOTE_STDERR C:/Users/timca/go/pkg/mod/github.com/nats-io/nats.go@v1.48.0/nats.go:2557 +0x2f9
INFO: REMOTE_STDERR <Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04"><Obj S="progress" RefId="0"><TN RefId="0"><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN><MS><I64 N="SourceId">1</I64><PR N="Record"><AV>Preparing modules for first use.</AV><AI>0</AI><Nil /><PI>-1</PI><PC>-1</PC><T>Completed</T><SR>-1</SR><SD> </SD></PR></MS></Obj></Objs>
INFO: REMOTE_STDERR_END
INFO: report: chrome src_v3 logs captured and service remains healthy on legion (browser_pid=14156)
PASS: [TEST][PASS] [STEP:chrome-logs-and-status] report: chrome src_v3 logs captured and service remains healthy on legion (browser_pid=14156)
```

#### Browser Logs

```text
<empty>
```

---

