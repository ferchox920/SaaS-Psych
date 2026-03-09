MIGRATE ?= migrate
MIGRATE_VERSION ?= v4.18.3
APP_DIR ?= apps/api
MIGRATIONS_DIR ?= apps/api/migrations
DATABASE_URL ?= postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable
RUN_PG_INTEGRATION ?= 1
POSTGRES_VOLUME ?= sessionflow_postgres_data

.PHONY: tools db-up db-down db-reset-local integration-preflight migrate-up migrate-down migrate-down-1 migrate-status db-prepare test test-integration-db

tools:
	@MIGRATE_VERSION=$(MIGRATE_VERSION) bash scripts/install_migrate.sh

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose stop postgres redis

db-reset-local:
	docker compose down --remove-orphans
	docker volume rm -f $(POSTGRES_VOLUME)
	docker compose up -d postgres redis

integration-preflight: db-up
	@echo "Checking local Postgres health and credentials..."
	docker compose exec -T postgres pg_isready -U sessionflow -d sessionflow
	docker compose exec -T postgres sh -lc "PGPASSWORD=sessionflow psql -U sessionflow -d sessionflow -c 'select 1' >/dev/null"
	@echo "Checking local Redis responsiveness..."
	docker compose exec -T redis redis-cli ping

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down

migrate-down-1:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-status:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

db-prepare:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

test:
	cd $(APP_DIR) && go test ./...

test-integration-db: integration-preflight db-prepare
	cd $(APP_DIR) && RUN_PG_INTEGRATION=$(RUN_PG_INTEGRATION) DATABASE_URL="$(DATABASE_URL)" go test ./...
