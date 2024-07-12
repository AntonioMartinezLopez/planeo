#!/bin/bash

# Function to handle Ctrl+C
cleanup() {
    echo "Caught Ctrl+C, stopping all processes..."
    # Kill all child processes
    pkill -P $$
    exit
}

# Trap Ctrl+C (SIGINT)
trap cleanup SIGINT

# start backend
BACKEND_DIR="backend"
echo "Starting backend application"
cd "../$BACKEND_DIR"
air . &

# Wait for all background processes to finish
wait