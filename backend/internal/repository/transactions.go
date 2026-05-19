package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

type TransactionRepo struct {
	db *sql.DB
}

func NewTransactionRepo(db *sql.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) BulkInsert(ctx context.Context, txns []domain.Transaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO transactions
		(id, external_id, amount, currency, status, type, psp_name, customer_ref, created_at, expected_settle_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range txns {
		t := &txns[i]
		if t.ID == "" {
			t.ID = uuid.NewString()
		}
		_, err = stmt.ExecContext(ctx,
			t.ID, t.ExternalID, t.Amount, t.Currency,
			t.Status, t.Type, t.PSPName, t.CustomerRef,
			t.CreatedAt.UTC(), t.ExpectedSettleDate.UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert transaction %s: %w", t.ID, err)
		}
	}

	return tx.Commit()
}

func (r *TransactionRepo) List(ctx context.Context, pspName string, from, to time.Time) ([]domain.Transaction, error) {
	q := `SELECT id, external_id, amount, currency, status, type, psp_name, customer_ref, created_at, expected_settle_date
		  FROM transactions WHERE 1=1`
	args := []any{}

	if pspName != "" {
		q += fmt.Sprintf(" AND psp_name = $%d", len(args)+1)
		args = append(args, pspName)
	}
	if !from.IsZero() {
		q += fmt.Sprintf(" AND created_at >= $%d", len(args)+1)
		args = append(args, from.UTC())
	}
	if !to.IsZero() {
		q += fmt.Sprintf(" AND created_at <= $%d", len(args)+1)
		args = append(args, to.UTC())
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactions(rows)
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, external_id, amount, currency, status, type, psp_name, customer_ref, created_at, expected_settle_date
		FROM transactions WHERE id = $1`, id)
	t := &domain.Transaction{}
	err := row.Scan(&t.ID, &t.ExternalID, &t.Amount, &t.Currency,
		&t.Status, &t.Type, &t.PSPName, &t.CustomerRef,
		&t.CreatedAt, &t.ExpectedSettleDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (r *TransactionRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions").Scan(&count)
	return count, err
}

func scanTransactions(rows *sql.Rows) ([]domain.Transaction, error) {
	var results []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(
			&t.ID, &t.ExternalID, &t.Amount, &t.Currency,
			&t.Status, &t.Type, &t.PSPName, &t.CustomerRef,
			&t.CreatedAt, &t.ExpectedSettleDate,
		); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	return results, rows.Err()
}
