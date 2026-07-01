#!/usr/bin/env bash
# Bootstraps a freshly-started Matomo container into a usable state for
# acceptance tests: installs Matomo non-interactively, activates Tag
# Manager, and prints MATOMO_BASE_URL/MATOMO_API_TOKEN in `KEY=value` form
# suitable for appending to $GITHUB_ENV.
#
# IMPLEMENTER NOTE: this script's exact `console` subcommands/flags have
# not been verified against a running Matomo instance (this development
# environment cannot run Docker). Verify against the pinned `matomo:latest`
# image's actual `console list` output before trusting this script in CI,
# and fix flag names/order here if they've changed.
set -euo pipefail

MATOMO_CONTAINER="${MATOMO_CONTAINER:-$(docker compose ps -q matomo)}"
SUPERUSER_LOGIN="acceptance-admin"
SUPERUSER_PASSWORD="acceptance-password-not-a-secret"
SUPERUSER_EMAIL="acceptance@example.com"

docker compose exec -T matomo php console core:install \
  --superuser-login="$SUPERUSER_LOGIN" \
  --superuser-password="$SUPERUSER_PASSWORD" \
  --superuser-email="$SUPERUSER_EMAIL" \
  --db-host=db \
  --db-username=matomo \
  --db-password=matomo \
  --db-name=matomo \
  --matomo-url="http://localhost:8080/" \
  --do-not-track=1 \
  --no-interaction

docker compose exec -T matomo php console plugin:activate TagManager

TOKEN=$(docker compose exec -T matomo php console user:generate-api-token \
  "$SUPERUSER_LOGIN" "$SUPERUSER_PASSWORD" | tail -n1 | tr -d '\r\n')

echo "MATOMO_BASE_URL=http://localhost:8080"
echo "MATOMO_API_TOKEN=$TOKEN"
