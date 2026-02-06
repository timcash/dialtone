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
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run test:auth:watch`
### last-error-loglines:
# None
### notes:
