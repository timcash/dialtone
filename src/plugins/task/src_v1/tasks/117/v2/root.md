# duckdb-graph-queries
### signature:
- status: wait
- issue: 117
- source: github
- url: https://github.com/timcash/dialtone/issues/117
- synced-at: 2026-02-21T19:50:23Z
### sync:
- github-updated-at: 2026-01-27T23:32:37Z
- last-pulled-at: 2026-02-21T19:50:23Z
- last-pushed-at: 
- github-labels-hash: 
### description:
- research DuckDB graph queries using https://duckdb.org/2025/10/22/duckdb-graph-queries-duckpgq
- integrate graph query capabilities into a simple plugin
- implement demo showing graph-based queries on local data
### tags:
- todo
- duckdb
- graph
- sql
### comments-github:
- none
### comments-outbound:
- TODO: add a bullet comment here to post to GitHub
### task-dependencies:
- none
### documentation:
- https://duckdb.org/2025/10/22/duckdb-graph-queries-duckpgq
### test-condition-1:
- `./dialtone.sh duckdb query "SELECT * FROM graph_scan(...)"` returns valid nodes
### test-command:
- `./dialtone.sh duckdb test-graph`
### reviewed:
### tested:
### last-error-types:
- none
### last-error-times:
- none
### log-stream-command:
- TODO
### last-error-loglines:
- none
### notes:
- title: integrate a simple plugin with duckdb and graph queries
- state: OPEN
- author: timcash
- created-at: 2026-01-27T23:32:37Z
- updated-at: 2026-01-27T23:32:37Z
