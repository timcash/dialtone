# Chrome src_v1 Workflow Test

Date: 2026-03-01T20:51:57-08:00

Host: `darkmac,gold,legion`  Role: `dev`  URL: `dialtone.earth`  Attach: `false`

## 9. policy hosts running and gold non-kiosk

Command: `./dialtone.sh chrome src_v1 remote-list --nodes darkmac,gold,legion --origin dialtone --role dev --verbose`

```
NODE       HOST                              PID     PPID    HEADLESS GPU       PORT    ORIGIN     ROLE             COMMAND
----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
darkmac    192.168.4.31                      12681   11940   false    true      59537   Dialtone   dev              /Applications/Google Chrome.app/Contents/MacOS/Google Chrome --remote-debugging-port=59537 --remote-debugging-address=0.0.0.0 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/tim/.dialtone/chrome/dev-profile --new-window --dialtone-origin=true --dialtone-role=dev --kiosk --start-fullscreen https://dialtone.earth
gold       192.168.4.53                      71021   1       false    true      9333    Dialtone   dev              /Applications/Google Chrome.app/Contents/MacOS/Google Chrome --remote-debugging-port=9333 --remote-debugging-address=0.0.0.0 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=/Users/user/.dialtone/chrome-dev-port-9333 --new-window --dialtone-origin=true --dialtone-role=dev https://dialtone.earth
legion     192.168.4.52                      89452   120672  false    true      61003   Dialtone   dev              "C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=61003 --remote-debugging-address=0.0.0.0 --remote-allow-origins=* --no-first-run --no-default-browser-check --user-data-dir=C:/Users/timca/.dialtone/chrome/dev-profile --new-window --dialtone-origin=true --dialtone-role=dev --kiosk --start-fullscreen --flag-switches-begin --flag-switches-end --do-not-de-elevate https://dialtone.earth
[T+0000s|INFO|src/plugins/chrome/cli/remote.go:119] remote-list: 3 process(es) across 3 node(s)
```

Result: PASS

