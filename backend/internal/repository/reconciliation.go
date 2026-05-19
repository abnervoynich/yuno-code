package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

type ReconciliationRepo struct {
	db *sql.DB
}

func NewReconciliationRepo(db *sql.DB) *ReconciliationRepo {
	return &ReconciliationRepo{db: db}
}

func (r *ReconciliationRepo) CreateRun(ctx context.Context, run *domain.ReconciliationRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	run.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO reconciliation_runs
		(id, name, period_start, period_end, status, total_expected, total_settled,
		 matched_count, missing_count, unexpected_count, mismatch_count, fee_error_count,
		 match_rate, discrepancy, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		run.ID, run.Name, run.PeriodStart.UTC(), run.PeriodEnd.UTC(),
		run.Status, run.TotalExpected, run.TotalSettled,
		run.MatchedCount, run.MissingCount, run.UnexpectedCount, run.MismatchCount, run.FeeErrorCount,
		run.MatchRate, run.Discrepancy, run.CreatedAt, run.CompletedAt,
	)
	return err
}

func (r *ReconciliationRepo) UpdateRun(ctx context.Context, run *domain.ReconciliationRun) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE reconciliation_runs SET
		status = $1, total_expected = $2, total_settled = $3,
		matched_count = $4, missing_count = $5, unexpected_count = $6,
		mismatch_count = $7, fee_error_count = $8, match_rate = $9,
		discrepancy = $10, completed_at = $11
		WHERE id = $12`,
		run.Status, run.TotalExpected, run.TotalSettled,
		run.MatchedCount, run.MissingCount, run.UnexpectedCount,
		run.MismatchCount, run.FeeErrorCount, run.MatchRate,
		run.Discrepancy, run.CompletedAt, run.ID,
	)
	return err
}

func (r *ReconciliationRepo) GetRun(ctx context.Context, id string) (*domain.ReconciliationRun, error) {
	var run domain.ReconciliationRun
	var completedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, period_start, period_end, status,
		       total_expected, total_settled, matched_count, missing_count,
		       unexpected_count, mismatch_count, fee_error_count, match_rate,
		       discrepancy, created_at, completed_at
		FROM reconciliation_runs WHERE id = $1`, id).Scan(
		&run.ID, &run.Name, &run.PeriodStart, &run.PeriodEnd, &run.Status,
		&run.TotalExpected, &run.TotalSettled, &run.MatchedCount, &run.MissingCount,
		&run.UnexpectedCount, &run.MismatchCount, &run.FeeErrorCount, &run.MatchRate,
		&run.Discrepancy, &run.CreatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, err
}

func (r *ReconciliationRepo) ListRuns(ctx context.Context) ([]domain.ReconciliationRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, period_start, period_end, status,
		       total_expected, total_settled, matched_count, missing_count,
		       unexpected_count, mismatch_count, fee_error_count, match_rate,
		       discrepancy, created_at, completed_at
		FROM reconciliation_runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.ReconciliationRun
	for rows.Next() {
		var run domain.ReconciliationRun
		var completedAt sql.NullTime
		if err := rows.Scan(
			&run.ID, &run.Name, &run.PeriodStart, &run.PeriodEnd, &run.Status,
			&run.TotalExpected, &run.TotalSettled, &run.MatchedCount, &run.MissingCount,
			&run.UnexpectedCount, &run.MismatchCount, &run.FeeErrorCount, &run.MatchRate,
			&run.Discrepancy, &run.CreatedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *ReconciliationRepo) BulkInsertResults(ctx context.Context, results []domain.MatchResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO match_results
		(id, reconciliation_run_id, transaction_id, settlement_record_id, status,
		 confidence_score, expected_amount, actual_amount, amount_diff_usd,
		 expected_fee, actual_fee, currency, psp_name, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range results {
		res := &results[i]
		if res.ID == "" {
			res.ID = uuid.NewString()
		}
		res.CreatedAt = time.Now().UTC()

		txnID := sql.NullString{String: res.TransactionID, Valid: res.TransactionID != ""}
		settleID := sql.NullString{String: res.SettlementRecordID, Valid: res.SettlementRecordID != ""}

		_, err = stmt.ExecContext(ctx,
			res.ID, res.ReconciliationRunID, txnID, settleID,
			res.Status, res.ConfidenceScore,
			res.ExpectedAmount, res.ActualAmount, res.AmountDiffUSD,
			res.ExpectedFee, res.ActualFee,
			res.Currency, res.PSPName, res.Notes, res.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert result: %w", err)
		}
	}

	return tx.Commit()
}

func (r *ReconciliationRepo) ListResults(ctx context.Context, runID string, status string) ([]domain.MatchResult, error) {
	q := `SELECT id, reconciliation_run_id, transaction_id, settlement_record_id,
		         status, confidence_score, expected_amount, actual_amount, amount_diff_usd,
		         expected_fee, actual_fee, currency, psp_name, notes, created_at
		  FROM match_results WHERE reconciliation_run_id = $1`
	args := []any{runID}

	if status != "" {
		q += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, status)
	}
	q += " ORDER BY ABS(amount_diff_usd) DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MatchResult
	for rows.Next() {
		var res domain.MatchResult
		var txnID, settleID sql.NullString
		if err := rows.Scan(
			&res.ID, &res.ReconciliationRunID, &txnID, &settleID,
			&res.Status, &res.ConfidenceScore,
			&res.ExpectedAmount, &res.ActualAmount, &res.AmountDiffUSD,
			&res.ExpectedFee, &res.ActualFee, &res.Currency, &res.PSPName,
			&res.Notes, &res.CreatedAt,
		); err != nil {
			return nil, err
		}
		if txnID.Valid {
			res.TransactionID = txnID.String
		}
		if settleID.Valid {
			res.SettlementRecordID = settleID.String
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *ReconciliationRepo) PSPBreakdowns(ctx context.Context, runID string) ([]domain.PSPBreakdown, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			psp_name,
			SUM(expected_amount) as total_expected,
			SUM(actual_amount) as total_settled,
			SUM(CASE WHEN status IN ('matched','fuzzy_match') THEN 1 ELSE 0 END) as matched,
			SUM(CASE WHEN status = 'missing_from_settlement' THEN 1 ELSE 0 END) as missing,
			SUM(CASE WHEN status = 'unexpected_in_settlement' THEN 1 ELSE 0 END) as unexpected,
			SUM(CASE WHEN status IN ('amount_mismatch','fee_mismatch') THEN 1 ELSE 0 END) as mismatch,
			COUNT(*) as total
		FROM match_results
		WHERE reconciliation_run_id = $1
		GROUP BY psp_name`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var breakdowns []domain.PSPBreakdown
	for rows.Next() {
		var b domain.PSPBreakdown
		var total int
		if err := rows.Scan(
			&b.PSPName, &b.TotalExpected, &b.TotalSettled,
			&b.MatchedCount, &b.MissingCount, &b.UnexpectedCount,
			&b.MismatchCount, &total,
		); err != nil {
			return nil, err
		}
		if total > 0 {
			b.MatchRate = float64(b.MatchedCount) / float64(total) * 100
		}
		breakdowns = append(breakdowns, b)
	}
	return breakdowns, rows.Err()
}

func (r *ReconciliationRepo) GetFeeConfig(ctx context.Context, pspName string) (*domain.PSPFeeConfig, error) {
	var cfg domain.PSPFeeConfig
	err := r.db.QueryRowContext(ctx,
		`SELECT psp_name, percentage_rate, fixed_fee, currency FROM psp_fee_configs WHERE psp_name = $1`,
		pspName).Scan(&cfg.PSPName, &cfg.PercentageRate, &cfg.FixedFee, &cfg.Currency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &cfg, err
}
