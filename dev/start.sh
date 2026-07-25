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

# prepare keycloak (OpenTofu-managed realm/org/users)
echo "Preparing Keycloak..."
task infra:apply:keycloak -- -auto-approve

# prepare kafka (OpenTofu-managed topics + SCRAM users)
echo "Waiting for Kafka..."
until docker compose exec -T kafka /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 > /dev/null 2>&1; do
  echo "Waiting for Kafka to be ready..."
  sleep 5
done

echo "Preparing Kafka..."
task infra:apply:kafka -- -auto-approve
task infra:sync-secrets:kafka

# First start of the two Kafka-client sidecars, now that
# task infra:sync-secrets:kafka has written their real SASL passwords into
# dev/.env. They were skipped by the earlier bare `docker compose up`
# because of their `profiles:` entry, so this is their first start, not a
# recreate — they never run with a blank credential.
docker compose up -d --no-deps email-received-producer email-received-consumer

# Wait for all background processes to finish
wait
