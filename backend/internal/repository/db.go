package repository

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS transactions (
		id                   TEXT PRIMARY KEY,
		external_id          TEXT,
		amount               DOUBLE PRECISION NOT NULL,
		currency             TEXT NOT NULL,
		status               TEXT NOT NULL,
		type                 TEXT NOT NULL,
		psp_name             TEXT NOT NULL,
		customer_ref         TEXT,
		created_at           TIMESTAMPTZ NOT NULL,
		expected_settle_date TIMESTAMPTZ NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_transactions_psp ON transactions(psp_name)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_created ON transactions(created_at)`,

	`CREATE TABLE IF NOT EXISTS settlement_batches (
		id           TEXT PRIMARY KEY,
		psp_name     TEXT NOT NULL,
		format       TEXT NOT NULL,
		filename     TEXT,
		period_start TIMESTAMPTZ,
		period_end   TIMESTAMPTZ,
		total_gross  DOUBLE PRECISION DEFAULT 0,
		total_fees   DOUBLE PRECISION DEFAULT 0,
		total_net    DOUBLE PRECISION DEFAULT 0,
		currency     TEXT,
		record_count INTEGER DEFAULT 0,
		status       TEXT NOT NULL,
		created_at   TIMESTAMPTZ NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS settlement_records (
		id                   TEXT PRIMARY KEY,
		batch_id             TEXT NOT NULL REFERENCES settlement_batches(id),
		psp_name             TEXT NOT NULL,
		psp_transaction_ref  TEXT NOT NULL,
		original_txn_ref     TEXT,
		gross_amount         DOUBLE PRECISION NOT NULL,
		fee                  DOUBLE PRECISION NOT NULL,
		net_amount           DOUBLE PRECISION NOT NULL,
		currency             TEXT NOT NULL,
		settlement_date      TIMESTAMPTZ NOT NULL,
		transaction_date     TIMESTAMPTZ,
		status               TEXT NOT NULL,
		type                 TEXT NOT NULL DEFAULT 'sale',
		created_at           TIMESTAMPTZ NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_records_batch ON settlement_records(batch_id)`,
	`CREATE INDEX IF NOT EXISTS idx_records_psp_ref ON settlement_records(psp_transaction_ref)`,
	`CREATE INDEX IF NOT EXISTS idx_records_orig_ref ON settlement_records(original_txn_ref)`,
	`CREATE INDEX IF NOT EXISTS idx_records_amount ON settlement_records(gross_amount, currency)`,

	`CREATE TABLE IF NOT EXISTS reconciliation_runs (
		id               TEXT PRIMARY KEY,
		name             TEXT,
		period_start     TIMESTAMPTZ NOT NULL,
		period_end       TIMESTAMPTZ NOT NULL,
		status           TEXT NOT NULL,
		total_expected   DOUBLE PRECISION DEFAULT 0,
		total_settled    DOUBLE PRECISION DEFAULT 0,
		matched_count    INTEGER DEFAULT 0,
		missing_count    INTEGER DEFAULT 0,
		unexpected_count INTEGER DEFAULT 0,
		mismatch_count   INTEGER DEFAULT 0,
		fee_error_count  INTEGER DEFAULT 0,
		match_rate       DOUBLE PRECISION DEFAULT 0,
		discrepancy      DOUBLE PRECISION DEFAULT 0,
		created_at       TIMESTAMPTZ NOT NULL,
		completed_at     TIMESTAMPTZ
	)`,

	`CREATE TABLE IF NOT EXISTS match_results (
		id                    TEXT PRIMARY KEY,
		reconciliation_run_id TEXT NOT NULL REFERENCES reconciliation_runs(id),
		transaction_id        TEXT REFERENCES transactions(id),
		settlement_record_id  TEXT REFERENCES settlement_records(id),
		status                TEXT NOT NULL,
		confidence_score      DOUBLE PRECISION DEFAULT 1.0,
		expected_amount       DOUBLE PRECISION DEFAULT 0,
		actual_amount         DOUBLE PRECISION DEFAULT 0,
		amount_diff_usd       DOUBLE PRECISION DEFAULT 0,
		expected_fee          DOUBLE PRECISION DEFAULT 0,
		actual_fee            DOUBLE PRECISION DEFAULT 0,
		currency              TEXT,
		psp_name              TEXT,
		notes                 TEXT,
		created_at            TIMESTAMPTZ NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS idx_match_run ON match_results(reconciliation_run_id)`,
	`CREATE INDEX IF NOT EXISTS idx_match_status ON match_results(status)`,

	`CREATE TABLE IF NOT EXISTS psp_fee_configs (
		psp_name        TEXT PRIMARY KEY,
		percentage_rate DOUBLE PRECISION NOT NULL,
		fixed_fee       DOUBLE PRECISION NOT NULL,
		currency        TEXT NOT NULL,
		created_at      TIMESTAMPTZ NOT NULL
	)`,

	`INSERT INTO psp_fee_configs(psp_name, percentage_rate, fixed_fee, currency, created_at)
	 VALUES
	     ('pspa', 0.029, 0.30, 'USD', NOW()),
	     ('pspb', 0.025, 0.25, 'AED', NOW()),
	     ('pspc', 0.031, 0.25, 'USD', NOW())
	 ON CONFLICT (psp_name) DO NOTHING`,
}
