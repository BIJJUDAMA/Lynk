.PHONY: help setup setup-hooks setup-go setup-frontend setup-ai test lint format format-check

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
