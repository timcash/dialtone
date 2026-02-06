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
### reviewed:
- USER-1> 2026-02-06T15:30:00Z :: key-sig-rst
- USER-2> 2026-02-06T15:35:00Z :: key-sig-uvw
- LLM-REVIEW> 2026-02-06T15:40:00Z :: key-sig-xyz
### tested:
- LLM-TEST> 2026-02-06T15:45:00Z :: key-sig-123
### last-error-types:
- ConnectionRefused
### last-error-times:
- ConnectionRefused: 2026-02-06T15:20:00Z
### log-stream-command:
`@DIALTONE run verify:staging --verbose`
### last-error-loglines:
- ConnectionRefused: "Error: connect ECONNREFUSED 127.0.0.1:4000"
### notes:
Rollback plan: Revert docker tag to v1.9.4.
