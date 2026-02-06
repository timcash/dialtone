# rate-limiter-impl
### description:
Implement Redis-based rate limiting for V2 API endpoints.
### tags:
- security
- performance
### task-dependencies:
- env-config-update
### documentation:
- src/security/throttling.md
### test-condition-1:
Limits enforced per IP.
### test-condition-2:
Auth endpoints have stricter limits.
### test-command:
`npm run test:security:ratelimit`
### reviewed-at:

### tested-at:

### last-error-type:

### last-error-time:

### log-stream-command:
`@DIALTONE npm run test:security --watch`
