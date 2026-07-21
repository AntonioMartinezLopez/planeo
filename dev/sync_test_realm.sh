#!/bin/bash
set -euo pipefail

# Regenerates services/core/internal/test/testdata/realm.json from the live,
# OpenTofu-provisioned "local" realm, so the integration-test fixture never
# drifts from what `task infra:apply:keycloak` actually provisions (e.g. the
# "groups" client scope/mapper regression this replaces would have been
# caught immediately by this script instead of shipping unnoticed).
#
# Run this manually after any change to infra/modules/keycloak-*/infra/environments/local/keycloak.
# Requires: dev stack up (`task up`) with `task infra:apply:keycloak` already applied.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$REPO_ROOT/services/core/.env"
USERS_FRAGMENT="$REPO_ROOT/services/core/internal/test/testdata/realm-users.json"
OUTPUT_FILE="$REPO_ROOT/services/core/internal/test/testdata/realm.json"
REALM="local"

if [ ! -f "$ENV_FILE" ]; then
    echo "$ENV_FILE not found. Copy services/core/.env.template to services/core/.env first. Exiting."
    exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

for var in KC_BASE_URL KC_ADMIN_CLIENT_ID KC_ADMIN_CLIENT_SECRET KC_ADMIN_USERNAME KC_ADMIN_PASSWORD; do
    if [ -z "${!var:-}" ]; then
        echo "$var not set in $ENV_FILE. Exiting."
        exit 1
    fi
done

echo "Authenticating as $KC_ADMIN_USERNAME against the master realm (admin-dev client)..."
TOKEN=$(curl -sf --request POST \
    --url "$KC_BASE_URL/realms/master/protocol/openid-connect/token" \
    --header 'content-type: application/x-www-form-urlencoded' \
    --data grant_type=password \
    --data "username=$KC_ADMIN_USERNAME" \
    --data "password=$KC_ADMIN_PASSWORD" \
    --data "client_id=$KC_ADMIN_CLIENT_ID" \
    --data "client_secret=$KC_ADMIN_CLIENT_SECRET" \
    | jq -r '.access_token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "Failed to obtain an admin token. Is the dev stack up and infra:apply:keycloak applied? Exiting."
    exit 1
fi

echo "Exporting realm '$REALM' (clients + groups/roles, no users) via partial-export..."
EXPORT=$(curl -sf --request POST \
    --url "$KC_BASE_URL/admin/realms/$REALM/partial-export?exportClients=true&exportGroupsAndRoles=true" \
    --header "Authorization: Bearer $TOKEN")

# partial-export redacts every client secret to the literal string "**********"
# rather than omitting it. The "local" client's secret is intentionally pinned
# to a fixed value (infra/environments/local/keycloak/terraform.tfvars,
# dev/.env.template, testcontainer_keycloak.go) so it must be restored here,
# or the redacted placeholder would silently become the client's real secret
# on import and break every password-grant call in the test suite.
LOCAL_CLIENT_SECRET="t4VlYX9CJIN3VTrlb5nRMXT8Qjr9SBdu"

echo "Stripping volatile ids, Keycloak's auto-generated 'Default Policy'/'Default Permission'"
echo "(js-typed, unreferenced by our own authorization schema - importing them requires the"
echo "deprecated upload-scripts feature and fails a fresh Keycloak with 'Script upload is"
echo "disabled'), restoring the local client's redacted secret, and merging in the static"
echo "dev-users fragment..."
jq --slurpfile users "$USERS_FRAGMENT" --arg localSecret "$LOCAL_CLIENT_SECRET" \
   'walk(if type == "object" then del(.id) else . end)
    | .users = $users[0]
    | .clients |= map(
        if .authorizationSettings.policies then
          .authorizationSettings.policies |= map(select(.name != "Default Policy" and .name != "Default Permission"))
        else . end
      )
    | .clients |= map(if .clientId == "local" then .secret = $localSecret else . end)' \
   <<<"$EXPORT" > "$OUTPUT_FILE"

echo "Wrote $OUTPUT_FILE"
echo "Run 'task test:core:integration' to verify the regenerated fixture."
