# env-config-update
### description:
Update environment configuration to support V2 auth services (OAuth2 credentials, JWT secrets).
### tags:
- infrastructure
- config
### task-dependencies:
[]
### documentation:
- infra/prod/env.md
### test-condition-1:
`source .env && echo $JWT_SECRET` returns value.
### test-condition-2:
App boots with new config.
### test-command:
`npm run dev:boot`
### reviewed-at:
2026-02-06T13:05:00Z
### tested-at:
2026-02-06T13:10:00Z
### last-error-type:
MissingKeyError
### last-error-time:
2026-02-06T13:00:00Z
### log-stream-command:
`@DIALTONE env check`
