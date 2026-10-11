#!/bin/sh
# Applies every service's migrations to the Postgres published on localhost:5433 (docker compose up postgres).
# Used by CI so the repository integration tests have their schemas; harmless to repeat.
set -eu
cd "$(dirname "$0")/.."
set -a; . ./.env; set +a
for pair in identity:identity:IDENTITY catalog:catalog:CATALOG inventory:inventory:INVENTORY order:orders:ORDER payment:payment:PAYMENT; do
  svc=${pair%%:*}; rest=${pair#*:}; db=${rest%%:*}; var=${rest#*:}
  pw=$(eval echo "\$${var}_DB_PASSWORD")
  echo "migrating $svc"
  (cd "services/$svc" && DATABASE_URL="postgres://$db:$pw@localhost:5433/$db?sslmode=disable" go run ./cmd/migrate -action up >/dev/null)
done
