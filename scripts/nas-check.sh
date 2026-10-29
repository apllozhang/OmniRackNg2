#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 1 ]]; then echo 'usage: bash scripts/nas-check.sh NEW_ABSOLUTE_EVIDENCE_DIRECTORY' >&2; exit 2; fi
out=$1
[[ "$out" == /* && ! -e "$out" ]] || { echo 'evidence directory must be absolute and new' >&2; exit 2; }
root=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$out"
(go version; node --version; npm --version; python3 --version; git -C "$root" rev-parse HEAD; git -C "$root" status --porcelain) > "$out/identity.txt"
(cd "$root/backend" && go test -count=1 ./...) > "$out/go.log" 2>&1
(cd "$root/frontend" && npm test) > "$out/frontend.log" 2>&1
(cd "$root/frontend" && npm run build) > "$out/frontend-build.log" 2>&1
(cd "$root" && python3 scripts/api-contract.py --check) > "$out/api-contract.log" 2>&1
(cd "$root/backend" && CGO_ENABLED=0 go build -trimpath -buildvcs=false -o "$out/omnirack-server" ./cmd/server) > "$out/backend-build.log" 2>&1
python3 "$root/scripts/integration-replica.py" --pg-image postgres:16-alpine --go-invariants --business-workflows --server "$out/omnirack-server" --output "$out/integration" > "$out/integration.log" 2>&1
# Marker is written only after every command exits successfully; raw logs remain on failure.
python3 - "$out" <<'PY'
import json,sys
from pathlib import Path
p=Path(sys.argv[1]); (p/'checks.json').write_text(json.dumps({'passed':True,'checks':{'go':'passed','frontend':'passed','api_contract':'passed','integration':'passed'},'limitations':['Not an external LDAP/SMTP/camera test','Not a database restore rehearsal or schema rollback proof']},indent=2)+'\n')
PY
printf 'PASS: %s\n' "$out"
