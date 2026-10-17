#!/usr/bin/env bash
set -euo pipefail

HOST_NAME="${1:-localhost}"

echo "==============================================="
echo "   Lynk Multi-Container Synthetic Health Probe "
echo "==============================================="
echo "Testing connectivity on host: ${HOST_NAME}"
echo ""

SERVICES=(
  "PostgreSQL (Raw Pool):5432"
  "PgBouncer (Tx Pool):6432"
  "SuperTokens IAM Core:3567"
  "MinIO Object Storage:9000"
  "Redis Cache & Pub/Sub:6379"
  "Go REST API & SSE:8080"
  "FastAPI AI Subsystem:8000"
)

PASSED=0
TOTAL=${#SERVICES[@]}

for ITEM in "${SERVICES[@]}"; do
  SVC="${ITEM%%:*}"
  PORT="${ITEM##*:}"
  if nc -z -w 1 "${HOST_NAME}" "${PORT}" 2>/dev/null || (exec 3<>/dev/tcp/"${HOST_NAME}"/"${PORT}") 2>/dev/null; then
    echo " [PASS] ${SVC} on port ${PORT}"
    PASSED=$((PASSED + 1))
  else
    echo " [DOWN] ${SVC} on port ${PORT} (offline or unmapped)"
  fi
done

echo ""
echo "Probe result: ${PASSED} / ${TOTAL} services responded."
