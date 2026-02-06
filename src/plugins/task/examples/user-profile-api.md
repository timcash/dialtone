# user-profile-api
### description:
Update user profile API to include new OAuth fields and profile picture.
### tags:
- api
- users
### task-dependencies:
- auth-middleware-v2
### documentation:
- src/users/api.md
### test-condition-1:
GET /users/me returns oauth provider.
### test-condition-2:
PATCH /users/me updates profile pic.
### test-command:
`npm run test:users`
### reviewed-at:
2026-02-06T14:20:00Z
### tested-at:
2026-02-06T14:35:00Z
### last-error-type:
ValidationFailed
### last-error-time:
2026-02-06T14:25:00Z
### log-stream-command:
`@DIALTONE npm run server:users --watch`
