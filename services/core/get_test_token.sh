#!/bin/bash

# Set the .env file path
ENV_FILE=".env"

# Check if the .env file exists
if [ ! -f "$ENV_FILE" ]; then
    echo ".env file not found at $ENV_FILE. Exiting."
    exit 1
fi

# Load variables from .env file
source "$ENV_FILE"

# Check if CLIENT_ID and CLIENT_SECRET are set
if [ -z "$OAUTH_CLIENT_ID" ] || [ -z "$OAUTH_CLIENT_SECRET" ]; then
    echo "CLIENT_ID or CLIENT_SECRET not set in $ENV_FILE. Exiting."
    exit 1
fi

# Prompt for username
read -p "Enter username: " username

# Prompt for password (hidden input)
read -sp "Enter password: " password
echo

# Perform the curl request
response=$(curl --silent --request POST \
    --url "$OAUTH_ISSUER/protocol/openid-connect/token" \
    --header 'content-type: application/x-www-form-urlencoded' \
    --data grant_type=password \
    --data "username=${username}" \
    --data "password=${password}" \
    --data 'scope=openid profile email' \
    --data "client_id=${OAUTH_CLIENT_ID}" \
    --data "client_secret=${OAUTH_CLIENT_SECRET}")

# Check for error in response
if echo "$response" | jq -e '.error' >/dev/null; then
    error=$(echo "$response" | jq -r '.error')
    error_description=$(echo "$response" | jq -r '.error_description')
    echo "Error: $error"
    echo "Error Description: $error_description"
else
    access_token=$(echo "$response" | jq -r '.access_token')
    id_token=$(echo "$response" | jq -r '.id_token')
    echo
    echo "Access Token: $access_token"
    echo
    echo
    echo "ID Token: $id_token"
    echo
fi

echo "check permissions..."
response_permissions=$(curl --silent --request POST \
    --url "$OAUTH_ISSUER/protocol/openid-connect/token" \
    --header "Authorization: Bearer ${access_token}" \
    --data grant_type=urn:ietf:params:oauth:grant-type:uma-ticket \
    --data "response_mode=permissions" \
    --data "audience=${OAUTH_CLIENT_ID}")

echo "Permissions: $response_permissions"
