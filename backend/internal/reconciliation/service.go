package reconciliation

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/matching"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
)

type Service struct {
	txnRepo    *repository.TransactionRepo
	settleRepo *repository.SettlementRepo
	recoRepo   *repository.ReconciliationRepo
}

func NewService(
	txnRepo *repository.TransactionRepo,
	settleRepo *repository.SettlementRepo,
	recoRepo *repository.ReconciliationRepo,
) *Service {
	return &Service{
		txnRepo:    txnRepo,
		settleRepo: settleRepo,
		recoRepo:   recoRepo,
	}
}

type RunRequest struct {
	Name        string
	PeriodStart time.Time
	PeriodEnd   time.Time
}

func (s *Service) Run(ctx context.Context, req RunRequest) (*domain.ReconciliationRun, error) {
	run := &domain.ReconciliationRun{
		Name:        req.Name,
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		Status:      "running",
	}
	if err := s.recoRepo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	err := s.execute(ctx, run)
	if err != nil {
		run.Status = "failed"
		_ = s.recoRepo.UpdateRun(ctx, run)
		return nil, err
	}

	return run, nil
}

func (s *Service) execute(ctx context.Context, run *domain.ReconciliationRun) error {
	// Load transactions for period
	transactions, err := s.txnRepo.List(ctx, "", run.PeriodStart, run.PeriodEnd)
	if err != nil {
		return fmt.Errorf("load transactions: %w", err)
	}

	// Load settlement records for period
	records, err := s.settleRepo.AllRecordsForPeriod(ctx, run.PeriodStart, run.PeriodEnd)
	if err != nil {
		return fmt.Errorf("load settlement records: %w", err)
	}

	// Load fee configs
	psps := []string{"pspa", "pspb", "pspc"}
	var feeConfigs []*domain.PSPFeeConfig
	for _, psp := range psps {
		cfg, err := s.recoRepo.GetFeeConfig(ctx, psp)
		if err != nil {
			return fmt.Errorf("load fee config for %s: %w", psp, err)
		}
		if cfg != nil {
			feeConfigs = append(feeConfigs, cfg)
		}
	}

	// Run matching engine
	engine := matching.NewEngine(feeConfigs)
	result := engine.Match(run.ID, transactions, records)

	// Collect all match results
	var allResults []domain.MatchResult
	allResults = append(allResults, result.Matched...)
	allResults = append(allResults, result.Missing...)
	allResults = append(allResults, result.Unexpected...)

	// Compute run statistics
	var totalExpected, totalSettled float64
	for _, txn := range transactions {
		totalExpected += domain.ToUSD(txn.Amount, txn.Currency)
	}
	for _, rec := range records {
		totalSettled += domain.ToUSD(rec.GrossAmount, rec.Currency)
	}

	matchedCount := countByStatus(allResults, domain.MatchMatched, domain.MatchFuzzy)
	mismatchCount := countByStatus(allResults, domain.MatchAmountMismatch)
	feeErrorCount := countByStatus(allResults, domain.MatchFeeMismatch)
	totalTxns := len(transactions)

	run.TotalExpected = totalExpected
	run.TotalSettled = totalSettled
	run.MatchedCount = matchedCount
	run.MissingCount = len(result.Missing)
	run.UnexpectedCount = len(result.Unexpected)
	run.MismatchCount = mismatchCount
	run.FeeErrorCount = feeErrorCount
	run.Discrepancy = totalExpected - totalSettled
	if totalTxns > 0 {
		run.MatchRate = float64(matchedCount) / float64(totalTxns) * 100
	}

	now := time.Now().UTC()
	run.Status = "completed"
	run.CompletedAt = &now

	// Save results
	if err := s.recoRepo.BulkInsertResults(ctx, allResults); err != nil {
		return fmt.Errorf("save results: %w", err)
	}
	if err := s.recoRepo.UpdateRun(ctx, run); err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	return nil
}

func (s *Service) Summary(ctx context.Context, runID string) (*domain.ReconciliationSummary, error) {
	run, err := s.recoRepo.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}

	breakdowns, err := s.recoRepo.PSPBreakdowns(ctx, runID)
	if err != nil {
		return nil, err
	}

	allResults, err := s.recoRepo.ListResults(ctx, runID, "")
	if err != nil {
		return nil, err
	}

	summary := &domain.ReconciliationSummary{
		Run:           *run,
		PSPBreakdowns: breakdowns,
	}

	for _, r := range allResults {
		r := r
		switch r.Status {
		case domain.MatchMissing:
			summary.MissingItems = append(summary.MissingItems, r)
		case domain.MatchUnexpected:
			summary.UnexpectedItems = append(summary.UnexpectedItems, r)
		case domain.MatchAmountMismatch:
			summary.AmountMismatches = append(summary.AmountMismatches, r)
		case domain.MatchFeeMismatch:
			summary.FeeErrors = append(summary.FeeErrors, r)
		}
	}

	// Top discrepancies by absolute dollar amount
	var discrepancies []domain.MatchResult
	for _, r := range allResults {
		if r.Status != domain.MatchMatched && r.Status != domain.MatchFuzzy {
			discrepancies = append(discrepancies, r)
		}
	}
	sort.Slice(discrepancies, func(i, j int) bool {
		ai := discrepancies[i].AmountDiffUSD
		aj := discrepancies[j].AmountDiffUSD
		if ai < 0 {
			ai = -ai
		}
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})
	if len(discrepancies) > 10 {
		discrepancies = discrepancies[:10]
	}
	summary.TopDiscrepancies = discrepancies

	return summary, nil
}

func countByStatus(results []domain.MatchResult, statuses ...domain.MatchStatus) int {
	set := make(map[domain.MatchStatus]bool)
	for _, s := range statuses {
		set[s] = true
	}
	count := 0
	for _, r := range results {
		if set[r.Status] {
			count++
		}
	}
	return count
}
