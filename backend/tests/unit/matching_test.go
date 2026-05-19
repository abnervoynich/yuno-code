package unit_test

import (
	"testing"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTransaction(id, externalID string, amount float64, currency, psp string, t time.Time) domain.Transaction {
	return domain.Transaction{
		ID:         id,
		ExternalID: externalID,
		Amount:     amount,
		Currency:   currency,
		PSPName:    psp,
		Type:       domain.TypeSale,
		Status:     domain.StatusCompleted,
		CreatedAt:  t,
		ExpectedSettleDate: t.Add(24 * time.Hour),
	}
}

func makeRecord(id, pspRef, origRef string, gross, fee, net float64, currency, psp string, txnDate time.Time) domain.SettlementRecord {
	return domain.SettlementRecord{
		ID:                id,
		PSPTransactionRef: pspRef,
		OriginalTxnRef:    origRef,
		GrossAmount:       gross,
		Fee:               fee,
		NetAmount:         net,
		Currency:          currency,
		PSPName:           psp,
		TransactionDate:   txnDate,
		SettlementDate:    txnDate.Add(24 * time.Hour),
		Status:            "settled",
		Type:              "sale",
	}
}

func defaultFeeConfigs() []*domain.PSPFeeConfig {
	return []*domain.PSPFeeConfig{
		{PSPName: "pspa", PercentageRate: 0.029, FixedFee: 0.30, Currency: "USD"},
		{PSPName: "pspb", PercentageRate: 0.025, FixedFee: 0.25, Currency: "AED"},
		{PSPName: "pspc", PercentageRate: 0.031, FixedFee: 0.25, Currency: "USD"},
	}
}

func TestMatchEngine_ExactIDMatch(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	txn := makeTransaction("uuid-1", "TXN-001", 100.00, "USD", "pspa", now)
	rec := makeRecord("rec-1", "TXN-001", "TXN-001", 100.00, 3.20, 96.80, "USD", "pspa", now)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchMatched, result.Matched[0].Status)
	assert.Equal(t, 1.0, result.Matched[0].ConfidenceScore)
	assert.Empty(t, result.Missing)
	assert.Empty(t, result.Unexpected)
}

func TestMatchEngine_PSPBPrefixStripped(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	txn := makeTransaction("uuid-1", "TXN-001", 500.00, "AED", "pspb", now)
	// PSP B prepends "PSPB_"
	rec := makeRecord("rec-1", "PSPB_TXN-001", "TXN-001", 500.00, 12.75, 487.25, "AED", "pspb", now)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchMatched, result.Matched[0].Status)
	assert.Empty(t, result.Missing)
}

func TestMatchEngine_FuzzyMatchByAmountAndTimestamp(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)
	closeTime := now.Add(3 * time.Minute) // within 5-minute window

	// Use USD so currency conversion doesn't interfere with fee check
	// PSP C config: 3.1% + $0.25; for $1000 = $31.25
	txn := makeTransaction("uuid-1", "TXN-PSPC-001", 1000.00, "USD", "pspc", now)
	// PSP C uses its own reference, no original ref; fee matches config exactly
	rec := makeRecord("rec-1", "PSPC-241201-0001", "", 1000.00, 31.25, 968.75, "USD", "pspc", closeTime)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchFuzzy, result.Matched[0].Status)
	assert.InDelta(t, 0.85, result.Matched[0].ConfidenceScore, 0.01)
}

func TestMatchEngine_MissingTransaction(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	txn := makeTransaction("uuid-1", "TXN-MISSING", 450.00, "USD", "pspa", now)
	// No matching settlement record

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{})

	assert.Empty(t, result.Matched)
	require.Len(t, result.Missing, 1)
	assert.Equal(t, domain.MatchMissing, result.Missing[0].Status)
	assert.Equal(t, "uuid-1", result.Missing[0].TransactionID)
}

func TestMatchEngine_UnexpectedRecord(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	rec := makeRecord("rec-ghost", "GHOST-001", "", 750.00, 22.05, 727.95, "USD", "pspa", now)

	result := engine.Match("run-1", []domain.Transaction{}, []domain.SettlementRecord{rec})

	assert.Empty(t, result.Matched)
	assert.Empty(t, result.Missing)
	require.Len(t, result.Unexpected, 1)
	assert.Equal(t, domain.MatchUnexpected, result.Unexpected[0].Status)
}

func TestMatchEngine_AmountMismatch(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	txn := makeTransaction("uuid-1", "TXN-MISMATCH", 150.00, "USD", "pspa", now)
	// PSP reports different amount
	rec := makeRecord("rec-1", "TXN-MISMATCH", "TXN-MISMATCH", 142.50, 4.44, 138.06, "USD", "pspa", now)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchAmountMismatch, result.Matched[0].Status)
	assert.InDelta(t, -7.50, result.Matched[0].AmountDiffUSD, 0.01)
}

func TestMatchEngine_FeeMismatch(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	// PSP A fee config: 2.9% + $0.30; for $150 = $4.65
	// Inflated fee: $9.80 (flagged as fee mismatch)
	txn := makeTransaction("uuid-1", "TXN-FEEERR", 150.00, "USD", "pspa", now)
	rec := makeRecord("rec-1", "TXN-FEEERR", "TXN-FEEERR", 150.00, 9.80, 140.20, "USD", "pspa", now)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchFeeMismatch, result.Matched[0].Status)
}

func TestMatchEngine_CurrencyConversion(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	// AED transaction: 850 AED = ~231.46 USD
	txn := makeTransaction("uuid-1", "TXN-AED", 850.00, "AED", "pspb", now)
	rec := makeRecord("rec-1", "PSPB_TXN-AED", "TXN-AED", 850.00, 21.50, 828.50, "AED", "pspb", now)

	result := engine.Match("run-1", []domain.Transaction{txn}, []domain.SettlementRecord{rec})

	require.Len(t, result.Matched, 1)
	assert.Equal(t, domain.MatchMatched, result.Matched[0].Status)
	// Both sides in AED, diff should be ~0
	assert.InDelta(t, 0.0, result.Matched[0].AmountDiffUSD, 0.01)
}

func TestMatchEngine_MultipleTransactionsMultiplePSPs(t *testing.T) {
	engine := matching.NewEngine(defaultFeeConfigs())
	now := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	txns := []domain.Transaction{
		makeTransaction("t1", "TXN-001", 100.00, "USD", "pspa", now),
		makeTransaction("t2", "TXN-002", 200.00, "AED", "pspb", now.Add(time.Hour)),
		makeTransaction("t3", "TXN-MISSING", 300.00, "USD", "pspa", now.Add(2*time.Hour)),
	}

	records := []domain.SettlementRecord{
		makeRecord("r1", "TXN-001", "TXN-001", 100.00, 3.20, 96.80, "USD", "pspa", now),
		makeRecord("r2", "PSPB_TXN-002", "TXN-002", 200.00, 5.25, 194.75, "AED", "pspb", now.Add(time.Hour)),
		makeRecord("r-ghost", "GHOST-001", "", 999.00, 29.28, 969.72, "USD", "pspa", now.Add(3*time.Hour)),
	}

	result := engine.Match("run-multi", txns, records)

	assert.Len(t, result.Matched, 2)
	assert.Len(t, result.Missing, 1)
	assert.Len(t, result.Unexpected, 1)

	assert.Equal(t, "TXN-MISSING", result.Missing[0].Transaction.ExternalID)
	assert.Equal(t, "GHOST-001", result.Unexpected[0].SettlementRecord.PSPTransactionRef)
}

func TestToUSD(t *testing.T) {
	tests := []struct {
		amount   float64
		currency string
		expected float64
	}{
		{100.00, "USD", 100.00},
		{100.00, "AED", 27.23},
		{100.00, "EUR", 108.12},
		{100.00, "UNKNOWN", 100.00}, // passthrough
	}

	for _, tt := range tests {
		result := domain.ToUSD(tt.amount, tt.currency)
		assert.InDelta(t, tt.expected, result, 0.01, "ToUSD(%v, %s)", tt.amount, tt.currency)
	}
}
