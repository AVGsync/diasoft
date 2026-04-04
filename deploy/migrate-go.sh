#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

case "$DATABASE_URL" in
  *\?*)
    GO_MIGRATE_DATABASE_URL="${DATABASE_URL}&x-migrations-table=go_schema_migrations"
    ;;
  *)
    GO_MIGRATE_DATABASE_URL="${DATABASE_URL}?x-migrations-table=go_schema_migrations"
    ;;
esac

echo "Installing golang-migrate..."
command -v migrate >/dev/null 2>&1 || {
  echo "migrate binary is not available in the container" >&2
  exit 1
}

echo "Waiting for PostgreSQL..."
until pg_isready -d "$DATABASE_URL" >/dev/null 2>&1; do
  sleep 2
done

legacy_table_exists="$(psql "$DATABASE_URL" -Atc "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'schema_migrations' AND column_name = 'filename' LIMIT 1;")"
go_table_exists="$(psql "$DATABASE_URL" -Atc "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'go_schema_migrations' AND column_name = 'version' LIMIT 1;")"

if [ "${legacy_table_exists:-}" = "1" ] && [ "${go_table_exists:-}" != "1" ]; then
  current_version="$(psql "$DATABASE_URL" -Atc "SELECT COALESCE(MAX((regexp_match(filename, '^gateway/([0-9]+)_'))[1]::bigint), 0) FROM schema_migrations WHERE filename LIKE 'gateway/%';")"

  if [ "${current_version:-0}" -gt 0 ]; then
    echo "Seeding go_schema_migrations with baseline version ${current_version} from legacy schema_migrations..."
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<SQL
CREATE TABLE IF NOT EXISTS go_schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    dirty BOOLEAN NOT NULL
);
INSERT INTO go_schema_migrations (version, dirty)
VALUES (${current_version}, FALSE)
ON CONFLICT (version) DO UPDATE SET dirty = EXCLUDED.dirty;
SQL
  fi
fi

echo "Running golang-migrate..."
migrate -path /workspace/gateway-service/migrations -database "$GO_MIGRATE_DATABASE_URL" up

echo "Database migrations finished successfully."
