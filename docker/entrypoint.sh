#!/bin/sh
# Container entrypoint — run pending migrations, then exec the server.
# Migrations are idempotent (golang-migrate tracks schema_migrations), so
# a roll-restart on the same image version is a no-op.
set -e

echo "[medigt] running database migrations..."
./migrate up

echo "[medigt] starting server..."
exec ./server
