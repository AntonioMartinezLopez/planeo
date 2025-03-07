# Default values
VERSION ?= latest
DOCKER_REGISTRY ?= ghcr.io/antoniomartinezlopez

# Extract extra arguments by filtering out our primary targets.
ARGS := $(filter-out run test build, $(MAKECMDGOALS))
SERVICE := $(firstword $(ARGS))
TEST_TYPE := $(word 2, $(ARGS))

.PHONY: run test build

## Show help.
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  setup                  - Install all dependencies and prepare dev environment."
	@echo "  login                  - User login to the dev environment."
	@echo "  up                     - Start up the dev environment."
	@echo "  down                   - Shut down the dev environment."
	@echo "  run <service>          - Run a service with Air auto-reloading."
	@echo "  test <service> [unit|integration]"
	@echo "                         - Test a service."
	@echo "  build <service> [VERSION=<tag>]"
	@echo "                         - Build a Docker image for a service."
	@echo "  help                   - Show this help message."

## Install all dependencies and prepare dev environment.
## Usage: make setup
setup:
	@echo "Installing dependencies..."
	cd dev && ./install.sh
	@echo "Setting up the dev environment variables..."
	cp ./dev/.env.template ./dev/.env
	cp ./services/core/db/.envrc.template ./services/core/db/.envrc
	cp ./services/core/.env.template ./services/core/.env
	cp ./services/email/.env.template ./services/email/.env

## User login to the dev environment.
## Usage: make login
login:
	@echo "Login to the dev environment..."
	cd dev && ./get_test_token.sh

## Start up the dev environment.
## Usage: make up
up:
	@echo "Starting up the dev environment..."
	cd dev && ./start.sh
	@echo "Start migration of core database..."
	cd ./services/core/db && source .envrc && goose up
	@echo "Start migration of email database..."
	cd ./services/email/db && source .envrc && goose up

## Shut down the dev environment.
## Usage: make down
down:
	@echo "Shutting down the dev environment..."
	cd dev && docker compose down -v

## Run a service with Air auto-reloading.
## Usage: make run <service>
run:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make run <service>"; exit 1; \
	fi
	@echo "Running service '$(SERVICE)' with Air auto-reloading..."
	cd services/$(SERVICE) && air -c air.toml

## Test a service.
## Usage: make test <service> [unit|integration]
test:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make test <service> [unit|integration]"; exit 1; \
	fi
	@if [ "$(TEST_TYPE)" = "unit" ]; then \
		echo "Running unit tests for $(SERVICE)..."; \
		go test ./services/$(SERVICE)/... -v -short -count=1; \
	elif [ "$(TEST_TYPE)" = "integration" ]; then \
		echo "Running integration tests for $(SERVICE)..."; \
		go test ./services/$(SERVICE)/... -v -count=1; \
	else \
		echo "Running all tests for $(SERVICE)..."; \
		go test ./services/$(SERVICE)/... -v -count=1; \
	fi

## Build a Docker image for a service.
## Usage: make build <service> [VERSION=<tag>]
build:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make build <service> [VERSION=<tag>]"; exit 1; \
	fi
	@if [ -f services/$(SERVICE)/Dockerfile ]; then \
		echo "Building Docker image for $(SERVICE) with tag $(VERSION)..."; \
		docker build -t $(DOCKER_REGISTRY)/$(SERVICE):$(VERSION) -f services/$(SERVICE)/Dockerfile . ; \
	else \
		echo "No Dockerfile found for $(SERVICE), skipping build..."; \
	fi

# Catch-all rule to ignore extra command-line words so that "app1" isn’t treated as a missing target.
%:
	@:
