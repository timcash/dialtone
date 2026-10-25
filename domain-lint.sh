#!/usr/bin/env bash
# Domain-language gate: fail if a banned synonym appears in src/.
# The vocabulary lives in goal.md ("Domain Language"). A line that must keep an old
# name for wire / CLI compatibility, or that names an iroh library type, ends with
# a `// compat` marker.
set -euo pipefail
cd "$(dirname "$0")"

# banned pattern (extended regex, case-insensitive) => word to use instead
RULES=(
  'heartbeat|(^|[^a-z0-9])hb([^a-z0-9]|$)|_hb([^a-z0-9]|$)|(^|[^a-z0-9])hb_=beat'
  'telemetry|host_?resources?=stats'
  'fleet|swarm=mesh'
  'artifact=blob'
  '(^|[^a-z0-9])cas([^a-z0-9]|$)=store'
  'blob_?cache|cache_dir|cache_root=store'
  'sub_?process|managed_?process|supervised_?process|(^|[^a-z])worker=process'
  '(^|[^0-9a-f])e2e([^0-9a-f]|$)|test[-_]tick|capability[-_ ]tick=probe'
  'dashboard=hud'
  'endpoint_ids?=node_id'
  'seed_peers?|bootstrap=seed / invite'
  'password=secret'
  'invite[_ ]?tickets?=invite'
)

status=0
show="${DOMAIN_LINT_SHOW:-5}"
files=$(ls src/*.rs src/hud/*.html src/hud/*.css src/hud/*.js | grep -v 'src/h3_land.rs')
for rule in "${RULES[@]}"; do
  pattern="${rule%=*}"
  use="${rule##*=}"
  hits=$(grep -nEi "$pattern" $files | grep -v '// compat' || true)
  if [ -n "$hits" ]; then
    count=$(printf '%s\n' "$hits" | wc -l | tr -d ' ')
    echo "domain-lint: $count line(s) use a banned word; say \"$use\" (pattern: $pattern)"
    if [ "$show" -gt 0 ]; then printf '%s\n' "$hits" | head -n "$show" | cut -c1-160 | sed 's/^/    /'; fi
    status=1
  fi
done
[ "$status" -eq 0 ] && echo "domain-lint: clean"
exit "$status"
