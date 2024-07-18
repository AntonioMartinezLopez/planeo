#!/bin/bash

# Function to handle Ctrl+C
cleanup() {
    echo "Caught Ctrl+C, stopping all processes..."
    # Stop docker compose here
    docker compose down -v
    # Kill all child processes
    pkill -P $$
    exit 0
}

# Trap Ctrl+C (SIGINT)
trap cleanup SIGINT

# start docker containers
docker compose up --build -d --remove-orphans

# start backend
BACKEND_DIR="backend"
echo "Starting backend application"
cd "../$BACKEND_DIR"
air . &

# Wait for all background processes to finish
wait