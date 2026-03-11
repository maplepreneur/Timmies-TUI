#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="timmies"
INSTALL_DIR="${GOBIN:-$(go env GOPATH)/bin}"

echo "==> Building $BINARY_NAME..."
go build -o "$BINARY_NAME" ./cmd/timmies
echo "    Built ./$BINARY_NAME"

echo "==> Installing to $INSTALL_DIR..."
go install ./cmd/timmies
echo "    Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"

echo "==> Verifying install..."
if command -v "$BINARY_NAME" &>/dev/null; then
    echo "    $BINARY_NAME $(${BINARY_NAME} --version 2>&1 | head -1)"
else
    echo "    Warning: $BINARY_NAME not found in PATH."
    echo "    Make sure $INSTALL_DIR is in your PATH."
fi

echo ""
echo "Done! Run '$BINARY_NAME --help' or '$BINARY_NAME tui' to get started."
