<div align="center">

# Lynk Tooling Architecture & Engineering Standards

**Authoritative, permanent engineering tooling policy across Go, Python, and TypeScript.**

<p align="center">
  <img src="https://img.shields.io/badge/Python-3.11+-3776AB?style=flat-square&logo=python&logoColor=white" alt="Python" />
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/TypeScript-5.6+-3178C6?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker" />
  <img src="https://img.shields.io/badge/Next.js-14+-black?style=flat-square&logo=next.js&logoColor=white" alt="Next.js" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/pgvector-Enabled-336791?style=flat-square" alt="pgvector" />
</p>

</div>

---

## 1. Architectural Philosophy

The objective of this specification is to lock in a permanent, scalable tooling architecture designed to support repository growth over years without constant re-architecting.

The core policy is:

> One tool per responsibility, except where a specialized tool provides complementary analysis.

### The Dual-Gate Model

```text
                     Developer Workstation
                               |
                           git commit
                               |
                               v
                     +--------------------+
                     |     PRE-COMMIT     |
                     |                    |
                     | Fast, local, diff  |
                     | Only changed files |
                     +---------+----------+
                               |
                           git push
                               |
                               v
                 +----------------------------+
                 |       GITHUB ACTIONS       |
                 |                            |
                 | Parallel matrix jobs       |
                 | Complete projects & tests  |
                 +-------------+--------------+
                               |
         +---------------------+---------------------+
         |                     |                     |
         v                     v                     v
   +-----------+         +-----------+         +-----------+
   | Frontend  |         |  Python   |         |    Go     |
   +-----------+         +-----------+         +-----------+
   | Prettier  |         | Ruff      |         | gofmt     |
   | Oxlint    |         | Mypy      |         | goimports |
   | ESLint    |         | Pytest    |         | golangci  |
   | TSC       |         | Security  |         | govuln    |
   | Tests     |         |           |         | go test   |
   +-----------+         +-----------+         +-----------+
         |                     |                     |
         +---------------------+---------------------+
                               |
                               v
                      +-----------------+
                      | Quality Gate OK |
                      +-----------------+
```

- Pre-commit is for developer experience: Sub-second to few-second checks on changed files only to catch obvious formatting, syntax, and hygiene issues before code is committed.
- GitHub Actions is the source of truth: Complete, exhaustive checks across all projects, full test suites, type checking, security scanning, and multi-container smoke tests. CI never assumes or requires developers to have run pre-commit locally.

---

## 2. Permanent Tooling Matrix

| Concern | Go Subsystem (`backend/`) | Python AI Subsystem (`ai/`) | TypeScript Frontend (`frontend/`) |
| :--- | :--- | :--- | :--- |
| **Formatting** | `gofmt` + `goimports` | **Ruff format** | **Prettier** |
| **Linting** | **golangci-lint** | **Ruff check** | **Oxlint** (fast) + **ESLint** (domain) |
| **Type Checking** | Go compiler (`go build` / `go vet`) | **mypy** (strict mode) | **tsc --noEmit** |
| **Testing** | `go test -race` | `pytest` | Project test runner (`npm test`) |
| **Security** | `govulncheck` + `gosec` via golangci | Dependency audit (`pip-audit`) | Dependency audit (`npm audit`) |
| **Local Hooks** | **pre-commit** | **pre-commit** | **pre-commit** |
| **Remote CI** | GitHub Actions | GitHub Actions | GitHub Actions |

### Explicit Inclusions and Exclusions

#### Do Not Add
- Black (Ruff format is an explicit, drop-in replacement designed to be Black-compatible, running 30x faster in Rust).
- Flake8, Pylint, isort, autoflake (Ruff check replaces all four with unified configuration and sub-second execution).
- A second Python formatter (only Ruff format is permitted).
- TypeScript type-aware ESLint as a replacement for `tsc` (Oxlint provides instant AST validation; ESLint provides React/Next.js rules; `tsc --noEmit` provides true compiler-backed type safety).
- Giant universal linter configurations (each language ecosystem maintains its own scoped configuration).
- Full-repository expensive checks on every local git commit.

---

## 3. Directory Ownership & Configuration Boundaries

Global policies reside at the repository root. Language-specific policies reside strictly inside each project boundary:

```text
Lynk/
+-- .github/workflows/          # Global CI/CD workflows (ci.yml, smoke-test.yml, docker-build.yml)
+-- .pre-commit-config.yaml      # Global pre-commit orchestration for changed files
+-- .editorconfig               # Universal whitespace and indentation standards
+-- .gitignore                  # Global repository exclusions
|
+-- frontend/                   # TypeScript / Next.js boundary
|   +-- package.json            # Node dependencies and scripts
|   +-- tsconfig.json           # TypeScript compiler configuration
|   +-- .prettierrc             # Prettier formatting rules
|   +-- eslint.config.mjs       # Next.js and React ESLint rules
|   +-- .oxlintrc.json          # Oxlint fast-lint configuration
|
+-- backend/                    # Go authoritative API boundary
|   +-- go.mod, go.sum          # Go modules and pinned dependency hashes
|   +-- .golangci.yml           # golangci-lint v2 configuration
|   +-- Makefile                # Local Go developer tasks
|
+-- ai/                         # Python AI subsystem boundary
|   +-- pyproject.toml          # Ruff (format + lint), mypy, pytest configuration
|   +-- requirements.txt        # Pinned Python package dependencies
|
+-- docs/                       # Architecture and operational specifications
+-- scripts/                    # End-to-end integration and smoke test scripts
```

This boundary ensures that if new services are added in the future (e.g. `services/search/`, `ml/`, `worker/`), the root architecture does not need to be rewritten; each service simply carries its own scoped toolchain configuration.

---

## 4. Language-Specific Configurations

### 4.1 Python Ecosystem (`ai/`)

All Python tooling configuration is centralized in `ai/pyproject.toml`.

#### Ruff Configuration
```toml
[tool.ruff]
line-length = 88
target-version = "py311"
src = ["ai"]

[tool.ruff.lint]
select = [
    "E",      # pycodestyle errors
    "W",      # pycodestyle warnings
    "F",      # Pyflakes (undefined names, unused imports)
    "I",      # isort (import sorting)
    "B",      # flake8-bugbear (common bug patterns)
    "C4",     # flake8-comprehensions
    "UP",     # pyupgrade (modern Python syntax)
    "S",      # flake8-bandit (security checks)
    "ASYNC",  # flake8-async (asyncio safety)
]
ignore = [
    "S101",   # allow assert in pytest test files
]

[tool.ruff.format]
quote-style = "double"
indent-style = "space"
line-ending = "lf"
```

#### Mypy Configuration
```toml
[tool.mypy]
python_version = "3.11"
warn_return_any = true
warn_unused_configs = true
disallow_untyped_defs = true
disallow_incomplete_defs = true
check_untyped_defs = true
no_implicit_optional = true
warn_redundant_casts = true
warn_unused_ignores = true

[[tool.mypy.overrides]]
module = [
    "sentence_transformers.*",
    "torch.*",
    "sklearn.*",
    "asyncpg.*",
]
ignore_missing_imports = true
```

---

### 4.2 TypeScript / TSX Ecosystem (`frontend/`)

The TypeScript frontend uses a layered, multi-tool approach where each tool handles what it does best:

```text
Prettier   -> "How should the code look?" (Formatting, line wraps, spacing)
Oxlint     -> "Is there something obviously broken?" (Sub-100ms syntax, hooks, a11y)
ESLint     -> "Does this violate Next.js / React application conventions?" (Domain rules)
tsc        -> "Is this valid TypeScript?" (True compiler type evaluation)
```

#### Prettier (`frontend/.prettierrc`)
```json
{
  "semi": true,
  "trailingComma": "es5",
  "singleQuote": false,
  "printWidth": 100,
  "tabWidth": 2,
  "useTabs": false
}
```

#### Oxlint (`frontend/.oxlintrc.json`)
```json
{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "plugins": ["react", "unicorn", "typescript"],
  "rules": {
    "no-debugger": "error",
    "no-unused-vars": "warn",
    "react-hooks/rules-of-hooks": "error",
    "react-hooks/exhaustive-deps": "warn"
  }
}
```

---

### 4.3 Go Ecosystem (`backend/`)

Go relies on the standard toolchain for formatting and compiler validation, backed by `golangci-lint` for static analysis.

#### golangci-lint Baseline (`backend/.golangci.yml`)
```yaml
run:
  timeout: 5m
  issues-exit-code: 1
  tests: true

linters:
  disable-all: true
  enable:
    - errcheck      # Unchecked errors
    - govet         # Official Go vet checks
    - ineffassign   # Ineffective assignments
    - staticcheck   # Static analysis standard
    - unused        # Unused constants, variables, functions
    - gosec         # Security scanner (SQLi, hardcoded credentials)
    - revive        # Drop-in, fast replacement for golint
    - gofmt         # Formatting validation
    - goimports     # Import organization validation

linters-settings:
  govet:
    enable-all: true
    disable:
      - fieldalignment  # Micro-optimization not required for MVP
  gosec:
    excludes:
      - G104  # Handled comprehensively by errcheck
  revive:
    rules:
      - name: exported
        disabled: true

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

---

## 5. Incremental Enforcement & Migration Strategy

To prevent blocking ongoing feature delivery when adding new rules to large codebases, tooling adopts **new-code enforcement**:

```text
Existing Codebase                New / Modified Pull Requests
+--------------------------+     +--------------------------+
| Baseline issues recorded |     | Must pass 100% of all    |
| Not blocking active dev  |     | linters, formatters, and |
| Burned down incrementally|     | type-check rules cleanly |
+--------------------------+     +--------------------------+
```

### Mechanisms for Incremental Adoption
1. **golangci-lint `new-from-rev`**:
   In CI, run `golangci-lint run --new-from-rev=origin/main` for PR branches to evaluate only newly introduced code lines, while running full evaluation on main.
2. **Ruff Per-File Scoping**:
   Use `extend-exclude` or granular rule ignores during initial rollout, removing ignores one module at a time.
3. **Mypy Progressive Strictness**:
   Start with `check_untyped_defs = true` and `warn_return_any = true`, then enable `disallow_untyped_defs = true` module by module.

---

## 6. Local Pre-Commit Hook Architecture

The root `.pre-commit-config.yaml` is intentionally small and fast. It only runs lightweight checks on staged files.

```yaml
repos:
  # Universal Hygiene (Instant)
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.6.0
    hooks:
      - id: check-added-large-files
        args: ["--maxkb=500"]
      - id: check-json
      - id: check-yaml
        args: ["--unsafe"]
      - id: end-of-file-fixer
      - id: trailing-whitespace

  # Python: Ruff Fast Lint & Format
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.6.4
    hooks:
      - id: ruff
        args: [--fix]
        files: ^ai/
      - id: ruff-format
        files: ^ai/

  # TypeScript: Prettier & Oxlint
  - repo: https://github.com/oxc-project/oxlint
    rev: v0.9.4
    hooks:
      - id: oxlint
        args: [--deny-warnings]
        files: ^frontend/

  - repo: https://github.com/pre-commit/mirrors-prettier
    rev: v4.0.0-alpha.8
    hooks:
      - id: prettier
        files: ^frontend/.*\.(ts|tsx|css|json|md)$

  # Go: Formatting
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
        files: ^backend/
      - id: go-imports
        files: ^backend/
```

### Pre-Commit Boundaries
- What belongs in pre-commit: Formatting, syntax checks, trailing whitespace, EOF fixers, small lint fixes (`ruff --fix`). Execution time must remain under 3 seconds.
- What NEVER belongs in pre-commit: Full `pytest` runs, Go integration tests, whole-repo `mypy`, Docker image builds, or expensive security scans. These belong exclusively in CI.

---

## 7. GitHub Actions CI Architecture

CI workflows run in parallel jobs inside `.github/workflows/ci.yml`. Each language executes independently so failures are isolated and easily diagnosed:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  frontend:
    name: Frontend Quality Gate
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: cd frontend && npm ci
      - name: Prettier Formatting Check
        run: cd frontend && npx prettier --check "src/**/*.{ts,tsx,css}"
      - name: Oxlint Fast Static Analysis
        run: cd frontend && npx oxlint --deny-warnings src/
      - name: TypeScript Typecheck
        run: cd frontend && npx tsc --noEmit
      - name: ESLint Next.js Rules
        run: cd frontend && npm run lint
      - name: Unit Tests
        run: cd frontend && npm test
      - name: Production Build
        run: cd frontend && npm run build

  backend:
    name: Go Backend Quality Gate
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache-dependency-path: backend/go.sum
      - name: golangci-lint Static Analysis
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.60
          working-directory: backend
      - name: Vulnerability Check
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          cd backend && govulncheck ./...
      - name: Test Suite with Race Detector
        run: cd backend && go test -v -race ./...

  python:
    name: Python AI Subsystem Quality Gate
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: "3.11"
          cache: pip
          cache-dependency-path: ai/requirements.txt
      - run: pip install -r ai/requirements.txt ruff mypy
      - name: Ruff Formatting Check
        run: ruff format --check ai/
      - name: Ruff Lint Check
        run: ruff check ai/
      - name: Mypy Strict Type Check
        run: mypy ai/
      - name: Pytest Suite
        run: python -m pytest ai/tests/ -v
      - name: Offline IR Evaluation Harness
        run: python -m ai.evaluation.harness
```

---

## 8. Version Pinning Strategy

To eliminate unexpected CI breaks from upstream changes, all tools must follow strict version pinning:

1. Package Managers:
   - `frontend/package.json`: Use exact versions for developer dependencies (`prettier`, `oxlint`, `eslint`).
   - `ai/requirements.txt`: Pin exact minor versions (`ruff==0.6.4`, `mypy==1.11.2`).
   - `backend/go.mod`: Go modules are cryptographically pinned via `go.sum`.
2. Pre-Commit Hooks:
   - Always reference explicit release tags (`rev: v0.6.4`) in `.pre-commit-config.yaml`. Never reference branch names (`main`, `master`).
3. GitHub Actions:
   - Action steps must reference specific major versions (`actions/checkout@v4`, `actions/setup-go@v5`).
4. Upgrades:
   - Toolchain upgrades are performed deliberately via scheduled maintenance PRs, never implicitly or automatically on unrelated feature branches.

---

## 9. Local Developer Command Reference

| Action | Go (`backend/`) | Python (`ai/`) | TypeScript (`frontend/`) |
| :--- | :--- | :--- | :--- |
| **Format Code** | `go fmt ./... && goimports -w .` | `ruff format ai/` | `npm run format` / `npx prettier --write .` |
| **Lint Code** | `golangci-lint run` | `ruff check ai/` | `npx oxlint src/ && npm run lint` |
| **Fix Lint Issues** | `golangci-lint run --fix` | `ruff check --fix ai/` | `npm run lint -- --fix` |
| **Type Check** | `go vet ./...` | `mypy ai/` | `npx tsc --noEmit` |
| **Run Unit Tests**| `go test -race ./...` | `python -m pytest ai/tests/ -v` | `npm test` |
| **Security Audit**| `govulncheck ./...` | `pip-audit` | `npm audit` |
| **Pre-Commit Run**| `pre-commit run --all-files` | `pre-commit run --all-files` | `pre-commit run --all-files` |
