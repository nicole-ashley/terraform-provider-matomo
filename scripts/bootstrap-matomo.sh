#!/usr/bin/env bash
# Bootstraps a freshly-started Matomo container into a usable state for
# acceptance tests: installs Matomo non-interactively, activates Tag
# Manager, and prints MATOMO_BASE_URL/MATOMO_API_TOKEN in `KEY=value` form
# suitable for appending to $GITHUB_ENV.
#
# Background: Matomo's `console` CLI has no install command and no
# user/token-generation command (verified against the real pinned
# `matomo:latest` (5.11.2) image's `console list` output - see git history
# of this file / the acceptance-testing design doc). The Docker image's
# MATOMO_DATABASE_* env vars only pre-fill the browser install wizard's
# form defaults; they do not create config/config.ini.php, tables, or a
# superuser by themselves.
#
# So this script drives Matomo's own install wizard
# (module=Installation) over plain HTTP, then mints a superuser API token
# via Matomo's own reporting API - all read directly from Matomo 5.x-dev's
# source (plugins/Installation/Controller.php and its Form*.php classes,
# plugins/UsersManager/API.php) rather than assumed:
#
#   - Controller::databaseSetup(), ::tablesCreation(), ::setupSuperUser(),
#     ::firstWebsiteSetup() and ::finished() each only gate on
#     checkPiwikIsNotInstalled()/checkInstallationIsNotExpired() - there is
#     no session-based state machine enforcing that steps be visited in
#     order, and Installation's own Plugin::dispatchIfNotInstalledYet()
#     hook explicitly lets any module=Installation request through
#     regardless of $action once installation_in_progress is set. So each
#     step below can be POSTed directly and independently.
#   - None of the Installation forms (FormDatabaseSetup, FormSuperUser,
#     FormFirstWebsiteSetup, FormDefaultSettings) require a CSRF/nonce
#     token; QuickForm2's validate() only checks that the request method
#     matches the form's method (POST).
#   - tablesCreation requires no form submission at all - creating tables
#     happens on a plain GET once config/config.ini.php exists and no
#     tables are present yet.
#   - The `finished` step matters: it's the only step that clears the
#     `installation_in_progress` config flag that
#     SettingsPiwik::isMatomoInstalled() checks, so skipping it would leave
#     Matomo stuck showing the installer forever.
#   - UsersManager\API::createAppSpecificTokenAuth($userLogin,
#     $passwordConfirmation, $description, ...) re-verifies the given
#     password itself and has no Piwik::checkUserHasSuperUserAccess() (or
#     similar) gate in its body, unlike every other sensitive method in
#     that class - it is Matomo's own supported bootstrap mechanism for
#     turning known credentials into an API token without needing a
#     pre-existing token_auth or session cookie. It's called here with
#     format=json and no token_auth at all.
#
# Since docker-compose.yml publishes the matomo container's port 80 as
# localhost:8080 on the CI runner (which always has curl), this script
# talks to Matomo over that published port directly rather than via
# `docker compose exec` + a guess about whether curl exists inside the
# `matomo:latest` image. `docker compose exec` is still used for the one
# step that has no HTTP-API equivalent: activating the TagManager plugin
# (`console plugin:activate`), a command confirmed to genuinely exist via
# a real `console list` run against the pinned image.
set -euo pipefail

BASE_URL="http://localhost:8080"
SUPERUSER_LOGIN="acceptance-admin"
SUPERUSER_PASSWORD="acceptance-password-not-a-secret"
SUPERUSER_EMAIL="acceptance@example.com"

log() {
  echo "$@" >&2
}

# Waits for Matomo's web server to start responding at all (it may still be
# mid-boot even after `docker compose up --wait` reports its healthcheck
# passed, since Apache and the DB connection are separate readiness
# concerns).
wait_for_http() {
  local attempts=30
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -sS -o /dev/null "$BASE_URL/index.php"; then
      return 0
    fi
    log "Waiting for $BASE_URL to respond (attempt $i/$attempts)..."
    sleep 2
  done
  log "ERROR: $BASE_URL never responded after $attempts attempts"
  exit 1
}

# POSTs one Installation-wizard step and returns its HTTP status code.
# Successful steps in Matomo's Installation\Controller redirect (302) to
# the next step; a 200 means the form was re-rendered, i.e. validation
# failed.
post_install_step() {
  local action="$1"
  shift
  local -a data_args=()
  local kv
  for kv in "$@"; do
    data_args+=(--data-urlencode "$kv")
  done

  local body_file
  body_file="$(mktemp)"
  local code
  code=$(curl -sS -o "$body_file" -w '%{http_code}' \
    -X POST "${data_args[@]}" \
    "$BASE_URL/index.php?module=Installation&action=$action")

  if [[ "$code" != "302" ]]; then
    log "ERROR: Installation step '$action' failed (HTTP $code), expected a redirect (302)."
    log "--- response body for '$action' ---"
    cat "$body_file" >&2
    log "--- end response body ---"
    rm -f "$body_file"
    exit 1
  fi
  rm -f "$body_file"
}

wait_for_http

log "Installation step 1/5: database setup"
post_install_step databaseSetup \
  "host=db" \
  "username=matomo" \
  "password=matomo" \
  "dbname=matomo" \
  "tables_prefix=matomo_" \
  "adapter=PDO\MYSQL" \
  "schema=Mysql" \
  "type=InnoDB" \
  "submit=Next"

log "Installation step 2/5: table creation"
tables_body_file="$(mktemp)"
tables_code=$(curl -sS -o "$tables_body_file" -w '%{http_code}' \
  "$BASE_URL/index.php?module=Installation&action=tablesCreation")
if [[ "$tables_code" != "200" ]]; then
  log "ERROR: tablesCreation step failed (HTTP $tables_code)"
  cat "$tables_body_file" >&2
  rm -f "$tables_body_file"
  exit 1
fi
rm -f "$tables_body_file"

log "Installation step 3/5: superuser setup"
post_install_step setupSuperUser \
  "login=$SUPERUSER_LOGIN" \
  "password=$SUPERUSER_PASSWORD" \
  "password_bis=$SUPERUSER_PASSWORD" \
  "email=$SUPERUSER_EMAIL" \
  "subscribe_newsletter_piwikorg=0" \
  "subscribe_newsletter_professionalservices=0" \
  "submit=Next"

log "Installation step 4/5: first website setup"
post_install_step firstWebsiteSetup \
  "siteName=Acceptance" \
  "url=$BASE_URL/" \
  "timezone=UTC" \
  "ecommerce=0" \
  "submit=Next"

log "Installation step 5/5: finishing installation"
post_install_step finished \
  "submit=1"

log "Activating TagManager plugin"
docker compose exec -T matomo php console plugin:activate TagManager >&2

log "Generating superuser API token"
token_response_file="$(mktemp)"
token_code=$(curl -sS -o "$token_response_file" -w '%{http_code}' \
  -X POST \
  --data-urlencode "module=API" \
  --data-urlencode "method=UsersManager.createAppSpecificTokenAuth" \
  --data-urlencode "format=json" \
  --data-urlencode "userLogin=$SUPERUSER_LOGIN" \
  --data-urlencode "passwordConfirmation=$SUPERUSER_PASSWORD" \
  --data-urlencode "description=terraform-provider-matomo acceptance tests" \
  --data-urlencode "expireHours=0" \
  "$BASE_URL/index.php")
token_response="$(cat "$token_response_file")"
rm -f "$token_response_file"

if [[ "$token_code" != "200" ]]; then
  log "ERROR: token generation request failed (HTTP $token_code)"
  log "$token_response"
  exit 1
fi

TOKEN=$(printf '%s' "$token_response" | sed -n 's/.*"value":"\([^"]*\)".*/\1/p')

if [[ -z "$TOKEN" ]]; then
  log "ERROR: could not extract API token from response:"
  log "$token_response"
  exit 1
fi

echo "MATOMO_BASE_URL=$BASE_URL"
echo "MATOMO_API_TOKEN=$TOKEN"
