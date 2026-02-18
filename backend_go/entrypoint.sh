#!/bin/sh
set -eu

echo "[entrypoint] Starting backend_go..."
echo "[entrypoint] PORT=${PORT:-3000}"
echo "[entrypoint] RUN_SEED=${RUN_SEED:-0} WAIT_FOR_DEPS=${WAIT_FOR_DEPS:-1}"

if [ "${WAIT_FOR_DEPS:-1}" = "1" ]; then
  DB_HOSTPORT="$(echo "${DATABASE_URL:-}" | sed -n 's|.*@||; s|/.*||p')"
  DB_HOST="$(echo "$DB_HOSTPORT" | cut -d: -f1)"
  DB_PORT="$(echo "$DB_HOSTPORT" | cut -d: -f2)"
  DB_PORT="${DB_PORT:-5432}"

  echo "[entrypoint] Waiting for Postgres at ${DB_HOST}:${DB_PORT} ..."
  i=0
  timeout="${DB_WAIT_TIMEOUT:-30}"
  while ! nc -z "$DB_HOST" "$DB_PORT" >/dev/null 2>&1; do
    i=$((i+1))
    if [ "$i" -ge "$timeout" ]; then
      echo "[entrypoint] ERROR: Postgres not reachable after ${timeout}s"
      exit 1
    fi
    sleep 1
  done
  echo "[entrypoint] Postgres is reachable."

  # ---- Wait for Redis ----
  REDIS_HOST="${REDIS_HOST:-redis}"
  REDIS_PORT="${REDIS_PORT:-6379}"

  echo "[entrypoint] Waiting for Redis at ${REDIS_HOST}:${REDIS_PORT} ..."
  i=0
  rtimeout="${REDIS_WAIT_TIMEOUT:-10}"
  while ! nc -z "$REDIS_HOST" "$REDIS_PORT" >/dev/null 2>&1; do
    i=$((i+1))
    if [ "$i" -ge "$rtimeout" ]; then
      echo "[entrypoint] ERROR: Redis not reachable after ${rtimeout}s"
      exit 1
    fi
    sleep 1
  done
  echo "[entrypoint] Redis is reachable."
fi

# ---- Seed (optional) ----
if [ "${RUN_SEED:-0}" = "1" ]; then
  echo "[entrypoint] Running seed..."
  /app/seed || {
    echo "[entrypoint] ERROR: seed failed"
    exit 1
  }
  echo "[entrypoint] Seed done."
else
  echo "[entrypoint] Seed skipped (RUN_SEED!=1)."
fi

# ---- Start server ----
echo "[entrypoint] Starting server..."
exec /app/server