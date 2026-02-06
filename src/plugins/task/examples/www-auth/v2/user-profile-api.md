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
- GET /users/me returns oauth provider.
### test-condition-2:
- PATCH /users/me updates profile pic.
### test-command:
- `npm run test:users`
### reviewed:
- USER-1> 2026-02-06T12:30:00Z :: key-sig-3cd
### tested:
- LLM-TEST> 2026-02-06T12:45:00Z :: key-sig-4ef
### last-error-types:
- ValidationFailed
### last-error-times:
- ValidationFailed: 2026-02-06T12:25:00Z
### log-stream-command:
- `@DIALTONE npm run server:users --watch`
### last-error-loglines:
- ValidationFailed: "Usage is not valid URL for: profile_pic_url"
### notes:
Consider image resizing lambda for uploads.
