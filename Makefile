.PHONY: help setup setup-hooks setup-go setup-frontend setup-ai test lint format format-check load-test load-test-rate-limit load-test-cache load-test-pool load-test-concurrency load-test-stress load-test-ultra load-test-saturation stack-ram stack-ram-down

help:
	@echo "Available commands:"
	@echo "  make setup          - Install git hooks and dependencies for all services"
	@echo "  make setup-hooks    - Install git pre-commit hooks"
	@echo "  make setup-go       - Download Go backend modules"
	@echo "  make setup-frontend - Install Next.js frontend npm dependencies"
	@echo "  make setup-ai       - Create Python venv and install AI dependencies"
	@echo "  make test           - Run backend, frontend, and AI unit test suites"
	@echo "  make lint           - Run all static analyzers and linters"
	@echo "  make format         - Auto-format Go, Python, and TypeScript code"
	@echo "  make format-check   - Verify code formatting across all services"
	@echo "  make load-test      - Run complete sequential k6 backend load test suite"
	@echo "  make load-test-ultra - Run Stage 6 ultra-concurrency stress test (2,500 VUs)"
	@echo "  make load-test-saturation - Run Stage 7 constant-arrival saturation ceiling (no think time)"
	@echo "  make stack-ram RAM=N - Start the server stack capped to N MiB total (e.g. RAM=4096, RAM=6144)"
	@echo "  make stack-ram-down  - Stop the RAM-capped server stack"

setup-hooks:
	@echo "==> Installing pre-commit hooks..."
	pre-commit install

setup-go:
	@echo "==> Downloading Go modules..."
	cd backend && go mod download && go mod verify

setup-frontend:
	@echo "==> Installing frontend npm dependencies..."
	cd frontend && npm install

setup-ai:
	@echo "==> Setting up Python virtual environment and AI dependencies..."
	cd ai && (python3 -m venv .venv 2>/dev/null || python -m venv .venv)
	@if [ -f ai/.venv/bin/pip ]; then \
		ai/.venv/bin/pip install --upgrade pip && ai/.venv/bin/pip install -r ai/requirements.txt; \
	elif [ -f ai/.venv/Scripts/pip.exe ]; then \
		ai/.venv/Scripts/pip.exe install --upgrade pip && ai/.venv/Scripts/pip.exe install -r ai/requirements.txt; \
	else \
		pip install -r ai/requirements.txt; \
	fi

setup: setup-hooks setup-go setup-frontend setup-ai
	@echo "==> All Lynk services and pre-commit hooks successfully setup!"

test:
	@echo "==> Running backend tests..."
	cd backend && go test ./...
	@echo "==> Running frontend tests..."
	cd frontend && npm test
	@echo "==> Running AI tests..."
	@if [ -f ai/.venv/bin/pytest ]; then \
		ai/.venv/bin/pytest ai/; \
	elif [ -f ai/.venv/Scripts/pytest.exe ]; then \
		ai/.venv/Scripts/pytest.exe ai/; \
	else \
		pytest ai/; \
	fi

lint:
	@echo "==> Running backend lint..."
	cd backend && golangci-lint run
	@echo "==> Running frontend lint (oxlint)..."
	cd frontend && npm run lint:ox
	@echo "==> Running AI lint (ruff)..."
	@if [ -f ai/.venv/bin/ruff ]; then \
		ai/.venv/bin/ruff check ai/; \
	elif [ -f ai/.venv/Scripts/ruff.exe ]; then \
		ai/.venv/Scripts/ruff.exe ai/; \
	else \
		ruff check ai/; \
	fi

format:
	@echo "==> Formatting Go files..."
	cd backend && gofmt -w .
	@echo "==> Formatting frontend files..."
	cd frontend && npm run format
	@echo "==> Formatting AI files..."
	@if [ -f ai/.venv/bin/ruff ]; then \
		ai/.venv/bin/ruff format ai/; \
	elif [ -f ai/.venv/Scripts/ruff.exe ]; then \
		ai/.venv/Scripts/ruff.exe ai/; \
	else \
		ruff format ai/; \
	fi

format-check:
	@echo "==> Checking Go formatting..."
	@cd backend && [ -z "$$(gofmt -l .)" ] || (echo "Unformatted Go files detected" && exit 1)
	@echo "==> Checking frontend formatting..."
	cd frontend && npm run format:check
	@echo "==> Checking AI formatting..."
	@if [ -f ai/.venv/bin/ruff ]; then \
		ai/.venv/bin/ruff format --check ai/; \
	elif [ -f ai/.venv/Scripts/ruff.exe ]; then \
		ai/.venv/Scripts/ruff.exe ai/; \
	else \
		ruff format --check ai/; \
	fi

load-test:
	@echo "==> Running complete backend load testing suite..."
	node tests/load/run_all.js

load-test-rate-limit:
	@echo "==> Running Stage 1: Rate limiter enforcement test..."
	k6 run tests/load/scripts/01_rate_limit.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run tests/load/scripts/01_rate_limit.js

load-test-cache:
	@echo "==> Running Stage 2: Cache & read benchmark..."
	k6 run tests/load/scripts/02_cache_benchmark.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run tests/load/scripts/02_cache_benchmark.js

load-test-pool:
	@echo "==> Running Stage 3: Database pool saturation test..."
	k6 run tests/load/scripts/03_pool_saturation.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run tests/load/scripts/03_pool_saturation.js

load-test-concurrency:
	@echo "==> Running Stage 4: Max user concurrency breakpoint test..."
	k6 run tests/load/scripts/04_user_concurrency.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run tests/load/scripts/04_user_concurrency.js

load-test-stress:
	@echo "==> Running Stage 5: High-concurrency stress test (1,000 VUs)..."
	k6 run tests/load/scripts/05_high_concurrency_stress.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run tests/load/scripts/05_high_concurrency_stress.js

load-test-ultra:
	@echo "==> Running Stage 6: Ultra-concurrency stress test (2,500 VUs)..."
	node tests/load/run_all.js --stage=06_ultra_concurrency_stress

load-test-saturation:
	@echo "==> Running Stage 7: Constant-arrival saturation ceiling (no think time)..."
	k6 run "--summary-trend-stats=avg,min,med,max,p(90),p(95),p(99)" --summary-export=tests/load/results/07_saturation_ceiling.json tests/load/scripts/07_saturation_ceiling.js || docker run --rm -i --network=host -v "$$(pwd):/lynk" -w /lynk -e API_BASE_URL=http://localhost:8080/api/v1 grafana/k6 run "--summary-trend-stats=avg,min,med,max,p(90),p(95),p(99)" --summary-export=tests/load/results/07_saturation_ceiling.json tests/load/scripts/07_saturation_ceiling.js

RAM ?= 4096

stack-ram:
	@echo "==> Generating RAM cap override for $(RAM) MiB total..."
	node tests/load/ramcap.js $(RAM) > docker-compose.ramcap.yml
	@echo "==> Starting server stack capped to $(RAM) MiB total..."
	docker compose -f docker-compose.yml -f docker-compose.ramcap.yml up -d --force-recreate

stack-ram-down:
	@echo "==> Stopping the RAM-capped server stack..."
	docker compose -f docker-compose.yml -f docker-compose.ramcap.yml down
