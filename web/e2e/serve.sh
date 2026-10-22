#!/bin/sh
# Starts a fresh server for end-to-end tests with an empty data directory.
# MAHLZEIT_BIN may point to a prebuilt binary; otherwise `go run` is used.
set -eu
port="$1"
data="$(mktemp -d)"
export MAHLZEIT_DATA_DIR="$data" MAHLZEIT_LISTEN="127.0.0.1:$port" MAHLZEIT_BASE_URL="http://127.0.0.1:$port" MAHLZEIT_SETUP_TOKEN="e2e-setup-token"
if [ -n "${MAHLZEIT_BIN:-}" ]; then
  exec "$MAHLZEIT_BIN"
fi
cd "$(dirname "$0")/../.."
exec go run ./cmd/mahlzeit
