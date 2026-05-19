package matching

import (
	"math"
	"strings"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
)

const (
	amountToleranceUSD = 0.02 // 2 cents tolerance for float rounding
	feeTolerancePct    = 0.01 // 1% tolerance for fee validation
)

// Engine matches expected transactions against settlement records.
type Engine struct {
	feeConfigs map[string]*domain.PSPFeeConfig
}

// NewEngine creates a matching engine with the given PSP fee configurations.
func NewEngine(feeConfigs []*domain.PSPFeeConfig) *Engine {
	cfgMap := make(map[string]*domain.PSPFeeConfig, len(feeConfigs))
	for _, fc := range feeConfigs {
		cfgMap[fc.PSPName] = fc
	}
	return &Engine{feeConfigs: cfgMap}
}

// MatchResult is the outcome of running Match.
type Result struct {
	Matched    []domain.MatchResult
	Missing    []domain.MatchResult
	Unexpected []domain.MatchResult
}

// Match performs reconciliation between expected transactions and settlement records.
func (e *Engine) Match(runID string, transactions []domain.Transaction, records []domain.SettlementRecord) Result {
	// Build lookup structures
	byExactID := make(map[string]*domain.SettlementRecord)    // original_txn_ref → record
	byNormID := make(map[string]*domain.SettlementRecord)     // normalized ref → record
	usedRecords := make(map[string]bool)                      // record ID → matched

	for i := range records {
		rec := &records[i]
		if rec.OriginalTxnRef != "" {
			byExactID[rec.OriginalTxnRef] = rec
			byNormID[normalizeRef(rec.OriginalTxnRef)] = rec
		}
		byNormID[normalizeRef(rec.PSPTransactionRef)] = rec
	}

	var result Result

	for _, txn := range transactions {
		mr := e.matchTransaction(runID, txn, records, byExactID, byNormID, usedRecords)
		switch mr.Status {
		case domain.MatchMatched, domain.MatchFuzzy:
			result.Matched = append(result.Matched, mr)
		case domain.MatchMissing:
			result.Missing = append(result.Missing, mr)
		default:
			result.Matched = append(result.Matched, mr)
		}
	}

	// Any settlement records not matched → unexpected
	for i := range records {
		rec := &records[i]
		if !usedRecords[rec.ID] {
			result.Unexpected = append(result.Unexpected, domain.MatchResult{
				ReconciliationRunID: runID,
				SettlementRecordID:  rec.ID,
				Status:              domain.MatchUnexpected,
				ConfidenceScore:     1.0,
				ActualAmount:        domain.ToUSD(rec.GrossAmount, rec.Currency),
				Currency:            rec.Currency,
				PSPName:             rec.PSPName,
				Notes:               "settlement record has no matching expected transaction",
				SettlementRecord:    rec,
			})
		}
	}

	return result
}

func (e *Engine) matchTransaction(
	runID string,
	txn domain.Transaction,
	allRecords []domain.SettlementRecord,
	byExactID map[string]*domain.SettlementRecord,
	byNormID map[string]*domain.SettlementRecord,
	used map[string]bool,
) domain.MatchResult {

	expectedUSD := domain.ToUSD(txn.Amount, txn.Currency)
	base := domain.MatchResult{
		ReconciliationRunID: runID,
		TransactionID:       txn.ID,
		ExpectedAmount:      expectedUSD,
		Currency:            txn.Currency,
		PSPName:             txn.PSPName,
		Transaction:         &txn,
	}

	// Strategy 1: exact original ID match
	if rec, ok := byExactID[txn.ExternalID]; ok && !used[rec.ID] {
		used[rec.ID] = true
		return e.buildMatchResult(base, rec, 1.0, txn)
	}

	// Strategy 2: normalized ID (strip PSP prefixes)
	normID := normalizeRef(txn.ExternalID)
	if rec, ok := byNormID[normID]; ok && !used[rec.ID] {
		used[rec.ID] = true
		return e.buildMatchResult(base, rec, 0.95, txn)
	}

	// Strategy 3: fuzzy match by amount + timestamp (PSP C style)
	if rec, score := e.fuzzyMatch(txn, allRecords, used); rec != nil {
		used[rec.ID] = true
		return e.buildMatchResult(base, rec, score, txn)
	}

	// No match found
	base.Status = domain.MatchMissing
	base.Notes = "transaction not found in any settlement report"
	return base
}

func (e *Engine) buildMatchResult(base domain.MatchResult, rec *domain.SettlementRecord, confidence float64, txn domain.Transaction) domain.MatchResult {
	actualUSD := domain.ToUSD(rec.GrossAmount, rec.Currency)
	diff := math.Abs(actualUSD - base.ExpectedAmount)

	base.SettlementRecordID = rec.ID
	base.ActualAmount = actualUSD
	base.AmountDiffUSD = actualUSD - base.ExpectedAmount
	base.ActualFee = rec.Fee
	base.ConfidenceScore = confidence
	base.SettlementRecord = rec

	// Check fee validation — compute expected fee in USD using correct currency handling:
	// percentage is applied to gross in its native currency, fixed fee is in config currency.
	if cfg, ok := e.feeConfigs[rec.PSPName]; ok {
		percentFeeUSD := domain.ToUSD(rec.GrossAmount*cfg.PercentageRate, rec.Currency)
		fixedFeeUSD := domain.ToUSD(cfg.FixedFee, cfg.Currency)
		base.ExpectedFee = percentFeeUSD + fixedFeeUSD
		actualFeeUSD := domain.ToUSD(rec.Fee, rec.Currency)
		feeDiff := math.Abs(actualFeeUSD - base.ExpectedFee)
		if feeDiff > base.ExpectedFee*feeTolerancePct+0.05 {
			base.Status = domain.MatchFeeMismatch
			base.Notes = "fee does not match configured PSP rate"
			return base
		}
	}

	// Check amount match
	if diff > amountToleranceUSD {
		base.Status = domain.MatchAmountMismatch
		base.Notes = "amount differs from expected"
		return base
	}

	if confidence < 1.0 {
		base.Status = domain.MatchFuzzy
		base.Notes = "matched via fuzzy logic"
	} else {
		base.Status = domain.MatchMatched
	}
	return base
}

// fuzzyMatchTolerance is the max amount difference (as fraction of expected) for initial fuzzy search.
const fuzzyMatchTolerance = 0.10 // 10% of expected amount

// fuzzyMatch finds a settlement record matching by amount + timestamp proximity.
// Uses a wide amount tolerance (10%) for the initial search so that amount mismatches
// can still be detected and reported correctly.
func (e *Engine) fuzzyMatch(txn domain.Transaction, records []domain.SettlementRecord, used map[string]bool) (*domain.SettlementRecord, float64) {
	expectedUSD := domain.ToUSD(txn.Amount, txn.Currency)
	var best *domain.SettlementRecord
	bestScore := 0.0

	for i := range records {
		rec := &records[i]
		if used[rec.ID] {
			continue
		}
		if rec.PSPName != txn.PSPName {
			continue
		}

		actualUSD := domain.ToUSD(rec.GrossAmount, rec.Currency)
		maxDiff := expectedUSD * fuzzyMatchTolerance
		if maxDiff < 1.0 {
			maxDiff = 1.0 // minimum $1 tolerance
		}
		if math.Abs(actualUSD-expectedUSD) > maxDiff {
			continue
		}

		score := scoreByTimestamp(txn.CreatedAt, rec.TransactionDate)
		if score > bestScore {
			bestScore = score
			best = rec
		}
	}

	if bestScore >= 0.60 {
		return best, bestScore
	}
	return nil, 0
}

func scoreByTimestamp(expected, actual time.Time) float64 {
	diff := expected.Sub(actual)
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff <= 5*time.Minute:
		return 0.85
	case diff <= 30*time.Minute:
		return 0.75
	case diff <= 2*time.Hour:
		return 0.65
	case diff <= 24*time.Hour:
		return 0.60
	default:
		return 0
	}
}

// normalizeRef strips known PSP prefixes for ID normalization.
func normalizeRef(ref string) string {
	ref = strings.TrimSpace(ref)
	for _, prefix := range []string{"PSPB_", "PSPA_", "PSPC_", "PSP_"} {
		ref = strings.TrimPrefix(ref, prefix)
	}
	return strings.ToUpper(ref)
}
