#!/bin/sh
set -e

echo "⏳ Waiting for PostgreSQL..."

until bun -e "
import { Client } from 'pg';
const c = new Client({ connectionString: process.env.DATABASE_URL });
await c.connect();
await c.end();
"; do
  sleep 2
done

echo "✅ PostgreSQL is ready"

echo "🌱 Running seed..."
bun src/utils/seed.js

echo "🚀 Starting backend..."
bun src/app.js