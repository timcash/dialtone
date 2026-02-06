# security-scan-report
### description:
Run automated security scan (SAST/DAST) against V2 code and endpoints.
### tags:
- security
- verification
### task-dependencies:
- user-profile-api
- admin-stats-api
### documentation:
- security/reports/latest.md
### test-condition-1:
No High or Critical vulnerabilities found.
### test-condition-2:
Dependencies are audit-clean.
### test-command:
`npm run audit:security`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run audit:security --watch`
### last-error-loglines:
# None
### notes:
