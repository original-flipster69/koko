#!/bin/bash

# Build script for koko - builds and installs to /usr/local/bin
set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Build the binary
echo "Building koko..."
go build -o koko ./cmd/koko

# Copy to /usr/local/bin
echo "Installing to /usr/local/bin..."
sudo cp koko /usr/local/bin/
sudo chmod +x /usr/local/bin/koko

echo "✓ koko installed successfully to /usr/local/bin/koko"
echo "✓ You can now run: koko"
