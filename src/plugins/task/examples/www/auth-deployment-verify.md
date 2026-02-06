# auth-deployment-verify
### description:
Verify the new auth service works in the staging environment.
### tags:
- deployment
- verification
### task-dependencies:
- auth-tests-fix
- api-load-test
- security-scan-report
- auth-docs-update
### documentation:
- deploy/staging/README.md
### test-condition-1:
`/health` endpoint returns 200 OK.
### test-condition-2:
Can exchange OAuth code for token.
### test-command:
`npm run verify:staging`
### reviewed-at:

### tested-at:

### last-error-type:
ConnectionRefused
### last-error-time:
2026-02-06T13:50:00Z
### log-stream-command:
`@DIALTONE run verify:staging --verbose`
