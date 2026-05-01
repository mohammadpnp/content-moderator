#!/bin/sh
# deploy/docker/entrypoint.sh

set -e

echo "Starting Content Moderator Service..."
echo "Environment: ${ENVIRONMENT:-development}"
echo "Go Version: $(go version 2>/dev/null || echo 'not available')"

# Wait for dependencies if needed
# In production, you might want to wait for PostgreSQL, Redis, NATS

echo "Running database migrations..."
# Will be implemented in Phase 1
# For now, just a placeholder

echo "Starting server..."
exec /app/server