#!/bin/bash

# Check if Go is installed
if ! command -v go &>/dev/null; then
    echo "Go is not installed. Please install Go first."
    exit 1
fi

# Get and echo the Go version
GO_VERSION=$(go version)
echo "Go installed: $GO_VERSION"
# Determine the operating system
OS_TYPE=$(uname)
echo "Operating System: $OS_TYPE"

# Check if .bashrc or .zshrc file exists
if [ "$SHELL" = "/bin/zsh" ] || [ "$SHELL" = "/usr/bin/zsh" ]; then
    SHELL_RC_FILE="$HOME/.zshrc"
else
    SHELL_RC_FILE="$HOME/.bashrc"
fi

if [ ! -f "$SHELL_RC_FILE" ]; then
    echo "$SHELL_RC_FILE file not found in the home directory. Creating $SHELL_RC_FILE file."
    touch "$SHELL_RC_FILE"
fi

# Check for the required environment variable exports in the shell rc file
GOPATH_EXPORT="export GOPATH=\$HOME/go"
PATH_EXPORT="export PATH=\$PATH:/usr/local/go/bin:\$GOPATH/bin"

add_to_shell_rc=false

if ! grep -q "$GOPATH_EXPORT" "$SHELL_RC_FILE"; then
    echo "$GOPATH_EXPORT" >>"$SHELL_RC_FILE"
    echo "Added '$GOPATH_EXPORT' to $SHELL_RC_FILE"
    add_to_shell_rc=true
fi

if ! grep -q "$PATH_EXPORT" "$SHELL_RC_FILE"; then
    echo "$PATH_EXPORT" >>"$SHELL_RC_FILE"
    echo "Added '$PATH_EXPORT' to $SHELL_RC_FILE"
    add_to_shell_rc=true
fi

if [ "$add_to_shell_rc" = true ]; then
    echo "The required environment variables have been added to $SHELL_RC_FILE."
    echo "Please run 'source $SHELL_RC_FILE' to apply the changes."
else
    echo "The required environment variables are already set in $SHELL_RC_FILE."
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

# check if mockery binary is installed
if [ -f "$GOPATH/bin/mockery" ]; then
    echo "Mockery is installed."
else
    echo "Mockery is not installed."
    echo "Installing 'mockery' binary..."
    go install github.com/vektra/mockery/v2@v2.52.2
    if [ $? -eq 0 ]; then
        echo "The 'mockery' binary has been successfully installed."
    else
        echo "Failed to install mockery. Please check your Go setup and try again."
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
