# LuxeCart Settlement Reconciliation Engine — Yuno Challenge

## Project Overview
Multi-PSP settlement reconciliation engine for LuxeCart, a luxury e-commerce platform processing $12M+/month across MENA. The system ingests settlement reports from 4 PSPs, normalizes data, detects discrepancies, and produces a unified reconciliation view.

## Architecture

```
┌─────────────────┐    ┌──────────────────────────────────────────┐
│  React Frontend │───▶│          Go Backend (REST API)           │
│  Vite+Tailwind  │    │                                          │
└─────────────────┘    │  ┌──────────────┐  ┌─────────────────┐  │
                        │  │  Ingestion   │  │    Matching     │  │
┌─────────────────┐    │  │  (3 parsers) │  │    Engine       │  │
│   Prometheus    │◀───│  └──────────────┘  └─────────────────┘  │
└────────┬────────┘    │  ┌──────────────────────────────────┐    │
         │             │  │  Reconciliation Service           │    │
┌────────▼────────┐    │  └──────────────────────────────────┘    │
│     Grafana     │    │  ┌──────────────────────────────────┐    │
└─────────────────┘    │  │  SQLite Repository                │    │
                        │  └──────────────────────────────────┘    │
                        └──────────────────────────────────────────┘
```

## Services (docker-compose)
- **backend** — Go 1.22, port 8080, REST API + `/metrics`
- **frontend** — React/Vite/Tailwind served by nginx, port 3000
- **prometheus** — Metrics scraping, port 9090
- **grafana** — Dashboards, port 3001 (admin/admin)

## Backend Structure (`backend/`)
```
cmd/server/main.go          — entry point, wires everything
internal/
  config/                   — env-based config
  domain/                   — pure domain types (Transaction, SettlementRecord, ReconciliationRun, MatchResult)
  api/
    handlers/               — HTTP handlers (transactions, settlements, reconciliation)
    middleware/             — logging, metrics, CORS
  ingestion/
    pspa_csv_parser.go      — PSP A: CSV, daily, original IDs
    pspb_json_parser.go     — PSP B: JSON, weekly, "PSPB_" prefix
    pspc_custom_parser.go   — PSP C: pipe-delimited, every 3 days, different refs
  matching/
    engine.go               — matching orchestration
    fuzzy.go                — fuzzy/confidence scoring
  reconciliation/
    service.go              — reconciliation business logic
  repository/
    db.go                   — SQLite setup + migrations
    transactions.go
    settlements.go
    reconciliation.go
  metrics/
    prometheus.go
testdata/                   — JSON seed + PSP report files
tests/
  unit/                     — unit tests for parsers, matching engine
  integration/              — integration tests hitting real HTTP + SQLite
```

## PSP Formats
| PSP | Format | Cadence | ID Mapping |
|-----|--------|---------|------------|
| PSP A | CSV | Daily | Original transaction ID |
| PSP B | JSON | Weekly | Prepends "PSPB_" to original ID |
| PSP C | Pipe-delimited | Every 3 days | Own ref; matched by amount+timestamp |

## Matching Strategy
1. **Exact ID** — direct lookup by transaction ID (confidence 1.0)
2. **Normalized ID** — strip known PSP prefixes like "PSPB_" (confidence 0.95)
3. **Fuzzy** — amount + timestamp ≤5min (0.85), ≤30min (0.75), ≤2h (0.60)

Currency normalization to USD uses static rates (AED→USD 0.2723, EUR→USD 1.0812).

## Discrepancy Types
- `matched` — exact amount match after fee normalization
- `fuzzy_match` — matched with confidence < 1.0
- `missing_from_settlement` — in expected but not in any PSP report
- `unexpected_in_settlement` — in PSP report but not in expected
- `amount_mismatch` — matched transaction but amounts differ
- `fee_mismatch` — matched transaction but fee doesn't match PSP's configured rate

## Test Data (`backend/testdata/`)
- `expected_transactions.json` — 100 LuxeCart transactions over Dec 1-7, 2024
- `pspa_settlement.csv` — PSP A daily CSV reports
- `pspb_settlement.json` — PSP B weekly JSON report
- `pspc_settlement.txt` — PSP C pipe-delimited custom report
- Planted discrepancies: 7 missing, 3 unexpected, 5 amount mismatches, 2 fee errors

## Key Commands
```bash
make dev          # start all services via docker-compose
make test         # run all tests
make seed         # seed database with test data
make openapi      # view OpenAPI docs via swagger-ui
make logs         # tail all service logs
```

## API Endpoints
```
POST   /api/v1/transactions/bulk          Import expected transactions
GET    /api/v1/transactions               List transactions
POST   /api/v1/settlements/upload         Upload PSP settlement file
GET    /api/v1/settlements/batches        List settlement batches
POST   /api/v1/reconciliation/run         Start reconciliation
GET    /api/v1/reconciliation/runs        List runs
GET    /api/v1/reconciliation/runs/:id    Run details + match results
GET    /api/v1/reconciliation/runs/:id/summary   Summary report
GET    /health                            Health check
GET    /metrics                           Prometheus metrics
```

## Fee Configuration
| PSP | Percentage | Fixed Fee |
|-----|------------|-----------|
| PSP A | 2.9% | $0.30 |
| PSP B | 2.5% | 0.25 AED |
| PSP C | 3.1% | $0.25 |
