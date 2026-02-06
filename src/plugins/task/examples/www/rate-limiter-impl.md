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
2026-02-06T14:10:00Z
### tested-at:
2026-02-06T14:25:00Z
### last-error-type:
RedisConnectionError
### last-error-time:
2026-02-06T14:15:00Z
### log-stream-command:
`@DIALTONE npm run test:security --watch`
