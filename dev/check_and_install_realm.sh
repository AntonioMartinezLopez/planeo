#!/bin/bash

# Load environment variables from the .env file
source ../services/core/.env

# Function to authenticate against keycloak as admin, returns access token or throws error
get_access_token() {
    
    access_token=$(curl --silent --request POST \
        --url "$KC_BASE_URL/realms/master/protocol/openid-connect/token" \
        --header 'content-type: application/x-www-form-urlencoded' \
        --data grant_type=password \
        --data "username=${KC_ADMIN_USERNAME}" \
        --data "password=${KC_ADMIN_PASSWORD}" \
        --data 'scope=openid profile email' \
        --data "client_id=${KC_ADMIN_CLIENT_ID}" \
        --data "client_secret=${KC_ADMIN_CLIENT_SECRET}" | jq -r '.access_token')

    if [ -z "$access_token" ]; then
        echo "Failed to authenticate against Keycloak."
        exit 1
    fi

    echo "$access_token"
}

# Function to check if Keycloak is up
check_keycloak_up() {

    response=$(curl --write-out '%{http_code}' --silent --output /dev/null "$KC_BASE_URL/realms/master/protocol/openid-connect/certs")
    
    if [ "$response" -eq 200 ]; then
        echo "Keycloak is up."
        return 0
    else
        echo "Keycloak is not reachable."
        return 1
    fi
}

# function to create a new client in the master realm using client.json in dev/auth/admin
prepare_admin_client() {

    access_token=$(curl --silent --request POST \
        --url "$KC_BASE_URL/realms/master/protocol/openid-connect/token" \
        --header 'content-type: application/x-www-form-urlencoded' \
        --data grant_type=password \
        --data "username=${KC_ADMIN_USERNAME}" \
        --data "password=${KC_ADMIN_PASSWORD}" \
        --data "client_id=admin-cli" \
        --data 'scope=openid profile email' | jq -r '.access_token')

    if [ -z "$access_token" ]; then
        echo "Failed to authenticate against Keycloak."
        exit 1
    fi

    clients=$(curl --silent --request GET \
        -H "Authorization: Bearer $access_token" \
        "$KC_BASE_URL/admin/realms/master/clients")

    client_exists=$(echo "$clients" | jq 'any(.[]; .clientId == "admin-dev")')

    if [ "$client_exists" == "true" ]; then
        echo "Admin client exists."
        return 0 # true
    else
        echo "Admin client does not exist."

        response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$KC_BASE_URL/admin/realms/master/clients" \
            -H "Authorization: Bearer $access_token" \
            -H "Content-Type: application/json" \
            -d @../auth/admin/client.json)

        if [ "$response" -eq 201 ]; then
            echo "Admin client created successfully."
            return 0
        else
            echo "Failed to create admin client. HTTP status code: $response"
            exit 1
        fi
    fi
}

# Function to check if the local realm exists
check_realm_exists() {

    access_token=$(get_access_token)

    realms=$(curl --silent --request GET \
        -H "Authorization: Bearer $access_token" \
        "$KC_BASE_URL/admin/realms")

    # Check if the "local" realm exists in the realms array and return a boolean value
    realm_exists=$(echo "$realms" | jq 'any(.[]; .realm == "local")')

    if [ "$realm_exists" == "true" ]; then
        echo "Realm 'local' exists."
        return 0 # true
    else
        echo "Realm 'local' does not exist."
        return 1 # false
    fi
}

# Function to create the local realm
create_realm() {

    access_token=$(get_access_token)

    response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$KC_BASE_URL/admin/realms" \
        -H "Authorization: Bearer $access_token" \
        -H "Content-Type: application/json" \
        -d @../auth/local/realm.json)

    if [ "$response" -eq 201 ]; then
        echo "Realm 'local' created successfully."
    else
        echo "Failed to create realm 'local'. HTTP status code: $response"
    fi
}

# Main script execution
if check_keycloak_up; then
    prepare_admin_client
    if ! check_realm_exists; then
        create_realm
    fi
else
    echo "Cannot proceed as Keycloak is not up."
fi
