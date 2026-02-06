# env-config-update
### description:
Update environment variables for the new V2 Auth Service.
### tags:
- config
- devops
### task-dependencies:
# None
### documentation:
- src/config/README.md
### test-condition-1:
`process.env.AUTH_V2_ENABLED` is true.
### test-condition-2:
Secret keys are loaded from Vault.
### test-command:
`npm run config:validate`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run config:print`
### last-error-loglines:
# None
### notes:
