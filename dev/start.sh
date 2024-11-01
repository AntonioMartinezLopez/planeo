#!/bin/bash

# Function to handle Ctrl+C
cleanup() {
  echo "Caught Ctrl+C, stopping all processes..."
  # Stop docker compose here
  docker compose -f ./dev/docker-compose.yaml down
  # Kill all child processes
  pkill -P $$
  exit 0
}

# Trap Ctrl+C (SIGINT)
trap cleanup SIGINT

# start docker containers
docker compose up --build -d --remove-orphans

# URL to check
URL="http://localhost:8080/realms/master/protocol/openid-connect/certs"

# Loop until a 200 response is received
while true; do
  # Send a HEAD request and capture the HTTP status code
  STATUS=$(curl -o /dev/null -s -w "%{http_code}" "$URL")

  # Check if the status code is 200
  if [ "$STATUS" -eq 200 ]; then
    echo "Received 200 response from $URL"
    break
  else
    echo "Waiting for 200 response... Current status: $STATUS"
    sleep 5 # Wait for 5 seconds before checking again
  fi
done

# prepare keycloak
echo "Preparing Keycloak..."
./check_and_install_realm.sh

# start backend
BACKEND_DIR="backend"
echo "Starting backend application"
cd "../$BACKEND_DIR"
air . &
cd ..

# Wait for all background processes to finish
wait
