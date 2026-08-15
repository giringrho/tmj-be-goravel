#!/bin/sh
set -e

# Create .env from .env.example if .env doesn't exist
if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
fi

# Render sets PORT; map it to APP_PORT for Goravel
if [ -n "$PORT" ]; then
  export APP_PORT="$PORT"
  export APP_HOST="0.0.0.0"
fi

# Build DSN from individual DB_* env vars if DB_DSN is not already set.
# Aiven MySQL requires SSL, so we append tls=true to the DSN.
if [ -z "$DB_DSN" ] && [ -n "$DB_HOST" ] && [ -n "$DB_USERNAME" ]; then
  export DB_DSN="${DB_USERNAME}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT:-3306})/${DB_DATABASE}?charset=utf8mb4&parseTime=true&loc=UTC&tls=true"
  export DB_CONNECTION=mysql
fi

echo "==> Running migrations..."
./main artisan migrate

echo "==> Running seeders..."
./main artisan db:seed || true

echo "==> Starting Goravel server on ${APP_HOST:-0.0.0.0}:${APP_PORT:-3000}..."
exec ./main
