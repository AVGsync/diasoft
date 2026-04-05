#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
  echo "command is required" >&2
  exit 1
fi

if [ "${INFISICAL_ENABLED:-false}" != "true" ]; then
  exec "$@"
fi

if ! command -v infisical >/dev/null 2>&1; then
  echo "infisical CLI is required in the runtime image" >&2
  exit 1
fi

if [ -z "${INFISICAL_PROJECT_ID:-}" ]; then
  echo "INFISICAL_PROJECT_ID is required when INFISICAL_ENABLED=true" >&2
  exit 1
fi

INFISICAL_API_URL="${INFISICAL_API_URL:-https://app.infisical.com}"
INFISICAL_ENV_SLUG="${INFISICAL_ENV_SLUG:-prod}"
INFISICAL_SECRET_PATH="${INFISICAL_SECRET_PATH:-/}"
INFISICAL_DISABLE_UPDATE_CHECK="${INFISICAL_DISABLE_UPDATE_CHECK:-true}"

if [ -z "${INFISICAL_TOKEN:-}" ]; then
  if [ -n "${INFISICAL_SERVICE_TOKEN:-}" ]; then
    INFISICAL_TOKEN="$INFISICAL_SERVICE_TOKEN"
    export INFISICAL_TOKEN
  elif [ -n "${INFISICAL_UNIVERSAL_AUTH_CLIENT_ID:-}" ] && [ -n "${INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET:-}" ]; then
    echo "Authenticating to Infisical via Universal Auth..."
    if [ -n "${INFISICAL_AUTH_ORGANIZATION_SLUG:-}" ]; then
      INFISICAL_TOKEN="$(infisical login --method=universal-auth --domain="$INFISICAL_API_URL" --client-id="$INFISICAL_UNIVERSAL_AUTH_CLIENT_ID" --client-secret="$INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET" --organization-slug="$INFISICAL_AUTH_ORGANIZATION_SLUG" --silent --plain)"
    else
      INFISICAL_TOKEN="$(infisical login --method=universal-auth --domain="$INFISICAL_API_URL" --client-id="$INFISICAL_UNIVERSAL_AUTH_CLIENT_ID" --client-secret="$INFISICAL_UNIVERSAL_AUTH_CLIENT_SECRET" --silent --plain)"
    fi
    export INFISICAL_TOKEN
  else
    echo "Set INFISICAL_TOKEN, INFISICAL_SERVICE_TOKEN, or Universal Auth credentials." >&2
    exit 1
  fi
fi

if [ -n "${INFISICAL_SECRET_TAGS:-}" ]; then
  exec infisical run \
    --token="$INFISICAL_TOKEN" \
    --domain="$INFISICAL_API_URL" \
    --projectId="$INFISICAL_PROJECT_ID" \
    --env="$INFISICAL_ENV_SLUG" \
    --path="$INFISICAL_SECRET_PATH" \
    --tags="$INFISICAL_SECRET_TAGS" \
    -- "$@"
fi

exec infisical run \
  --token="$INFISICAL_TOKEN" \
  --domain="$INFISICAL_API_URL" \
  --projectId="$INFISICAL_PROJECT_ID" \
  --env="$INFISICAL_ENV_SLUG" \
  --path="$INFISICAL_SECRET_PATH" \
  -- "$@"
