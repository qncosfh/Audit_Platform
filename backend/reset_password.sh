#!/bin/bash

set -e

CONTAINER_NAME="platform-postgres"
DB_USER="sYsadMin"
DB_NAME="platform"

echo "👉 Connecting to PostgreSQL container: $CONTAINER_NAME"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "❌ Container $CONTAINER_NAME is not running!"
  exit 1
fi

echo "👉 Creating or updating user sYsAdMin..."

docker exec -i $CONTAINER_NAME psql -U $DB_USER -d $DB_NAME <<'EOF'

CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO users (username, email, password, role)
VALUES (
    'sYsAdMin',
    'admin@example.com',
    crypt('pAsSwOrd@123!', gen_salt('bf', 10)),
    'admin'
)
ON CONFLICT (username) DO UPDATE
SET password = EXCLUDED.password;

-- 验证
SELECT username,
       crypt('pAsSwOrd@123!', password) = password AS is_valid
FROM users
WHERE username = 'sYsAdMin';

EOF

echo "✅ User ensured and password updated!"