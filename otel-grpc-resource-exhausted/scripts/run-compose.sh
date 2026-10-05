#!/usr/bin/env bash
# Same experiment as run-local.sh, against the docker compose stack.
# Usage: scripts/run-compose.sh [attr_bytes] [rps] [stall]
set -euo pipefail
cd "$(dirname "$0")/.."

ATTR_BYTES=${1:-12000}
RPS=${2:-10}
STALL=${3:-10s}
URL="http://api-server:8080/work?children=5&attr_bytes=${ATTR_BYTES}"

docker compose up -d --build otel-collector api-server
docker compose build loadgen
until curl -sf localhost:8080/healthz >/dev/null; do sleep 0.5; done

echo "== A: attr_bytes=${ATTR_BYTES} rps=${RPS}, no stall"
SINCE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
docker compose run --rm loadgen -url "$URL" -rps "$RPS" -duration 15s 2>/dev/null
sleep 6
docker compose logs --no-log-prefix --since "$SINCE" api-server | grep -E '\[export\]|\[otel\]' || true

echo "== B: attr_bytes=${ATTR_BYTES} rps=${RPS}, stall ${STALL}"
SINCE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
(sleep 3 && curl -s -XPOST "localhost:8080/admin/stall?d=${STALL}" >/dev/null) &
docker compose run --rm loadgen -url "$URL" -rps "$RPS" -duration 20s 2>/dev/null
sleep 6
docker compose logs --no-log-prefix --since "$SINCE" api-server | grep -E '\[export\]|\[otel\]|\[stall\]' || true
