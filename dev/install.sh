#!/bin/bash

# Check if Go is installed
if ! command -v go &>/dev/null; then
    echo "Go is not installed. Please install Go first."
    exit 1
fi

# Get and echo the Go version
GO_VERSION=$(go version)
echo "Go installed: $GO_VERSION"

# Check if .bashrc file exists
BASHRC_FILE="$HOME/.bashrc"
if [ ! -f "$BASHRC_FILE" ]; then
    echo ".bashrc file not found in the home directory. Creating .bashrc file."
    touch "$BASHRC_FILE"
fi

# Check for the required environment variable exports in .bashrc
GOPATH_EXPORT="export GOPATH=\$HOME/go"
PATH_EXPORT="export PATH=\$PATH:/usr/local/go/bin:\$GOPATH/bin"

add_to_bashrc=false

if ! grep -q "$GOPATH_EXPORT" "$BASHRC_FILE"; then
    echo "$GOPATH_EXPORT" >>"$BASHRC_FILE"
    echo "Added '$GOPATH_EXPORT' to .bashrc"
    add_to_bashrc=true
fi

if ! grep -q "$PATH_EXPORT" "$BASHRC_FILE"; then
    echo "$PATH_EXPORT" >>"$BASHRC_FILE"
    echo "Added '$PATH_EXPORT' to .bashrc"
    add_to_bashrc=true
fi

if [ "$add_to_bashrc" = true ]; then
    echo "The required environment variables have been added to .bashrc."
    echo "Please run 'source ~/.bashrc' to apply the changes."
else
    echo "The required environment variables are already set in .bashrc."
fi

# Check if the air binary is installed
if [ -f "$GOPATH/bin/air" ]; then
    echo "The 'air' binary is installed."
else
    echo "The 'air' binary is not installed."
    echo "Installing 'air' binary..."
    go install github.com/air-verse/air@latest
    if [ $? -eq 0 ]; then
        echo "The 'air' binary has been successfully installed."
    else
        echo "Failed to install the 'air' binary. Please check your Go setup and try again."
        exit 1
    fi
fi

# Check if the goose binary is installed
if [ -f "$GOPATH/bin/goose" ]; then
    echo "Golang goose is installed."
else
    echo "Goose is not installed."
    echo "Installing 'goose' binary..."
    go install github.com/pressly/goose/v3/cmd/goose@latest
    if [ $? -eq 0 ]; then
        echo "The 'goose' binary has been successfully installed."
    else
        echo "Failed to install goose. Please check your Go setup and try again."
        exit 1
    fi
fi

BACKEND_DIR="backend"
echo "Installing Go modules from $BACKEND_DIR/go.mod..."
cd "../$BACKEND_DIR"
go mod tidy
if [ $? -eq 0 ]; then
    echo "Successfully installed all Go modules from $BACKEND_DIR/go.mod."
else
    echo "Failed to install Go modules from $BACKEND_DIR/go.mod. Please check your Go setup and try again."
    exit 1
fi
