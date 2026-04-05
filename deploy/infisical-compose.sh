#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"

INFISICAL_ENV_FILE="${INFISICAL_ENV_FILE:-$ROOT_DIR/.env.infisical}"
LOCAL_ENV_FILE="${LOCAL_ENV_FILE:-$ROOT_DIR/.env.machine}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required." >&2
  exit 1
fi

TMP_ENV_FILE="$(mktemp "${TMPDIR:-/tmp}/diasoft-compose.XXXXXX.env")"

cleanup() {
  rm -f "$TMP_ENV_FILE"
}

trap cleanup EXIT INT TERM

if [ -f "$LOCAL_ENV_FILE" ]; then
  cat "$LOCAL_ENV_FILE" > "$TMP_ENV_FILE"
else
  : > "$TMP_ENV_FILE"
fi

if [ -f "$INFISICAL_ENV_FILE" ]; then
  printf "\n" >> "$TMP_ENV_FILE"
  cat "$INFISICAL_ENV_FILE" >> "$TMP_ENV_FILE"
fi

echo "Running docker compose with merged env files..."
cd "$ROOT_DIR"
exec docker compose --env-file "$TMP_ENV_FILE" "$@"
