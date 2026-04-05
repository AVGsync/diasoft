#!/bin/sh
set -eu

POSTGRES_HOST="${POSTGRES_HOST:-}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DB="${POSTGRES_DB:-}"
POSTGRES_USER="${POSTGRES_USER:-}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
POSTGRES_SSLMODE="${POSTGRES_SSLMODE:-disable}"
DB_MIGRATE_WAIT_TIMEOUT="${DB_MIGRATE_WAIT_TIMEOUT:-60}"
PGCONNECT_TIMEOUT="${PGCONNECT_TIMEOUT:-5}"
export PGCONNECT_TIMEOUT

has_postgres_connection_env() {
  [ -n "$POSTGRES_HOST" ] && [ -n "$POSTGRES_DB" ] && [ -n "$POSTGRES_USER" ]
}

build_database_url() {
  if [ -z "$POSTGRES_HOST" ] || [ -z "$POSTGRES_DB" ] || [ -z "$POSTGRES_USER" ]; then
    return 1
  fi

  printf 'postgres://%s:%s@%s:%s/%s?sslmode=%s' \
    "$POSTGRES_USER" \
    "$POSTGRES_PASSWORD" \
    "$POSTGRES_HOST" \
    "$POSTGRES_PORT" \
    "$POSTGRES_DB" \
    "$POSTGRES_SSLMODE"
}

if has_postgres_connection_env; then
  DATABASE_URL="$(build_database_url)"
  export DATABASE_URL
elif [ -n "${DATABASE_URL:-}" ]; then
  export DATABASE_URL
fi

if [ -z "${DATABASE_URL:-}" ] && { [ -z "$POSTGRES_HOST" ] || [ -z "$POSTGRES_DB" ] || [ -z "$POSTGRES_USER" ]; }; then
  echo "DATABASE_URL or POSTGRES_HOST/POSTGRES_DB/POSTGRES_USER is required" >&2
  exit 1
fi

if [ -n "$POSTGRES_HOST" ]; then
  export PGHOST="$POSTGRES_HOST"
fi
if [ -n "$POSTGRES_PORT" ]; then
  export PGPORT="$POSTGRES_PORT"
fi
if [ -n "$POSTGRES_DB" ]; then
  export PGDATABASE="$POSTGRES_DB"
fi
if [ -n "$POSTGRES_USER" ]; then
  export PGUSER="$POSTGRES_USER"
fi
if [ -n "$POSTGRES_PASSWORD" ]; then
  export PGPASSWORD="$POSTGRES_PASSWORD"
fi
if [ -n "$POSTGRES_SSLMODE" ]; then
  export PGSSLMODE="$POSTGRES_SSLMODE"
fi

psql_cmd() {
  if [ -n "${DATABASE_URL:-}" ]; then
    psql -w "$DATABASE_URL" "$@"
    return
  fi

  psql -w "$@"
}

psql_scalar() {
  if [ -n "${DATABASE_URL:-}" ]; then
    psql -w "$DATABASE_URL" -Atc "$1" 2>/dev/null || true
    return
  fi

  psql -w -Atc "$1" 2>/dev/null || true
}

echo "Waiting for PostgreSQL at ${POSTGRES_HOST:-database-host}:${POSTGRES_PORT}..."
elapsed=0
until pg_isready -h "${POSTGRES_HOST:-localhost}" -p "$POSTGRES_PORT" -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" >/dev/null 2>&1; do
  elapsed=$((elapsed + 2))
  if [ "$elapsed" -ge "$DB_MIGRATE_WAIT_TIMEOUT" ]; then
    echo "Timed out waiting for PostgreSQL after ${DB_MIGRATE_WAIT_TIMEOUT}s." >&2
    echo "Check POSTGRES_HOST/POSTGRES_PORT and whether the postgres container is accepting TCP connections." >&2
    exit 1
  fi
  sleep 2
done

if ! psql_cmd -Atc "SELECT 1" >/dev/null 2>&1; then
  echo "Failed to connect to PostgreSQL with the configured credentials." >&2
  echo "If the postgres volume was initialized with old credentials, either reset the volume or align POSTGRES_PASSWORD with the existing cluster." >&2
  exit 1
fi

MIGRATIONS_TABLE="file_schema_migrations"

psql_cmd -v ON_ERROR_STOP=1 <<SQL
CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

table_has_column() {
  table_name="$1"
  column_name="$2"
  psql_scalar "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '${table_name}' AND column_name = '${column_name}' LIMIT 1;"
}

table_exists() {
  table_name="$1"
  psql_scalar "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '${table_name}' LIMIT 1;"
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
      psql_cmd -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATIONS_TABLE} (filename) VALUES ('gateway/${base_name}') ON CONFLICT (filename) DO NOTHING;" >/dev/null
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
  legacy_version="$(psql_scalar "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")"
elif [ "$(table_has_column schema_migrations filename)" = "1" ]; then
  legacy_version="$(psql_scalar "SELECT COALESCE(MAX((regexp_match(filename, '^gateway/([0-9]+)_'))[1]::bigint), 0) FROM schema_migrations WHERE filename LIKE 'gateway/%'")"
elif [ "$(table_has_column go_schema_migrations version)" = "1" ]; then
  legacy_version="$(psql_scalar "SELECT COALESCE(MAX(version), 0) FROM go_schema_migrations")"
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
  psql_cmd -v ON_ERROR_STOP=1 -f "$file_path"
  psql_cmd -v ON_ERROR_STOP=1 -c "INSERT INTO ${MIGRATIONS_TABLE} (filename) VALUES ('$file_name') ON CONFLICT (filename) DO NOTHING;"
}

for migration in /workspace/gateway-service/migrations/*.up.sql; do
  apply_file "$migration" "gateway/$(basename "$migration")"
done

apply_file "/workspace/pubver/migrations/007_public_verification_support.sql" "pubver/007_public_verification_support.sql"

echo "Database migrations finished successfully."
