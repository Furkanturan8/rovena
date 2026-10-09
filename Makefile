SERVICES := identity

.PHONY: up down logs test vet seed-admin

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	cd shared && go test ./...
	@for s in $(SERVICES); do (cd services/$$s && go test ./...) || exit 1; done

vet:
	cd shared && go vet ./...
	@for s in $(SERVICES); do (cd services/$$s && go vet ./...) || exit 1; done

# Needs ADMIN_EMAIL / ADMIN_PASSWORD in .env
seed-admin:
	docker compose --profile tools run --rm identity-seed
