#!/bin/bash

# Script to compile JSON prompt file to Go string constant
# Usage: ./compile_json_prompt.sh [json-file] [output-file] [package-name]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

JSON_FILE="${1:-$PROJECT_ROOT/promts/f.json}"
OUTPUT_FILE="${2:-}"
PACKAGE_NAME="${3:-}"

# Build the Go script if needed
cd "$PROJECT_ROOT"
go build -o "$SCRIPT_DIR/compile_json_prompt" "$SCRIPT_DIR/compile_json_prompt.go"

# Run the compiler
if [ -z "$OUTPUT_FILE" ]; then
    "$SCRIPT_DIR/compile_json_prompt" "$JSON_FILE"
elif [ -z "$PACKAGE_NAME" ]; then
    "$SCRIPT_DIR/compile_json_prompt" "$JSON_FILE" "$OUTPUT_FILE"
else
    "$SCRIPT_DIR/compile_json_prompt" "$JSON_FILE" "$OUTPUT_FILE" "$PACKAGE_NAME"
fi

