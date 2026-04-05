#!/bin/sh
set -eu

echo "migrate-go.sh is deprecated. Delegating to deploy/migrate.sh to avoid legacy golang-migrate numbering conflicts." >&2
exec /bin/sh /workspace/deploy/migrate.sh
