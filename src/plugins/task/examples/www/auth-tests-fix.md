# auth-tests-fix
### description:
Fix regressions in login and token refresh tests caused by V2 changes.
### tags:
- testing
- bugfix
### task-dependencies:
- auth-middleware-v2
### documentation:
- src/auth/tests/FAILURES.md
### test-condition-1:
`test/login_test.js` passes.
### test-condition-2:
`test/refresh_token_test.js` passes.
### test-command:
`npm run test:auth:flaky`
### reviewed:
- USER-1> 2026-02-06T14:45:00Z :: key-sig-qrs
- LLM-CODE> 2026-02-06T14:50:00Z :: key-sig-tuv
- LLM-TEST> 2026-02-06T14:55:00Z :: key-sig-wxy
### tested:
- LLM-TEST> 2026-02-06T15:00:00Z :: key-sig-zab
### last-error-types:
- TokenMismatchError
- TimeoutError
### last-error-times:
- TokenMismatchError: 2026-02-06T14:20:00Z
- TimeoutError: 2026-02-06T14:20:00Z
### log-stream-command:
`@DIALTONE npm run test:auth:watch`
### last-error-loglines:
- TokenMismatchError: "Error: Token mismatch. Expected JWT, got Basic."
- TimeoutError: "Error: Timeout waiting for refresh."
### notes:
These tests were flaky on CI last week.
