package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

type SettlementRepo struct {
	db *sql.DB
}

func NewSettlementRepo(db *sql.DB) *SettlementRepo {
	return &SettlementRepo{db: db}
}

func (r *SettlementRepo) CreateBatch(ctx context.Context, b *domain.SettlementBatch) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	b.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settlement_batches
		(id, psp_name, format, filename, period_start, period_end, total_gross, total_fees, total_net, currency, record_count, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		b.ID, b.PSPName, b.Format, b.Filename,
		b.PeriodStart.UTC(), b.PeriodEnd.UTC(),
		b.TotalGross, b.TotalFees, b.TotalNet, b.Currency,
		b.RecordCount, b.Status, b.CreatedAt,
	)
	return err
}

func (r *SettlementRepo) BulkInsertRecords(ctx context.Context, records []domain.SettlementRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO settlement_records
		(id, batch_id, psp_name, psp_transaction_ref, original_txn_ref, gross_amount, fee, net_amount, currency, settlement_date, transaction_date, status, type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range records {
		rec := &records[i]
		if rec.ID == "" {
			rec.ID = uuid.NewString()
		}
		rec.CreatedAt = time.Now().UTC()
		_, err = stmt.ExecContext(ctx,
			rec.ID, rec.BatchID, rec.PSPName, rec.PSPTransactionRef,
			rec.OriginalTxnRef, rec.GrossAmount, rec.Fee, rec.NetAmount,
			rec.Currency, rec.SettlementDate.UTC(), rec.TransactionDate.UTC(),
			rec.Status, rec.Type, rec.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert record %s: %w", rec.PSPTransactionRef, err)
		}
	}

	return tx.Commit()
}

func (r *SettlementRepo) ListBatches(ctx context.Context) ([]domain.SettlementBatch, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, psp_name, format, filename, period_start, period_end,
		       total_gross, total_fees, total_net, currency, record_count, status, created_at
		FROM settlement_batches ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []domain.SettlementBatch
	for rows.Next() {
		var b domain.SettlementBatch
		if err := rows.Scan(&b.ID, &b.PSPName, &b.Format, &b.Filename,
			&b.PeriodStart, &b.PeriodEnd,
			&b.TotalGross, &b.TotalFees, &b.TotalNet, &b.Currency,
			&b.RecordCount, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}

func (r *SettlementRepo) GetBatch(ctx context.Context, id string) (*domain.SettlementBatch, error) {
	var b domain.SettlementBatch
	err := r.db.QueryRowContext(ctx, `
		SELECT id, psp_name, format, filename, period_start, period_end,
		       total_gross, total_fees, total_net, currency, record_count, status, created_at
		FROM settlement_batches WHERE id = $1`, id).Scan(
		&b.ID, &b.PSPName, &b.Format, &b.Filename,
		&b.PeriodStart, &b.PeriodEnd,
		&b.TotalGross, &b.TotalFees, &b.TotalNet, &b.Currency,
		&b.RecordCount, &b.Status, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (r *SettlementRepo) ListRecords(ctx context.Context, batchID string) ([]domain.SettlementRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, psp_name, psp_transaction_ref, original_txn_ref,
		       gross_amount, fee, net_amount, currency, settlement_date, transaction_date,
		       status, type, created_at
		FROM settlement_records WHERE batch_id = $1 ORDER BY settlement_date ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (r *SettlementRepo) ListRecordsByPSP(ctx context.Context, pspName string, from, to time.Time) ([]domain.SettlementRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, psp_name, psp_transaction_ref, original_txn_ref,
		       gross_amount, fee, net_amount, currency, settlement_date, transaction_date,
		       status, type, created_at
		FROM settlement_records
		WHERE psp_name = $1 AND transaction_date >= $2 AND transaction_date <= $3
		ORDER BY transaction_date ASC`, pspName, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (r *SettlementRepo) AllRecordsForPeriod(ctx context.Context, from, to time.Time) ([]domain.SettlementRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, batch_id, psp_name, psp_transaction_ref, original_txn_ref,
		       gross_amount, fee, net_amount, currency, settlement_date, transaction_date,
		       status, type, created_at
		FROM settlement_records
		WHERE transaction_date >= $1 AND transaction_date <= $2
		ORDER BY psp_name, transaction_date ASC`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

func scanRecords(rows *sql.Rows) ([]domain.SettlementRecord, error) {
	var results []domain.SettlementRecord
	for rows.Next() {
		var r domain.SettlementRecord
		var origRef sql.NullString
		if err := rows.Scan(
			&r.ID, &r.BatchID, &r.PSPName, &r.PSPTransactionRef, &origRef,
			&r.GrossAmount, &r.Fee, &r.NetAmount, &r.Currency,
			&r.SettlementDate, &r.TransactionDate,
			&r.Status, &r.Type, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		if origRef.Valid {
			r.OriginalTxnRef = origRef.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
