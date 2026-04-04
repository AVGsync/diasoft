MIGRATE ?= migrate
MIGRATIONS_DIR ?= migrations
DATABASE_URL ?= postgres://gateway_user:gateway_password@localhost:5432/postgres?sslmode=disable

.PHONY: migrate-up migrate-down migrate-version migrate-force migrate-drop-legacy-table

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-version:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

migrate-force:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" force $(VERSION)

migrate-drop-legacy-table:
	docker compose exec -T postgres psql -U gateway_user -d postgres -c "DROP TABLE IF EXISTS schema_migrations"

.PHONY: build
build:
	go build -v ./cmd/apiserver


.DEFAULT_GOAL := build
