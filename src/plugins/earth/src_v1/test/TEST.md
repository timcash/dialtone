# Test Report: earth-src-v1

- **Date**: Fri, 27 Feb 2026 22:44:50 PST
- **Total Duration**: 1m17.118561518s

## Summary

- **Steps**: 1 / 2 passed
- **Status**: FAILED

## Details

### 1. ✅ 01-build-earth-ui-and-binary

- **Duration**: 2.118168242s
- **Report**: earth UI and server binary build succeeded

#### Logs

```text
INFO: [ACTION] run earth src_v1 build
INFO: [ACTION] run earth src_v1 go-build
INFO: earth-build-ok
INFO: [ACTION] verify earth UI index contains ARIA labels
INFO: earth-dist-contract-ok
INFO: report: earth UI and server binary build succeeded
PASS: [TEST][PASS] [STEP:01-build-earth-ui-and-binary] report: earth UI and server binary build succeeded
```

#### Browser Logs

```text
<empty>
```

---

### 2. ❌ 02-earth-browser-smoke

- **Duration**: 1m15.000386499s
- **Error**: `step 02-earth-browser-smoke timed out`

#### Errors

```text
FAIL: [TEST][FAIL] [STEP:02-earth-browser-smoke] timed out after 1m15s
```

#### Browser Logs

```text
<empty>
```

---

<!-- DIALTONE_CHROME_REPORT_START -->

## Chrome Report

- hostnode: `legion`
- chrome_count: `unknown`
- error: `remote browser inventory on legion failed: ssh command failed on legion: command error: Process exited with status 1
Output: At line:5 char:3
+ if [ -n "$procs" ]; then
+   ~
Missing '(' after 'if' in if statement.
At line:5 char:5
+ if [ -n "$procs" ]; then
+     ~
Missing type name after '['.
At line:6 char:55
+   printf '%s\n' "$procs" | while IFS= read -r line; do
+                                                       ~
Missing statement body in do loop.
At line:7 char:6
+     [ -n "$line" ] || continue
+      ~
Missing type name after '['.
At line:7 char:20
+     [ -n "$line" ] || continue
+                    ~~
The token '||' is not a valid statement separator in this version.
    + CategoryInfo          : ParserError: (:) [], ParentContainsErrorRecordException
    + FullyQualifiedErrorId : MissingOpenParenthesisInIfStatement`

<!-- DIALTONE_CHROME_REPORT_END -->
