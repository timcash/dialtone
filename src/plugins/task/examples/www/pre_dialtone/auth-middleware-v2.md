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
Valid JWT allows access.
### test-condition-2:
Expired JWT returns 401.
### test-command:
`npm run test:auth`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run server:auth --watch`
### last-error-loglines:
# None
### notes:
