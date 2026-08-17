#!/usr/bin/env bash
# Mint a Kener API token for a LOCAL Docker instance by inserting a row directly
# into the api_keys table (auth only checks hashed_key + status=ACTIVE). This
# avoids the admin-UI-only token creation flow. Prints the plaintext token to
# stdout; all diagnostics go to stderr so the output is safe to capture.
#
# Usage: KENER_CONTAINER=kener-tf scripts/kener-token.sh
set -euo pipefail

CONTAINER="${KENER_CONTAINER:-kener-tf}"
KEY_NAME="${KENER_KEY_NAME:-terraform-acc-test}"

if ! docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
  echo "error: container '$CONTAINER' is not running (try 'make kener-up')" >&2
  exit 1
fi

docker exec "$CONTAINER" node -e '
const crypto = require("crypto");
const Database = require("/app/node_modules/better-sqlite3");
const db = new Database("/app/database/kener.sqlite.db");
const secret = process.env.KENER_SECRET_KEY || "";
const name = process.argv[1];
const token = "kener_" + crypto.randomBytes(32).toString("hex");
const hashed = crypto.createHmac("sha256", secret).update(token).digest("hex");
const masked = token.slice(0, 10) + "..." + token.slice(-4);
// Replace any prior key of the same name so this is idempotent.
db.prepare("DELETE FROM api_keys WHERE name = ?").run(name);
db.prepare("INSERT INTO api_keys (name, hashed_key, masked_key, status) VALUES (?,?,?,?)")
  .run(name, hashed, masked, "ACTIVE");
process.stdout.write(token);
' "$KEY_NAME"
