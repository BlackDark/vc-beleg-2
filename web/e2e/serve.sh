#!/bin/sh
# Starts a fresh server for end-to-end tests with an empty data directory.
# MAHLZEIT_BIN may point to a prebuilt binary; otherwise `go run` is used.
set -eu
port="$1"
data="$(mktemp -d)"
# Inherited settings would override the test setup (a _FILE secret wins over the plain variable).
unset MAHLZEIT_SETUP_TOKEN_FILE MAHLZEIT_OIDC_ISSUER MAHLZEIT_IMAP_HOST MAHLZEIT_WEBDAV_URL MAHLZEIT_EXTRACT_PROVIDER ANTHROPIC_API_KEY ANTHROPIC_API_KEY_FILE
export MAHLZEIT_DATA_DIR="$data" MAHLZEIT_LISTEN="127.0.0.1:$port" MAHLZEIT_BASE_URL="http://127.0.0.1:$port" MAHLZEIT_SETUP_TOKEN="e2e-setup-token"
if [ -n "${MAHLZEIT_BIN:-}" ]; then
  exec "$MAHLZEIT_BIN"
fi
cd "$(dirname "$0")/../.."
exec go run ./cmd/mahlzeit
