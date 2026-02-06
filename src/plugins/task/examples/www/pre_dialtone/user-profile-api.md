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

### tested-at:

### last-error-type:

### last-error-time:

### log-stream-command:
`@DIALTONE npm run server:users --watch`
