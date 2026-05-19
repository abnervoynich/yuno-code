# LuxeCart Settlement Reconciliation Engine

A multi-PSP settlement reconciliation engine built for the Yuno challenge. Designed for LuxeCart, a luxury e-commerce platform processing $12M+/month across MENA.

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.22+ | Build and run the backend locally / run tests |
| [Node.js](https://nodejs.org/) | 18+ | Build and run the frontend locally |
| [Docker](https://docs.docker.com/get-docker/) | 24+ | Run all services via `docker-compose` |
| [Docker Compose](https://docs.docker.com/compose/install/) | v2+ | Orchestrate all containers (`make dev`) |
| [Make](https://www.gnu.org/software/make/) | any | Run Makefile targets (`make dev`, `make test`, …) |
| [curl](https://curl.se/) + [jq](https://jqlang.github.io/jq/) | any | Used by `make seed-load` to POST test data via the API |

> **Windows users:** Make is available via [Chocolatey](https://chocolatey.org/) (`choco install make`), [Scoop](https://scoop.sh/) (`scoop install make`), or the [Git for Windows](https://gitforwindows.org/) shell. `curl` ships with Windows 10+ and `jq` can be installed via `winget install jqlang.jq`.

### Minimal setup (Docker only)

If you only want to run the full stack with Docker and skip local Go/Node development:

```
Docker 24+ with Docker Compose v2
Make (or run docker-compose commands manually)
curl + jq (for make seed-load)
```

### Local development setup (without Docker)

To run backend and frontend directly on your machine:

```
Go 1.22+    (backend: make backend-run)
Node.js 18+ (frontend: make frontend-run / npm run dev)
```

---

## Quick Start

```bash
# 1. Generate test data
make seed-generate

# 2. Start all services (requires Docker)
make dev

# 3. Load test data into the running backend
make seed-load

# 4. Run reconciliation over the full Dec 1-7 test period
make reconcile

# 5. View results in the dashboard
open http://localhost:3030
```

**Service URLs** (defaults — edit `.env` to change ports):
| Service | URL |
|---------|-----|
| Backend API | http://localhost:8090 |
| Frontend UI | http://localhost:3030 |
| API Docs (Swagger) | `make openapi` → http://localhost:8081 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3001 (admin/admin) |

---

## Using the UI

Open **http://localhost:3030** after running `make dev`.

### Step 1 — Import expected transactions

Go to **Transactions** and click **Import JSON**, then upload `backend/testdata/expected_transactions.json`.

> Shortcut: `make seed-load` does steps 1–3 automatically via the API.

### Step 2 — Upload PSP settlement files

Go to **Settlements**, select the PSP, and upload the matching file:

| PSP | File |
|-----|------|
| PSP A | `backend/testdata/pspa_settlement.csv` |
| PSP B | `backend/testdata/pspb_settlement.json` |
| PSP C | `backend/testdata/pspc_settlement.txt` |

### Step 3 — Run reconciliation

Go to **Reconciliation**, enter a name (e.g. `Dec 2024`) and the date range `2024-12-01` → `2024-12-07`, then click **Run**.

> Shortcut: `make reconcile` triggers this automatically.

### Step 4 — Explore results

Click the run to open its detail page. Use the tabs to drill into each discrepancy category:

| Tab | What it shows |
|-----|---------------|
| Summary | KPIs: matched count, missing, mismatches, fee errors |
| Missing | 7 transactions never reported by any PSP |
| Unexpected | 3 ghost records present in settlement files but not in expected transactions |
| Amount Mismatch | 5 transactions where the PSP reported a different gross amount |
| Fee Errors | 2 transactions where the PSP overcharged fees vs. their rate card |
| Matched | All successfully reconciled transactions |

### Step 5 — Dashboard

The **Dashboard** page shows aggregate charts across all runs: PSP distribution and reconciliation result breakdown.

---

## Architecture (300-500 word writeup)

### Overview

The reconciliation engine is a single Go microservice backed by SQLite, chosen for simplicity and portability. The frontend is a React/Vite/Tailwind SPA, and monitoring is provided by Prometheus + Grafana, all orchestrated via Docker Compose.

```
Frontend (React) -> Go REST API -> SQLite
                              -> Prometheus metrics -> Grafana
```

### Matching Logic

The engine uses a three-tier matching strategy:

**Tier 1 - Exact ID match** (confidence 1.0): The settlement record's `original_txn_ref` exactly matches the expected transaction's `external_id`. PSP A uses this directly.

**Tier 2 - Normalized ID match** (confidence 0.95): Known PSP prefixes (`PSPB_`) are stripped before comparison. PSP B uses this - its settlement file contains references like `PSPB_TXN-20241201-00001`, which normalizes to `TXN-20241201-00001`.

**Tier 3 - Fuzzy match** (confidence 0.60-0.85): Used for PSP C, which assigns its own opaque reference numbers. The matching engine searches for a settlement record from the same PSP with the same currency and an amount within 10% of the expected, then scores by timestamp proximity:
- <=5 minutes apart: 0.85
- <=30 minutes: 0.75
- <=2 hours: 0.65
- <=24 hours: 0.60

For each match, the engine then performs exact amount comparison (tolerance $0.02 for float rounding) and fee validation (PSP-configured rate +/- 1% + $0.05 buffer). If amounts match but fees don't: `fee_mismatch`. If amounts differ: `amount_mismatch`. Any settlement records not claimed after all expected transactions are processed: `unexpected_in_settlement`.

### Format Differences

Each PSP parser implements the `Parser` interface with a single `Parse(io.Reader) ([]SettlementRecord, error)` method:

- **PSP A (CSV)**: Uses Go's standard `encoding/csv`. Column headers are mapped by name so column ordering is flexible. Dates are parsed with a flexible multi-format parser.
- **PSP B (JSON)**: A strongly-typed Go struct mirrors the PSP B report structure. The `processed_at` field handles RFC3339 timestamps with timezone offsets.
- **PSP C (custom pipe-delimited)**: A line-by-line scanner splits on `|`. Special lines (`PSPC_REPORT`, `PERIOD`, `SUMMARY`, `END_REPORT`) are handled individually. Date+time fields are in compact `YYYYMMDD`/`HHMMSS` format and are reformatted before parsing.

### Trade-offs

- **SQLite over PostgreSQL**: Simplifies deployment (no separate DB container). At LuxeCart's scale, a proper database would be needed, but SQLite handles millions of records comfortably for a POC.
- **Synchronous reconciliation**: The reconciliation run happens synchronously in the HTTP handler thread. At scale, this should be a background job with status polling.
- **Static exchange rates**: Real deployment needs live FX rates from a provider like Open Exchange Rates. The static rates here are accurate as of Dec 2024.
- **In-memory matching**: The matching engine loads all transactions and settlement records into memory. For very large datasets (10M+ rows), a SQL-based matching approach would be more appropriate.

### Test Data

100 transactions over Dec 1-7, 2024 across 3 PSPs and 3 currencies (USD, AED, EUR), with deliberately planted discrepancies:
- **7 missing** from settlement (transactions never reported by PSP)
- **3 unexpected** entries in settlement (PSP records with no corresponding transaction)
- **5 amount mismatches** (PSP reported a different gross amount)
- **2 fee errors** (PSP charged an inflated fee inconsistent with their rate card)

---

## API Reference

See `api/openapi.yaml` or run `make openapi` to view the interactive Swagger UI.

Key endpoints:
```
POST /api/v1/transactions/bulk            Import expected transactions
POST /api/v1/settlements/upload           Upload PSP settlement file (multipart)
POST /api/v1/reconciliation/run           Run reconciliation for a date range
GET  /api/v1/reconciliation/runs/:id/summary   Full reconciliation summary
```

## Tests

```bash
make test              # all tests
make test-unit         # parser + matching engine unit tests
make test-integration  # full HTTP + DB integration tests
```
