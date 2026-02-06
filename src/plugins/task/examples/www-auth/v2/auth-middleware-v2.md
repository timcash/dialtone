# auth-middleware-v2
### description:
Implement new support for JWTs from Auth0 provider.
### tags:
- backend
- auth
### task-dependencies:
- env-config-update
- database-migration-users
### documentation:
- src/auth/middleware.js
### test-condition-1:
- Valid JWT allows access.
### test-condition-2:
- Expired JWT returns 401.
### test-command:
- `npm run test:auth`
### reviewed:
- USER-1> 2026-02-06T11:00:00Z :: key-sig-jkl
- LLM-CODE> 2026-02-06T11:05:00Z :: key-sig-mno
- LLM-TEST> 2026-02-06T11:10:00Z :: key-sig-pqr
### tested:
- LLM-TEST> 2026-02-06T11:15:00Z :: key-sig-stu
### last-error-types:
- TokenExpiredError
- JsonWebTokenError
### last-error-times:
- TokenExpiredError: 2026-02-06T10:50:00Z
- JsonWebTokenError: 2026-02-06T10:55:00Z
### log-stream-command:
- `@DIALTONE npm run server:auth --watch`
### last-error-loglines:
- TokenExpiredError: "jwt expired at 2026-02-06T10:00:00Z"
- JsonWebTokenError: "jwt malformed"
### notes:
Key rotation schedule needs to be updated.
