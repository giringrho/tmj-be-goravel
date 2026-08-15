#!/bin/sh
set -e

# Create .env from .env.example if it doesn't exist
if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
fi

# If PORT is set (Railway), use it as APP_PORT
if [ -n "$PORT" ]; then
  export APP_PORT="$PORT"
fi

# If DATABASE_URL is set (Railway MySQL plugin), parse it into DB_* vars.
# Format: mysql://user:pass@host:port/dbname
if [ -n "$DATABASE_URL" ]; then
  # Strip scheme
  rest="${DATABASE_URL#mysql://}"
  # Extract user:pass
  creds="${rest%%@*}"
  DB_USERNAME="${creds%%:*}"
  DB_PASSWORD="${creds#*:}"
  # Extract host:port/db
  hostport_db="${rest#*@}"
  DB_HOST="${hostport_db%%:*}"
  hostport="${hostport_db%%/*}"
  DB_PORT="${hostport#*:}"
  if [ "$DB_PORT" = "$hostport" ]; then
    DB_PORT=3306
  fi
  DB_DATABASE="${hostport_db#*/}"
  export DB_USERNAME DB_PASSWORD DB_HOST DB_PORT DB_DATABASE DB_CONNECTION=mysql
fi

echo "==> Running migrations..."
./main artisan migrate

echo "==> Running seeders..."
./main artisan db:seed || true

echo "==> Starting server on port ${APP_PORT:-3000}..."
exec ./main
