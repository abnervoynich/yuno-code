package domain

import "time"

type TransactionType string
type TransactionStatus string
type MatchStatus string

const (
	TypeSale       TransactionType = "sale"
	TypeRefund     TransactionType = "refund"
	TypeChargeback TransactionType = "chargeback"

	StatusPending    TransactionStatus = "pending"
	StatusCompleted  TransactionStatus = "completed"
	StatusRefunded   TransactionStatus = "refunded"
	StatusChargeback TransactionStatus = "chargeback"

	MatchMatched          MatchStatus = "matched"
	MatchFuzzy            MatchStatus = "fuzzy_match"
	MatchMissing          MatchStatus = "missing_from_settlement"
	MatchUnexpected       MatchStatus = "unexpected_in_settlement"
	MatchAmountMismatch   MatchStatus = "amount_mismatch"
	MatchFeeMismatch      MatchStatus = "fee_mismatch"
)

// Transaction is a sale or refund recorded in LuxeCart's checkout system.
type Transaction struct {
	ID                 string            `json:"id"`
	ExternalID         string            `json:"external_id"`
	Amount             float64           `json:"amount"`
	Currency           string            `json:"currency"`
	Status             TransactionStatus `json:"status"`
	Type               TransactionType   `json:"type"`
	PSPName            string            `json:"psp_name"`
	CustomerRef        string            `json:"customer_ref"`
	CreatedAt          time.Time         `json:"created_at"`
	ExpectedSettleDate time.Time         `json:"expected_settle_date"`
}

// SettlementBatch is a file uploaded from a PSP.
type SettlementBatch struct {
	ID          string    `json:"id"`
	PSPName     string    `json:"psp_name"`
	Format      string    `json:"format"` // csv, json, custom
	Filename    string    `json:"filename"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	TotalGross  float64   `json:"total_gross"`
	TotalFees   float64   `json:"total_fees"`
	TotalNet    float64   `json:"total_net"`
	Currency    string    `json:"currency"`
	RecordCount int       `json:"record_count"`
	Status      string    `json:"status"` // processing, completed, failed
	CreatedAt   time.Time `json:"created_at"`
}

// SettlementRecord is a single line item from a PSP settlement report.
type SettlementRecord struct {
	ID                string    `json:"id"`
	BatchID           string    `json:"batch_id"`
	PSPName           string    `json:"psp_name"`
	PSPTransactionRef string    `json:"psp_transaction_ref"` // PSP's own reference
	OriginalTxnRef    string    `json:"original_txn_ref"`    // mapped back to LuxeCart ID when possible
	GrossAmount       float64   `json:"gross_amount"`
	Fee               float64   `json:"fee"`
	NetAmount         float64   `json:"net_amount"`
	Currency          string    `json:"currency"`
	SettlementDate    time.Time `json:"settlement_date"`
	TransactionDate   time.Time `json:"transaction_date"`
	Status            string    `json:"status"`
	Type              string    `json:"type"` // sale, refund, chargeback, fee
	CreatedAt         time.Time `json:"created_at"`
}

// ReconciliationRun tracks one execution of reconciliation over a time period.
type ReconciliationRun struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	PeriodStart     time.Time  `json:"period_start"`
	PeriodEnd       time.Time  `json:"period_end"`
	Status          string     `json:"status"` // pending, running, completed, failed
	TotalExpected   float64    `json:"total_expected"`
	TotalSettled    float64    `json:"total_settled"`
	MatchedCount    int        `json:"matched_count"`
	MissingCount    int        `json:"missing_count"`
	UnexpectedCount int        `json:"unexpected_count"`
	MismatchCount   int        `json:"mismatch_count"`
	FeeErrorCount   int        `json:"fee_error_count"`
	MatchRate       float64    `json:"match_rate"`
	Discrepancy     float64    `json:"discrepancy_usd"` // total unreconciled amount in USD
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// MatchResult is the outcome of attempting to match one expected transaction.
type MatchResult struct {
	ID                  string      `json:"id"`
	ReconciliationRunID string      `json:"reconciliation_run_id"`
	TransactionID       string      `json:"transaction_id,omitempty"`
	SettlementRecordID  string      `json:"settlement_record_id,omitempty"`
	Status              MatchStatus `json:"status"`
	ConfidenceScore     float64     `json:"confidence_score"`
	ExpectedAmount      float64     `json:"expected_amount"`
	ActualAmount        float64     `json:"actual_amount"`
	AmountDiffUSD       float64     `json:"amount_diff_usd"`
	ExpectedFee         float64     `json:"expected_fee"`
	ActualFee           float64     `json:"actual_fee"`
	Currency            string      `json:"currency"`
	PSPName             string      `json:"psp_name"`
	Notes               string      `json:"notes,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`

	// Denormalized for display
	Transaction      *Transaction      `json:"transaction,omitempty"`
	SettlementRecord *SettlementRecord `json:"settlement_record,omitempty"`
}

// PSPBreakdown summarises reconciliation results per PSP.
type PSPBreakdown struct {
	PSPName         string  `json:"psp_name"`
	TotalExpected   float64 `json:"total_expected"`
	TotalSettled    float64 `json:"total_settled"`
	MatchedCount    int     `json:"matched_count"`
	MissingCount    int     `json:"missing_count"`
	UnexpectedCount int     `json:"unexpected_count"`
	MismatchCount   int     `json:"mismatch_count"`
	MatchRate       float64 `json:"match_rate"`
}

// ReconciliationSummary is the human-readable output for a completed run.
type ReconciliationSummary struct {
	Run               ReconciliationRun `json:"run"`
	PSPBreakdowns     []PSPBreakdown    `json:"psp_breakdowns"`
	TopDiscrepancies  []MatchResult     `json:"top_discrepancies"` // largest by AmountDiffUSD
	MissingItems      []MatchResult     `json:"missing_items"`
	UnexpectedItems   []MatchResult     `json:"unexpected_items"`
	AmountMismatches  []MatchResult     `json:"amount_mismatches"`
	FeeErrors         []MatchResult     `json:"fee_errors"`
}

// PSPFeeConfig defines expected fee rates per PSP.
type PSPFeeConfig struct {
	PSPName        string  `json:"psp_name"`
	PercentageRate float64 `json:"percentage_rate"` // e.g. 0.029 for 2.9%
	FixedFee       float64 `json:"fixed_fee"`       // in the PSP's primary currency
	Currency       string  `json:"currency"`
}
