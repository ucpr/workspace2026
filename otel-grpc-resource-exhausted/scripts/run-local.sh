#!/usr/bin/env bash
# Run the experiment without Docker: bin/receiver (4 MiB limit) + bin/server + bin/loadgen.
# Usage: scripts/run-local.sh [attr_bytes] [rps] [stall]
set -euo pipefail
cd "$(dirname "$0")/.."

ATTR_BYTES=${1:-8000}
RPS=${2:-10}
STALL=${3:-10s}
URL="http://localhost:8080/work?children=5&attr_bytes=${ATTR_BYTES}"
LOG_DIR=${LOG_DIR:-$(mktemp -d)}

go build -o bin/ ./cmd/...

./bin/receiver >"$LOG_DIR/receiver.log" 2>&1 &
RECV=$!
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 OTEL_EXPORTER_OTLP_INSECURE=true \
  ./bin/server >"$LOG_DIR/server.log" 2>&1 &
SRV=$!
trap 'kill $SRV $RECV 2>/dev/null || true' EXIT
until curl -sf localhost:8080/healthz >/dev/null; do sleep 0.2; done

echo "== A: attr_bytes=${ATTR_BYTES} rps=${RPS}, no stall"
./bin/loadgen -url "$URL" -rps "$RPS" -duration 15s 2>/dev/null
sleep 6
grep -aE '\[export\]|\[otel\]' "$LOG_DIR/server.log" | tr -d "\000" || true
: >"$LOG_DIR/server.log"

echo "== B: attr_bytes=${ATTR_BYTES} rps=${RPS}, stall ${STALL}"
(sleep 3 && curl -s -XPOST "localhost:8080/admin/stall?d=${STALL}" >/dev/null) &
./bin/loadgen -url "$URL" -rps "$RPS" -duration 20s 2>/dev/null
sleep 6
grep -aE '\[export\]|\[otel\]|\[stall\]' "$LOG_DIR/server.log" | tr -d "\000" || true

echo "logs: $LOG_DIR"
