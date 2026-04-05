#!/bin/sh
set -eu

build_database_url() {
  host="${POSTGRES_HOST:-}"
  port="${POSTGRES_PORT:-5432}"
  database="${POSTGRES_DB:-}"
  user="${POSTGRES_USER:-}"
  password="${POSTGRES_PASSWORD:-}"
  sslmode="${POSTGRES_SSLMODE:-disable}"

  if [ -z "$host" ] || [ -z "$database" ] || [ -z "$user" ]; then
    return 1
  fi

  printf 'postgres://%s:%s@%s:%s/%s?sslmode=%s' \
    "$user" \
    "$password" \
    "$host" \
    "$port" \
    "$database" \
    "$sslmode"
}

if [ -z "${DATABASE_URL:-}" ]; then
  if DATABASE_URL="$(build_database_url)"; then
    export DATABASE_URL
  fi
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

echo "Waiting for PostgreSQL..."
until pg_isready -d "$DATABASE_URL" >/dev/null 2>&1; do
  sleep 2
done

if ! psql "$DATABASE_URL" -Atc "SELECT 1" >/dev/null 2>&1; then
  echo "Failed to connect to PostgreSQL using DATABASE_URL." >&2
  echo "If the postgres volume was initialized with old credentials, either reset the volume or align POSTGRES_PASSWORD with the existing cluster." >&2
  exit 1
fi

MIGRATIONS_TABLE="file_schema_migrations"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<SQL
CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

table_has_column() {
  table_name="$1"
  column_name="$2"
  psql "$DATABASE_URL" -Atc "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '${table_name}' AND column_name = '${column_name}' LIMIT 1;" 2>/dev/null || true
}

table_exists() {
  table_name="$1"
  psql "$DATABASE_URL" -Atc "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '${table_name}' LIMIT 1;" 2>/dev/null || true
}

bootstrap_by_prefix() {
  target_version="$1"
  if [ -z "${target_version:-}" ] || [ "$target_version" -le 0 ]; then
    return
  fi

  echo "Bootstrapping ${MIGRATIONS_TABLE} to version prefix ${target_version}..."
  for migration in /workspace/gateway-service/migrations/*.up.sql; do
    base_name="$(basename "$migration")"
    prefix="$(printf '%s' "$base_name" | cut -d_ -f1)"
    migration_number="$(printf '%s' "$prefix" | sed 's/^0*//')"
    if [ -z "$migration_number" ]; then
      migration_number=0
    fi

    if [ "$migration_number" -le "$target_version" ]; then
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATIONS_TABLE} (filename) VALUES ('gateway/${base_name}') ON CONFLICT (filename) DO NOTHING;" >/dev/null
    fi
  done
}

infer_schema_version() {
  if [ "$(table_exists verification_events)" = "1" ]; then
    printf '15'
    return
  fi
  if [ "$(table_exists batch_record_payloads)" = "1" ]; then
    printf '14'
    return
  fi
  if [ "$(table_exists batch_record_attributes)" = "1" ]; then
    printf '12'
    return
  fi
  if [ "$(table_exists university_signing_keys)" = "1" ]; then
    printf '10'
    return
  fi
  if [ "$(table_exists share_links)" = "1" ]; then
    printf '8'
    return
  fi
  if [ "$(table_exists batch_results)" = "1" ]; then
    printf '7'
    return
  fi
  if [ "$(table_exists diploma_hashes)" = "1" ]; then
    printf '6'
    return
  fi
  if [ "$(table_exists batches)" = "1" ]; then
    printf '4'
    return
  fi
  if [ "$(table_exists api_keys)" = "1" ]; then
    printf '3'
    return
  fi
  if [ "$(table_exists universities)" = "1" ]; then
    printf '2'
    return
  fi
  if [ "$(table_exists platform_admins)" = "1" ]; then
    printf '1'
    return
  fi
  printf '0'
}

legacy_version=""

if [ "$(table_has_column schema_migrations version)" = "1" ]; then
  legacy_version="$(psql "$DATABASE_URL" -tAc "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" 2>/dev/null || true)"
elif [ "$(table_has_column schema_migrations filename)" = "1" ]; then
  legacy_version="$(psql "$DATABASE_URL" -tAc "SELECT COALESCE(MAX((regexp_match(filename, '^gateway/([0-9]+)_'))[1]::bigint), 0) FROM schema_migrations WHERE filename LIKE 'gateway/%'" 2>/dev/null || true)"
elif [ "$(table_has_column go_schema_migrations version)" = "1" ]; then
  legacy_version="$(psql "$DATABASE_URL" -tAc "SELECT COALESCE(MAX(version), 0) FROM go_schema_migrations" 2>/dev/null || true)"
fi

if [ -n "${legacy_version:-}" ] && [ "${legacy_version:-0}" -gt 0 ]; then
  bootstrap_by_prefix "$legacy_version"
else
  inferred_version="$(infer_schema_version)"
  if [ "${inferred_version:-0}" -gt 0 ]; then
    echo "Detected existing schema without tracked file migrations, inferring baseline version ${inferred_version}..."
    bootstrap_by_prefix "$inferred_version"
  fi
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
