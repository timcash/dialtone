# api-load-test
### description:
Run calibrated load tests against the V2 API endpoints.
### tags:
- testing
- performance
### task-dependencies:
- user-profile-api
- admin-stats-api
- rate-limiter-impl
### documentation:
- tests/load/k6-scripts.md
### test-condition-1:
P95 latency < 200ms at 1000 RPS.
### test-condition-2:
Error rate < 0.1%.
### test-command:
`npm run test:load`
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
`@DIALTONE npm run test:load --dashboard`
### last-error-loglines:
# None
### notes:
