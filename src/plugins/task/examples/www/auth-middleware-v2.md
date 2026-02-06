# auth-middleware-v2
### description:
Refactor the authentication middleware to support JWT and OAuth2 flows.
### tags:
- auth
- refactor
- security
### task-dependencies:
- env-config-update
- database-migration-users
### documentation:
- src/auth/README.md
- src/auth/DESIGN.md
### test-condition-1:
All unit tests in `src/auth/tests` pass.
### test-condition-2:
Integration smoke test `npm run test:auth:integration` passes.
### test-command:
`npm run test:auth`
### reviewed-at:
2026-02-06T14:00:00Z
### tested-at:
2026-02-06T14:15:00Z
### last-error-type:
None
### last-error-time:
N/A
### log-stream-command:
`@DIALTONE npm run build:auth --watch`
