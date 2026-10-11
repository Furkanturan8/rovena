SERVICES := identity catalog inventory payment

.PHONY: up down logs test test-integration vet seed-admin seed-catalog seed-inventory

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	cd shared && go test ./...
	@for s in $(SERVICES); do (cd services/$$s && go test ./...) || exit 1; done

# Needs the stack running (make up) and the DB passwords in .env
test-integration:
	@set -a; . ./.env; set +a; \
	(cd shared && ROVENA_TEST_DATABASE_URL="postgres://postgres:$$POSTGRES_PASSWORD@localhost:5433/postgres?sslmode=disable" \
		ROVENA_TEST_AMQP_URL="amqp://$$RABBITMQ_USER:$$RABBITMQ_PASSWORD@localhost:5672/" \
		go test -tags integration -count=1 ./messaging ./outbox) && \
	(cd services/catalog && CATALOG_TEST_DATABASE_URL="postgres://catalog:$$CATALOG_DB_PASSWORD@localhost:5433/catalog?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/identity && IDENTITY_TEST_DATABASE_URL="postgres://identity:$$IDENTITY_DB_PASSWORD@localhost:5433/identity?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/inventory && INVENTORY_TEST_DATABASE_URL="postgres://inventory:$$INVENTORY_DB_PASSWORD@localhost:5433/inventory?sslmode=disable" \
		go test -tags integration -count=1 ./internal/repository/postgres) && \
	(cd services/payment && PAYMENT_TEST_DATABASE_URL="postgres://payment:$$PAYMENT_DB_PASSWORD@localhost:5433/payment?sslmode=disable" \
		ROVENA_TEST_AMQP_URL="amqp://$$RABBITMQ_USER:$$RABBITMQ_PASSWORD@localhost:5672/" \
		go test -tags integration -count=1 ./internal/repository/postgres ./internal/transport/consumer)

vet:
	cd shared && go vet ./...
	@for s in $(SERVICES); do (cd services/$$s && go vet ./...) || exit 1; done

# Needs ADMIN_EMAIL / ADMIN_PASSWORD in .env
seed-admin:
	docker compose --profile tools run --rm identity-seed

# Demo categories and products
seed-catalog:
	docker compose --profile tools run --rm catalog-seed

# Opening stock for every demo variant; needs seed-catalog first
seed-inventory:
	docker compose --profile tools run --rm inventory-seed
