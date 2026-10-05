.PHONY: build up down repro no-repro local logs

build:
	go build -o bin/ ./cmd/...

up:
	docker compose up -d --build otel-collector api-server

down:
	docker compose down

# ~10 KB/span: 512-span batch after a stall exceeds 4 MiB -> ResourceExhausted
repro:
	scripts/run-compose.sh 12000

# ~7 KB/span: same stall, 512-span batch stays under 4 MiB -> no error
no-repro:
	scripts/run-compose.sh 8000

# Same as repro without Docker (bin/receiver stands in for the collector)
local:
	scripts/run-local.sh 12000

logs:
	docker compose logs -f api-server otel-collector
