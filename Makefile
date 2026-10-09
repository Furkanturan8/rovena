SERVICES := identity catalog

.PHONY: up down logs test test-integration vet seed-admin seed-catalog

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	cd shared && go test ./...
	@for s in $(SERVICES); do (cd services/$$s && go test ./...) || exit 1; done

# Needs the stack running (make up) and CATALOG_DB_PASSWORD in .env
test-integration:
	@set -a; . ./.env; set +a; cd services/catalog && \
	CATALOG_TEST_DATABASE_URL="postgres://catalog:$$CATALOG_DB_PASSWORD@localhost:5433/catalog?sslmode=disable" \
	go test -tags integration -count=1 ./internal/repository/postgres

vet:
	cd shared && go vet ./...
	@for s in $(SERVICES); do (cd services/$$s && go vet ./...) || exit 1; done

# Needs ADMIN_EMAIL / ADMIN_PASSWORD in .env
seed-admin:
	docker compose --profile tools run --rm identity-seed

# Demo categories and products
seed-catalog:
	docker compose --profile tools run --rm catalog-seed
