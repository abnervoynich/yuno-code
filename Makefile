SHELL := /bin/bash
.PHONY: dev down build seed seed-generate seed-load reconcile test test-unit test-integration test-coverage openapi openapi-validate logs backend-run backend-build frontend-run clean help

# Load port overrides from .env if it exists (silently, so make itself doesn't fail)
-include .env
BACKEND_PORT  ?= 8090
FRONTEND_PORT ?= 3000
PROMETHEUS_PORT ?= 9090
GRAFANA_PORT  ?= 3001

API_URL := http://localhost:$(BACKEND_PORT)

# ─── Docker ──────────────────────────────────────────────────────────────────

## dev: Start all services (backend, frontend, prometheus, grafana) via docker-compose
dev:
	docker-compose up --build -d
	@echo ""
	@echo "Services:"
	@echo "  Backend API:  $(API_URL)"
	@echo "  Frontend UI:  http://localhost:$(FRONTEND_PORT)"
	@echo "  Prometheus:   http://localhost:$(PROMETHEUS_PORT)"
	@echo "  Grafana:      http://localhost:$(GRAFANA_PORT)  (admin / admin)"
	@echo ""
	@echo "To change ports: edit .env and re-run 'make dev'"

## down: Stop all services
down:
	docker-compose down

## logs: Tail all service logs
logs:
	docker-compose logs -f

## build: Build all Docker images
build:
	docker-compose build

# ─── Seed & Test Data ─────────────────────────────────────────────────────────

## seed: Generate test data files AND load them into a running backend
seed: seed-generate seed-load

## seed-generate: Only generate the test data files under backend/testdata/
seed-generate:
	@cd backend && go run ./cmd/seed/... testdata/
	@echo "Test data written to backend/testdata/"

## seed-load: Load test data into the running backend via API (uses BACKEND_PORT from .env)
seed-load:
	@echo "Loading 100 expected transactions into $(API_URL)..."
	curl -s -X POST $(API_URL)/api/v1/transactions/bulk \
	  -H "Content-Type: application/json" \
	  -d @backend/testdata/expected_transactions.json
	@echo ""
	@echo "Uploading PSP A settlement (CSV)..."
	curl -s -X POST $(API_URL)/api/v1/settlements/upload \
	  -F "psp_name=pspa" -F "file=@backend/testdata/pspa_settlement.csv"
	@echo ""
	@echo "Uploading PSP B settlement (JSON)..."
	curl -s -X POST $(API_URL)/api/v1/settlements/upload \
	  -F "psp_name=pspb" -F "file=@backend/testdata/pspb_settlement.json"
	@echo ""
	@echo "Uploading PSP C settlement (custom)..."
	curl -s -X POST $(API_URL)/api/v1/settlements/upload \
	  -F "psp_name=pspc" -F "file=@backend/testdata/pspc_settlement.txt"
	@echo ""
	@echo "Done! Now run: make reconcile"

## reconcile: Run a reconciliation over Dec 1-7, 2024
reconcile:
	curl -s -X POST $(API_URL)/api/v1/reconciliation/run \
	  -H "Content-Type: application/json" \
	  -d '{"name":"Dec 2024 Full Week","period_start":"2024-12-01","period_end":"2024-12-07"}'

# ─── Testing ─────────────────────────────────────────────────────────────────

## test: Run all Go tests (unit + integration)
test:
	@cd backend && go test ./... -v -count=1

## test-unit: Run only unit tests
test-unit:
	@cd backend && go test ./tests/unit/... -v

## test-integration: Run only integration tests (requires TEST_DATABASE_URL)
test-integration:
	@cd backend && go test ./tests/integration/... -v

## test-coverage: Run tests with coverage report
test-coverage:
	@cd backend && go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out

# ─── Local Development (without Docker) ──────────────────────────────────────

## backend-run: Run the Go backend locally on port 8080 (requires Go 1.22+ and a running PostgreSQL)
backend-run:
	@cd backend && DATABASE_URL=postgres://yuno:yuno@localhost:5432/reconciliation?sslmode=disable PORT=8080 go run ./cmd/server

## frontend-run: Run the Vite dev server (proxies API to localhost:8080 by default)
frontend-run:
	@cd frontend && npm run dev

## backend-build: Build the Go binary
backend-build:
	@cd backend && go build -o bin/server ./cmd/server

# ─── OpenAPI / Documentation ─────────────────────────────────────────────────

## openapi: View OpenAPI docs in Swagger UI (requires Docker)
openapi:
	@echo "Starting Swagger UI at http://localhost:8081 ..."
	docker run --rm -d -p 8081:8080 \
	  -e SWAGGER_JSON=/api/openapi.yaml \
	  -v $(CURDIR)/api:/api \
	  swaggerapi/swagger-ui
	@echo "Visit: http://localhost:8081"

## openapi-validate: Validate the OpenAPI spec (requires npx)
openapi-validate:
	npx @redocly/cli lint api/openapi.yaml

# ─── Clean ───────────────────────────────────────────────────────────────────

## clean: Remove build artifacts and Docker volumes
clean:
	@cd backend && rm -rf bin/ data/ coverage.out
	docker-compose down -v

# ─── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "LuxeCart Settlement Reconciliation Engine"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
	@echo ""
	@echo "Active ports (edit .env to change):"
	@echo "  BACKEND_PORT=$(BACKEND_PORT)  FRONTEND_PORT=$(FRONTEND_PORT)  PROMETHEUS_PORT=$(PROMETHEUS_PORT)  GRAFANA_PORT=$(GRAFANA_PORT)"
	@echo ""
