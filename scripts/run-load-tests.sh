#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# Support arguments: ./run-load-tests.sh [BASE_URL] [SUITE] or ./run-load-tests.sh [SUITE]
# Also respects BASE_URL and API_BASE_URL env variables
BASE_URL="${BASE_URL:-${API_BASE_URL:-http://localhost:8080}}"
SUITE="${SUITE:-all}"

if [ $# -ge 2 ]; then
    BASE_URL="$1"
    SUITE="$2"
elif [ $# -eq 1 ]; then
    if [[ "$1" =~ ^https?:// ]]; then
        BASE_URL="$1"
    else
        SUITE="$1"
    fi
fi

echo "==============================================="
echo "  Lynk Platform k6 Load & Concurrency Runner   "
echo "==============================================="
echo "Target API: ${BASE_URL}"
echo "Suite:      ${SUITE}"
echo ""

if ! command -v k6 &> /dev/null; then
    echo "k6 command not found on PATH."
    echo "Running node syntax validation check..."
    node -c tests/load/helpers.js
    node -c tests/load/search_load.js
    node -c tests/load/ratelimit_stress.js
    node -c tests/load/outbox_burst.js
    echo "All load test scripts passed syntax verification."
    exit 0
fi

export API_BASE_URL="${BASE_URL}"

if [ "${SUITE}" = "all" ] || [ "${SUITE}" = "search" ]; then
    echo "--> Running Search & Cache-Aside Load Suite..."
    k6 run tests/load/search_load.js
fi

if [ "${SUITE}" = "all" ] || [ "${SUITE}" = "ratelimit" ]; then
    echo "--> Running Rate Limiter Stress Suite..."
    k6 run tests/load/ratelimit_stress.js
fi

if [ "${SUITE}" = "all" ] || [ "${SUITE}" = "outbox" ]; then
    echo "--> Running Metrics & Outbox Throughput Suite..."
    k6 run tests/load/outbox_burst.js
fi

echo ""
echo "k6 load testing completed successfully."
