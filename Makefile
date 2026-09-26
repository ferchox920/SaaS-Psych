MIGRATE ?= migrate
MIGRATE_VERSION ?= v4.18.3
APP_DIR ?= apps/api
MIGRATIONS_DIR ?= apps/api/migrations
DATABASE_URL ?= postgres://sessionflow:sessionflow@127.0.0.1:5433/sessionflow?sslmode=disable
RUN_PG_INTEGRATION ?= 1
POSTGRES_VOLUME ?= sessionflow_postgres_data
INTEGRATION_TEST_PACKAGES ?= ./internal/http ./internal/infra/db
GO_TEST_INTEGRATION_FLAGS ?= -count=1

.PHONY: tools db-up db-down db-reset-local integration-preflight integration-recover-local migrate-up migrate-down migrate-down-1 migrate-status db-prepare demo-prepare demo-seed test test-integration-db test-integration-db-reset

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
	@echo "Checking for legacy containers using required ports..."
	@pg_conflicts=""; \
	for name in $$(docker ps --filter publish=5433 --format '{{.Names}}'); do \
		if [ "$$name" != "sessionflow-postgres" ]; then \
			pg_conflicts="$$pg_conflicts $$name"; \
		fi; \
	done; \
	if [ -n "$$pg_conflicts" ]; then \
		echo "Port 5433 is already in use by:$$pg_conflicts"; \
		echo "Stop/remove conflicting legacy containers before running integration tests."; \
		exit 1; \
	fi
	@redis_conflicts=""; \
	for name in $$(docker ps --filter publish=6379 --format '{{.Names}}'); do \
		if [ "$$name" != "sessionflow-redis" ]; then \
			redis_conflicts="$$redis_conflicts $$name"; \
		fi; \
	done; \
	if [ -n "$$redis_conflicts" ]; then \
		echo "Port 6379 is already in use by:$$redis_conflicts"; \
		echo "Stop/remove conflicting legacy containers before running integration tests."; \
		exit 1; \
	fi
	@echo "Checking local Postgres health and credentials..."
	docker compose exec -T postgres pg_isready -U sessionflow -d sessionflow
	docker compose exec -T postgres sh -lc "PGPASSWORD=sessionflow psql -U sessionflow -d sessionflow -c 'select 1' >/dev/null"
	@echo "Checking local Redis responsiveness..."
	docker compose exec -T redis redis-cli ping

integration-recover-local:
	$(MAKE) db-reset-local
	$(MAKE) integration-preflight
	$(MAKE) db-prepare

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

demo-seed:
	docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U sessionflow -d sessionflow < tools/demo/seed.sql

demo-prepare: db-up db-prepare demo-seed

test:
	cd $(APP_DIR) && go test ./...

test-integration-db: integration-preflight db-prepare
	cd $(APP_DIR) && RUN_PG_INTEGRATION=$(RUN_PG_INTEGRATION) DATABASE_URL="$(DATABASE_URL)" go test $(GO_TEST_INTEGRATION_FLAGS) $(INTEGRATION_TEST_PACKAGES)

test-integration-db-reset: integration-recover-local
	cd $(APP_DIR) && RUN_PG_INTEGRATION=$(RUN_PG_INTEGRATION) DATABASE_URL="$(DATABASE_URL)" go test $(GO_TEST_INTEGRATION_FLAGS) $(INTEGRATION_TEST_PACKAGES)
