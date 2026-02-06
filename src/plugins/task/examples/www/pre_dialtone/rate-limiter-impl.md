# rate-limiter-impl
### description:
Implement sliding window rate limiter for API V2.
### tags:
- backend
- performance
### task-dependencies:
- env-config-update
### documentation:
- src/middleware/rate_limit.js
### test-condition-1:
Limit > 100 req/min returns 429.
### test-condition-2:
Headers include Retry-After.
### test-command:
`npm run test:ratelimit`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run server:load --verbose`
### last-error-loglines:
# None
### notes:
