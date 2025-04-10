#!/bin/bash

# Stop on any errors
set -e 

# Build the server
echo "Building DeepseekMCP..."
go build -o bin/deepseek-mcp

# Set API key if provided as argument or prompt for it
if [ -n "$1" ]; then
  export DEEPSEEK_API_KEY="$1"
else
  # Check if DEEPSEEK_API_KEY is already set
  if [ -z "$DEEPSEEK_API_KEY" ]; then
    echo "DEEPSEEK_API_KEY not set. Please enter your DeepSeek API key:"
    read -r api_key
    export DEEPSEEK_API_KEY="$api_key"
  fi
fi

# Run the server with any additional arguments passed to this script (starting from $2)
echo "Starting DeepseekMCP server..."
shift 2>/dev/null || true  # Shift away the API key if it was provided
./bin/deepseek-mcp "$@"
