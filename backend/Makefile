SERVICES := identity catalog inventory payment order

.PHONY: up down prod-up logs build lint tidy-check deps e2e test test-env test-integration vet seed-admin seed-catalog seed-inventory

up:
	docker compose up -d --build

down:
	docker compose down

# Production server (docs/deploy.md). The gateway listens on 127.0.0.1:18080 only; the server's nginx adds HTTPS in front of it.
prod-up: up

logs:
	docker compose logs -f

MODULES := shared $(addprefix services/,$(SERVICES))

build:
	@for m in $(MODULES); do (cd $$m && go build ./...) || exit 1; done

# Needs golangci-lint v2 (brew install golangci-lint). Config: .golangci.yml
lint:
	@for m in $(MODULES); do echo "== $$m"; (cd $$m && golangci-lint run ./...) || exit 1; done

# Fails when a go.mod/go.sum is not exactly what `go mod tidy` would write, and checks downloaded modules
tidy-check:
	@for m in $(MODULES); do echo "== $$m"; (cd $$m && go mod tidy -diff && go mod verify) || exit 1; done

# Brings go.mod/go.sum of every module up to date after dependencies of shared/ changed.
deps:
	@for m in $(MODULES); do (cd $$m && go mod tidy) || exit 1; done

# End-to-end and fault-injection scenarios (about 8 minutes). Starts the Order service with the short recovery
# timings of docker-compose.e2e.yml and puts the normal ones back afterwards. Stops and starts containers.
e2e:
	@test -x .venv/bin/python || (python3 -m venv .venv && .venv/bin/pip install -q requests)
	docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d order
	@until curl -fs localhost:8086/readyz >/dev/null; do sleep 1; done
	@.venv/bin/python e2e/run.py $(SCENARIO); rc=$$?; docker compose up -d order >/dev/null; exit $$rc

test:
	cd shared && go test ./...
	@for s in $(SERVICES); do (cd services/$$s && go test ./...) || exit 1; done

# Integration tests that write outbox events (shared, payment, order) run against their own databases
# (orders_test, payment_test) and their own RabbitMQ vhost (test). The running services would otherwise
# publish those test events through their outbox relays, charge made-up orders and fill dead-letter queues.
# Idempotent; needs the stack running (make up) and the passwords in .env.
test-env:
	@set -a; . ./.env; set +a; \
	docker compose exec -T postgres psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='orders_test'" | grep -q 1 || \
		docker compose exec -T postgres psql -U postgres -q -c "CREATE DATABASE orders_test OWNER orders"; \
	docker compose exec -T postgres psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='payment_test'" | grep -q 1 || \
		docker compose exec -T postgres psql -U postgres -q -c "CREATE DATABASE payment_test OWNER payment"; \
	(cd services/order && DATABASE_URL="postgres://orders:$$ORDER_DB_PASSWORD@localhost:5433/orders_test?sslmode=disable" \
		go run ./cmd/migrate -action up) >/dev/null && \
	(cd services/payment && DATABASE_URL="postgres://payment:$$PAYMENT_DB_PASSWORD@localhost:5433/payment_test?sslmode=disable" \
		go run ./cmd/migrate -action up) >/dev/null && \
	curl -fsS -o /dev/null -u "$$RABBITMQ_USER:$$RABBITMQ_PASSWORD" -X PUT http://localhost:15672/api/vhosts/test && \
	curl -fsS -o /dev/null -u "$$RABBITMQ_USER:$$RABBITMQ_PASSWORD" -X PUT -H 'content-type: application/json' \
		-d '{"configure":".*","write":".*","read":".*"}' http://localhost:15672/api/permissions/test/$$RABBITMQ_USER

test-integration: test-env
	@set -a; . ./.env; set +a; \
	(cd shared && ROVENA_TEST_DATABASE_URL="postgres://postgres:$$POSTGRES_PASSWORD@localhost:5433/postgres?sslmode=disable" \
		ROVENA_TEST_AMQP_URL="amqp://$$RABBITMQ_USER:$$RABBITMQ_PASSWORD@localhost:5672/test" \
		go test -tags integration -count=1 ./messaging ./outbox) && \
	(cd services/catalog && CATALOG_TEST_DATABASE_URL="postgres://catalog:$$CATALOG_DB_PASSWORD@localhost:5433/catalog?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/identity && IDENTITY_TEST_DATABASE_URL="postgres://identity:$$IDENTITY_DB_PASSWORD@localhost:5433/identity?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/inventory && INVENTORY_TEST_DATABASE_URL="postgres://inventory:$$INVENTORY_DB_PASSWORD@localhost:5433/inventory?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/payment && PAYMENT_TEST_DATABASE_URL="postgres://payment:$$PAYMENT_DB_PASSWORD@localhost:5433/payment_test?sslmode=disable" \
		ROVENA_TEST_AMQP_URL="amqp://$$RABBITMQ_USER:$$RABBITMQ_PASSWORD@localhost:5672/test" \
		go test -tags integration -count=1 ./internal/repository/postgres ./internal/transport/consumer) && \
	(cd services/order && ORDER_TEST_DATABASE_URL="postgres://orders:$$ORDER_DB_PASSWORD@localhost:5433/orders_test?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres)

vet:
	cd shared && go vet ./...
	@for s in $(SERVICES); do (cd services/$$s && go vet ./...) || exit 1; done

# Needs ADMIN_EMAIL / ADMIN_PASSWORD in .env
seed-admin:
	docker compose --profile tools run --rm identity-seed

# Storefront categories and products (cmd/seed/store.json). Needs MEDIA_BASE_URL in .env.
# `make seed-catalog SEED_FLAGS=-reset` replaces an existing catalog (development only).
seed-catalog:
	docker compose --profile tools run --rm catalog-seed $(SEED_FLAGS)

# Opening stock for every demo variant (cmd/seed/stock.json); needs seed-catalog first.
# `make seed-inventory SEED_FLAGS=-reset` clears existing stock first (development only).
seed-inventory:
	docker compose --profile tools run --rm inventory-seed $(SEED_FLAGS)
