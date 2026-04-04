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

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

apply_file() {
  file_path="$1"
  file_name="$2"

  if [ "$(psql "$DATABASE_URL" -tAc "SELECT 1 FROM schema_migrations WHERE filename = '$file_name'")" = "1" ]; then
    echo "Skipping migration: $file_name"
    return
  fi

  echo "Applying migration: $file_name"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$file_path"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "INSERT INTO schema_migrations (filename) VALUES ('$file_name') ON CONFLICT (filename) DO NOTHING;"
}

for migration in /workspace/gateway-service/migrations/*.up.sql; do
  apply_file "$migration" "gateway/$(basename "$migration")"
done

apply_file "/workspace/pubver/migrations/007_public_verification_support.sql" "pubver/007_public_verification_support.sql"

echo "Database migrations finished successfully."
