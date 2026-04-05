#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

echo "Waiting for PostgreSQL..."
until pg_isready -d "$DATABASE_URL" >/dev/null 2>&1; do
  sleep 2
done

MIGRATIONS_TABLE="file_schema_migrations"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<SQL
CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

LEGACY_VERSION="$(psql "$DATABASE_URL" -tAc "SELECT version FROM schema_migrations LIMIT 1" 2>/dev/null || true)"

if [ -n "$LEGACY_VERSION" ]; then
  echo "Bootstrapping ${MIGRATIONS_TABLE} from legacy schema_migrations version ${LEGACY_VERSION}..."
  for migration in /workspace/gateway-service/migrations/*.up.sql; do
    base_name="$(basename "$migration")"
    prefix="$(printf '%s' "$base_name" | cut -d_ -f1)"
    migration_number="$(printf '%s' "$prefix" | sed 's/^0*//')"
    if [ -z "$migration_number" ]; then
      migration_number=0
    fi

    if [ "$migration_number" -le "$LEGACY_VERSION" ]; then
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATIONS_TABLE} (filename) VALUES ('gateway/${base_name}') ON CONFLICT (filename) DO NOTHING;" >/dev/null
    fi
  done
fi

apply_file() {
  file_path="$1"
  file_name="$2"

  if [ "$(psql "$DATABASE_URL" -tAc "SELECT 1 FROM ${MIGRATIONS_TABLE} WHERE filename = '$file_name'")" = "1" ]; then
    echo "Skipping migration: $file_name"
    return
  fi

  echo "Applying migration: $file_name"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$file_path"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATIONS_TABLE} (filename) VALUES ('$file_name') ON CONFLICT (filename) DO NOTHING;"
}

for migration in /workspace/gateway-service/migrations/*.up.sql; do
  apply_file "$migration" "gateway/$(basename "$migration")"
done

apply_file "/workspace/pubver/migrations/007_public_verification_support.sql" "pubver/007_public_verification_support.sql"

echo "Database migrations finished successfully."
