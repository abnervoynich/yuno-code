// seed generates test data for the LuxeCart reconciliation engine.
// It creates expected_transactions.json and the three PSP settlement files,
// then optionally loads them into the database via the API.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

// computeFeeInNative computes the expected fee in the record's native currency
// using the PSP fee config. Percentage is applied to gross in native currency;
// fixed fee is converted from cfg.Currency to native.
func computeFeeInNative(gross float64, currency, configCurrency string, pct, fixedFeeInConfigCurrency float64) float64 {
	percentFee := gross * pct
	// Convert fixed fee from config currency to native currency
	fixedFeeUSD := domain.ToUSD(fixedFeeInConfigCurrency, configCurrency)
	fixedFeeNative := fixedFeeUSD / domain.ExchangeRates[currency]
	return percentFee + fixedFeeNative
}

// Deliberate discrepancies planted in the test data:
// Missing from settlement: these will NOT appear in any PSP report
var missingFromSettlement = map[string]bool{
	"TXN-20241201-00008": true,
	"TXN-20241202-00004": true,
	"TXN-20241203-00007": true,
	"TXN-20241204-00003": true,
	"TXN-20241205-00006": true,
	"TXN-20241206-00002": true,
	"TXN-20241207-00001": true,
}

// Amount mismatches: PSP will report a different (lower) gross amount
var amountMismatches = map[string]float64{
	"TXN-20241201-00005": 142.50,  // expected 150.00
	"TXN-20241202-00009": 830.00,  // expected 850.00 AED
	"TXN-20241203-00012": 2450.00, // expected 2500.00
	"TXN-20241204-00006": 305.00,  // expected 320.00 EUR
	"TXN-20241205-00011": 1150.00, // expected 1200.00 AED
}

// Fee errors: PSP will report an inflated fee (while gross amount is correct)
var feeErrors = map[string]float64{
	"TXN-20241201-00003": 9.80,  // pspa: should be ~$4.65 (2.9%*150+0.30)
	"TXN-20241202-00012": 22.50, // pspb: should be ~AED 12.75 (2.5%*500+0.25)
}

type txnDef struct {
	id       string
	amount   float64
	currency string
	psp      string
	txnType  string
	hour     int
	min      int
}

func main() {
	outDir := "testdata"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	// Generate the 100 expected transactions
	transactions := generateTransactions()

	// Write expected_transactions.json
	if err := writeJSON(filepath.Join(outDir, "expected_transactions.json"), transactions); err != nil {
		fmt.Fprintf(os.Stderr, "write transactions: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d transactions to %s/expected_transactions.json\n", len(transactions), outDir)

	// Build PSP-specific transaction lists
	pspaTxns := filter(transactions, "pspa")
	pspbTxns := filter(transactions, "pspb")
	pspcTxns := filter(transactions, "pspc")

	// Write PSP A CSV
	if err := writePSPACSV(filepath.Join(outDir, "pspa_settlement.csv"), pspaTxns); err != nil {
		fmt.Fprintf(os.Stderr, "write pspa: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote PSP A settlement CSV (%d records)\n", len(pspaTxns))

	// Write PSP B JSON
	if err := writePSPBJSON(filepath.Join(outDir, "pspb_settlement.json"), pspbTxns); err != nil {
		fmt.Fprintf(os.Stderr, "write pspb: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote PSP B settlement JSON (%d records)\n", len(pspbTxns))

	// Write PSP C custom
	if err := writePSPCCustom(filepath.Join(outDir, "pspc_settlement.txt"), pspcTxns); err != nil {
		fmt.Fprintf(os.Stderr, "write pspc: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote PSP C settlement TXT (%d records)\n", len(pspcTxns))

	fmt.Println("\nDiscrepancies planted:")
	fmt.Printf("  Missing from settlement: %d transactions\n", len(missingFromSettlement))
	fmt.Printf("  Amount mismatches: %d transactions\n", len(amountMismatches))
	fmt.Printf("  Fee errors: %d transactions\n", len(feeErrors))
	fmt.Println("  Unexpected in settlement: 3 (GHOST-PSPA-001, GHOST-PSPB-001, GHOST-PSPC-001)")
}

func generateTransactions() []domain.Transaction {
	// Day → list of transaction definitions
	days := []struct {
		date string
		txns []txnDef
	}{
		{"2024-12-01", day1Txns()},
		{"2024-12-02", day2Txns()},
		{"2024-12-03", day3Txns()},
		{"2024-12-04", day4Txns()},
		{"2024-12-05", day5Txns()},
		{"2024-12-06", day6Txns()},
		{"2024-12-07", day7Txns()},
	}

	var transactions []domain.Transaction
	for _, day := range days {
		date, _ := time.Parse("2006-01-02", day.date)
		for _, def := range day.txns {
			createdAt := date.Add(time.Duration(def.hour)*time.Hour + time.Duration(def.min)*time.Minute)
			settleDays := settlementDays(def.psp)
			txn := domain.Transaction{
				ID:                 uuid.NewString(),
				ExternalID:         def.id,
				Amount:             def.amount,
				Currency:           def.currency,
				Status:             "completed",
				Type:               domain.TransactionType(def.txnType),
				PSPName:            def.psp,
				CustomerRef:        fmt.Sprintf("CUST-%s", randomCustomerID()),
				CreatedAt:          createdAt.UTC(),
				ExpectedSettleDate: createdAt.Add(time.Duration(settleDays) * 24 * time.Hour).UTC(),
			}
			transactions = append(transactions, txn)
		}
	}
	return transactions
}

func settlementDays(psp string) int {
	switch psp {
	case "pspa":
		return 1 // T+1
	case "pspb":
		return 5 // T+5 (weekly)
	case "pspc":
		return 3 // T+3
	}
	return 2
}

func filter(txns []domain.Transaction, psp string) []domain.Transaction {
	var result []domain.Transaction
	for _, t := range txns {
		if t.PSPName == psp {
			result = append(result, t)
		}
	}
	return result
}

func writePSPACSV(path string, txns []domain.Transaction) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"transaction_id", "transaction_date", "settlement_date", "gross_amount", "fee_amount", "net_amount", "currency", "status", "type"})

	for _, txn := range txns {
		if missingFromSettlement[txn.ExternalID] {
			continue // deliberately omit
		}

		gross := txn.Amount
		if mismatch, ok := amountMismatches[txn.ExternalID]; ok {
			gross = mismatch
		}

		fee := computeFeeInNative(gross, txn.Currency, "USD", 0.029, 0.30)
		if feeOverride, ok := feeErrors[txn.ExternalID]; ok {
			fee = feeOverride
		}
		net := gross - fee
		settleDate := txn.ExpectedSettleDate

		w.Write([]string{
			txn.ExternalID,
			txn.CreatedAt.Format(time.RFC3339),
			settleDate.Format("2006-01-02"),
			fmt.Sprintf("%.2f", gross),
			fmt.Sprintf("%.2f", fee),
			fmt.Sprintf("%.2f", net),
			txn.Currency,
			"settled",
			string(txn.Type),
		})
	}

	// Add ghost/unexpected record
	w.Write([]string{
		"GHOST-PSPA-001",
		"2024-12-03T11:00:00Z",
		"2024-12-04",
		"750.00",
		"22.05",
		"727.95",
		"USD",
		"settled",
		"sale",
	})

	w.Flush()
	return w.Error()
}

type pspbReport struct {
	Report struct {
		Period struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"period"`
		MerchantID  string         `json:"merchant_id"`
		Settlements []pspbSettle   `json:"settlements"`
	} `json:"report"`
}

type pspbSettle struct {
	Reference          string                 `json:"reference"`
	TransactionDetails pspbTransactionDetails `json:"transaction_details"`
	SettlementDetails  pspbSettlementDetails  `json:"settlement_details"`
}

type pspbTransactionDetails struct {
	Amount      pspbMoney `json:"amount"`
	Fee         pspbMoney `json:"fee"`
	Net         pspbMoney `json:"net"`
	Type        string    `json:"type"`
	ProcessedAt string    `json:"processed_at"`
}

type pspbMoney struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type pspbSettlementDetails struct {
	Date   string `json:"date"`
	Status string `json:"status"`
}

func writePSPBJSON(path string, txns []domain.Transaction) error {
	report := pspbReport{}
	report.Report.MerchantID = "LUXECART_001"
	report.Report.Period.From = "2024-12-01"
	report.Report.Period.To = "2024-12-07"

	for _, txn := range txns {
		if missingFromSettlement[txn.ExternalID] {
			continue
		}

		gross := txn.Amount
		if mismatch, ok := amountMismatches[txn.ExternalID]; ok {
			gross = mismatch
		}

		fee := computeFeeInNative(gross, txn.Currency, "AED", 0.025, 0.25)
		if feeOverride, ok := feeErrors[txn.ExternalID]; ok {
			fee = feeOverride
		}
		net := gross - fee
		ref := "PSPB_" + txn.ExternalID

		settle := pspbSettle{
			Reference: ref,
			TransactionDetails: pspbTransactionDetails{
				Amount:      pspbMoney{Value: gross, Currency: txn.Currency},
				Fee:         pspbMoney{Value: fee, Currency: txn.Currency},
				Net:         pspbMoney{Value: net, Currency: txn.Currency},
				Type:        string(txn.Type),
				ProcessedAt: txn.CreatedAt.Format(time.RFC3339),
			},
			SettlementDetails: pspbSettlementDetails{
				Date:   txn.ExpectedSettleDate.Format("2006-01-02"),
				Status: "COMPLETED",
			},
		}
		report.Report.Settlements = append(report.Report.Settlements, settle)
	}

	// Add ghost record
	report.Report.Settlements = append(report.Report.Settlements, pspbSettle{
		Reference: "PSPB_GHOST-PSPB-001",
		TransactionDetails: pspbTransactionDetails{
			Amount:      pspbMoney{Value: 430.00, Currency: "AED"},
			Fee:         pspbMoney{Value: 11.00, Currency: "AED"},
			Net:         pspbMoney{Value: 419.00, Currency: "AED"},
			Type:        "sale",
			ProcessedAt: "2024-12-04T15:30:00Z",
		},
		SettlementDetails: pspbSettlementDetails{
			Date:   "2024-12-09",
			Status: "COMPLETED",
		},
	})

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writePSPCCustom(path string, txns []domain.Transaction) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "PSPC_REPORT|VERSION:1.0|MERCHANT:LUXECART|DATE:%s\n", time.Now().Format("20060102"))
	fmt.Fprintf(f, "PERIOD|2024-12-01|2024-12-07\n")
	fmt.Fprintf(f, "# Fields: TXN|ref|date_yyyymmdd|time_hhmmss|amount|currency|fee|net|type|status\n")

	counter := 1
	for _, txn := range txns {
		if missingFromSettlement[txn.ExternalID] {
			continue
		}

		gross := txn.Amount
		if mismatch, ok := amountMismatches[txn.ExternalID]; ok {
			gross = mismatch
		}

		fee := computeFeeInNative(gross, txn.Currency, "USD", 0.031, 0.25)
		net := gross - fee

		ref := fmt.Sprintf("PSPC-%s-%04d", txn.CreatedAt.Format("060102"), counter)
		counter++

		fmt.Fprintf(f, "TXN|%s|%s|%s|%.2f|%s|%.2f|%.2f|%s|SETTLED\n",
			ref,
			txn.CreatedAt.Format("20060102"),
			txn.CreatedAt.Format("150405"),
			gross,
			txn.Currency,
			fee,
			net,
			string(txn.Type),
		)
	}

	// Add ghost record
	fmt.Fprintf(f, "TXN|PSPC-GHOST-001|20241205|090000|1899.99|USD|59.13|1840.86|sale|SETTLED\n")

	fmt.Fprintf(f, "SUMMARY|%d\n", counter)
	fmt.Fprintf(f, "END_REPORT\n")
	return nil
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

var rng = rand.New(rand.NewSource(42))

func randomCustomerID() string {
	return fmt.Sprintf("%06d", rng.Intn(999999))
}

// ---- Transaction definitions for each day ----
// Format: id, amount, currency, psp, type, hour, minute
// PSP assignment: position%3==0→pspa, ==1→pspb, ==2→pspc

func day1Txns() []txnDef {
	return []txnDef{
		{"TXN-20241201-00001", 2499.00, "USD", "pspa", "sale", 8, 15},
		{"TXN-20241201-00002", 750.00, "AED", "pspb", "sale", 8, 42},
		{"TXN-20241201-00003", 150.00, "USD", "pspa", "sale", 9, 10}, // fee error
		{"TXN-20241201-00004", 1200.00, "AED", "pspc", "sale", 9, 33},
		{"TXN-20241201-00005", 150.00, "USD", "pspa", "sale", 10, 5},  // amount mismatch
		{"TXN-20241201-00006", 3400.00, "AED", "pspb", "sale", 10, 28},
		{"TXN-20241201-00007", 89.99, "USD", "pspc", "sale", 11, 0},
		{"TXN-20241201-00008", 450.00, "EUR", "pspa", "sale", 11, 45}, // MISSING
		{"TXN-20241201-00009", 275.00, "USD", "pspb", "sale", 12, 12},
		{"TXN-20241201-00010", 4500.00, "USD", "pspc", "sale", 12, 55},
		{"TXN-20241201-00011", 99.00, "AED", "pspa", "sale", 13, 20},
		{"TXN-20241201-00012", 1800.00, "EUR", "pspb", "sale", 14, 5},
		{"TXN-20241201-00013", 65.00, "USD", "pspc", "sale", 14, 40},
		{"TXN-20241201-00014", 320.00, "AED", "pspa", "sale", 15, 15},
		{"TXN-20241201-00015", -89.99, "USD", "pspc", "refund", 15, 50}, // refund
	}
}

func day2Txns() []txnDef {
	return []txnDef{
		{"TXN-20241202-00001", 3200.00, "USD", "pspa", "sale", 9, 5},
		{"TXN-20241202-00002", 480.00, "AED", "pspb", "sale", 9, 30},
		{"TXN-20241202-00003", 125.00, "EUR", "pspc", "sale", 10, 0},
		{"TXN-20241202-00004", 780.00, "USD", "pspa", "sale", 10, 45}, // MISSING
		{"TXN-20241202-00005", 2100.00, "AED", "pspb", "sale", 11, 10},
		{"TXN-20241202-00006", 55.00, "USD", "pspc", "sale", 11, 35},
		{"TXN-20241202-00007", 1450.00, "EUR", "pspa", "sale", 12, 0},
		{"TXN-20241202-00008", 330.00, "AED", "pspb", "sale", 12, 25},
		{"TXN-20241202-00009", 850.00, "AED", "pspc", "sale", 12, 50}, // amount mismatch (pspb handles via fuzzy)
		{"TXN-20241202-00010", 4200.00, "USD", "pspa", "sale", 13, 15},
		{"TXN-20241202-00011", 175.00, "EUR", "pspb", "sale", 13, 40},
		{"TXN-20241202-00012", 500.00, "AED", "pspb", "sale", 14, 5}, // fee error
		{"TXN-20241202-00013", 88.00, "USD", "pspc", "sale", 14, 30},
		{"TXN-20241202-00014", 990.00, "AED", "pspa", "sale", 15, 0},
		{"TXN-20241202-00015", -125.00, "EUR", "pspc", "refund", 15, 25}, // refund
	}
}

func day3Txns() []txnDef {
	return []txnDef{
		{"TXN-20241203-00001", 1750.00, "USD", "pspa", "sale", 8, 30},
		{"TXN-20241203-00002", 630.00, "AED", "pspb", "sale", 9, 0},
		{"TXN-20241203-00003", 44.99, "USD", "pspc", "sale", 9, 25},
		{"TXN-20241203-00004", 2800.00, "EUR", "pspa", "sale", 9, 50},
		{"TXN-20241203-00005", 410.00, "AED", "pspb", "sale", 10, 15},
		{"TXN-20241203-00006", 1100.00, "USD", "pspc", "sale", 10, 40},
		{"TXN-20241203-00007", 255.00, "AED", "pspb", "sale", 11, 5}, // MISSING
		{"TXN-20241203-00008", 3600.00, "USD", "pspa", "sale", 11, 30},
		{"TXN-20241203-00009", 75.00, "EUR", "pspb", "sale", 11, 55},
		{"TXN-20241203-00010", 890.00, "AED", "pspc", "sale", 12, 20},
		{"TXN-20241203-00011", 1250.00, "USD", "pspa", "sale", 12, 45},
		{"TXN-20241203-00012", 2500.00, "USD", "pspc", "sale", 13, 10}, // amount mismatch
		{"TXN-20241203-00013", 380.00, "AED", "pspb", "sale", 13, 35},
		{"TXN-20241203-00014", 145.00, "EUR", "pspa", "sale", 14, 0},
		{"TXN-20241203-00015", -44.99, "USD", "pspc", "refund", 14, 25}, // refund
	}
}

func day4Txns() []txnDef {
	return []txnDef{
		{"TXN-20241204-00001", 920.00, "USD", "pspa", "sale", 8, 10},
		{"TXN-20241204-00002", 1650.00, "AED", "pspb", "sale", 8, 35},
		{"TXN-20241204-00003", 340.00, "EUR", "pspb", "sale", 9, 0}, // MISSING
		{"TXN-20241204-00004", 75.00, "USD", "pspc", "sale", 9, 25},
		{"TXN-20241204-00005", 2300.00, "AED", "pspa", "sale", 9, 50},
		{"TXN-20241204-00006", 320.00, "EUR", "pspa", "sale", 10, 15}, // amount mismatch
		{"TXN-20241204-00007", 485.00, "USD", "pspb", "sale", 10, 40},
		{"TXN-20241204-00008", 1900.00, "AED", "pspc", "sale", 11, 5},
		{"TXN-20241204-00009", 115.00, "EUR", "pspa", "sale", 11, 30},
		{"TXN-20241204-00010", 4400.00, "USD", "pspb", "sale", 11, 55},
		{"TXN-20241204-00011", 220.00, "AED", "pspc", "sale", 12, 20},
		{"TXN-20241204-00012", 1380.00, "USD", "pspa", "sale", 12, 45},
		{"TXN-20241204-00013", 95.00, "EUR", "pspb", "sale", 13, 10},
		{"TXN-20241204-00014", 670.00, "AED", "pspc", "sale", 13, 35},
		{"TXN-20241204-00015", -75.00, "USD", "pspa", "refund", 14, 0}, // refund
	}
}

func day5Txns() []txnDef {
	return []txnDef{
		{"TXN-20241205-00001", 1120.00, "USD", "pspa", "sale", 9, 0},
		{"TXN-20241205-00002", 560.00, "AED", "pspb", "sale", 9, 30},
		{"TXN-20241205-00003", 185.00, "EUR", "pspc", "sale", 10, 0},
		{"TXN-20241205-00004", 3100.00, "USD", "pspa", "sale", 10, 30},
		{"TXN-20241205-00005", 720.00, "AED", "pspb", "sale", 11, 0},
		{"TXN-20241205-00006", 250.00, "USD", "pspc", "sale", 11, 30}, // MISSING
		{"TXN-20241205-00007", 1780.00, "AED", "pspa", "sale", 12, 0},
		{"TXN-20241205-00008", 88.00, "EUR", "pspb", "sale", 12, 30},
		{"TXN-20241205-00009", 3800.00, "USD", "pspc", "sale", 13, 0},
		{"TXN-20241205-00010", 430.00, "AED", "pspa", "sale", 13, 30},
		{"TXN-20241205-00011", 1200.00, "AED", "pspb", "sale", 14, 0}, // amount mismatch
		{"TXN-20241205-00012", 195.00, "EUR", "pspc", "sale", 14, 30},
		{"TXN-20241205-00013", -185.00, "EUR", "pspa", "refund", 15, 0},  // refund
		{"TXN-20241205-00014", 2600.00, "USD", "pspb", "sale", 15, 30},
	}
}

func day6Txns() []txnDef {
	return []txnDef{
		{"TXN-20241206-00001", 1450.00, "USD", "pspa", "sale", 9, 15},
		{"TXN-20241206-00002", 860.00, "AED", "pspc", "sale", 9, 45}, // MISSING
		{"TXN-20241206-00003", 320.00, "EUR", "pspb", "sale", 10, 15},
		{"TXN-20241206-00004", 2750.00, "USD", "pspa", "sale", 10, 45},
		{"TXN-20241206-00005", 490.00, "AED", "pspb", "sale", 11, 15},
		{"TXN-20241206-00006", 130.00, "EUR", "pspc", "sale", 11, 45},
		{"TXN-20241206-00007", 4100.00, "USD", "pspa", "sale", 12, 15},
		{"TXN-20241206-00008", 780.00, "AED", "pspb", "sale", 12, 45},
		{"TXN-20241206-00009", 55.00, "USD", "pspc", "sale", 13, 15},
		{"TXN-20241206-00010", 1600.00, "AED", "pspa", "sale", 13, 45},
		{"TXN-20241206-00011", 245.00, "EUR", "pspb", "sale", 14, 15},
		{"TXN-20241206-00012", 3250.00, "USD", "pspc", "sale", 14, 45},
		{"TXN-20241206-00013", -130.00, "EUR", "pspa", "refund", 15, 15}, // refund
	}
}

func day7Txns() []txnDef {
	return []txnDef{
		{"TXN-20241207-00001", 1850.00, "USD", "pspa", "sale", 8, 0}, // MISSING
		{"TXN-20241207-00002", 640.00, "AED", "pspb", "sale", 8, 30},
		{"TXN-20241207-00003", 275.00, "EUR", "pspc", "sale", 9, 0},
		{"TXN-20241207-00004", 2200.00, "USD", "pspa", "sale", 9, 30},
		{"TXN-20241207-00005", 910.00, "AED", "pspb", "sale", 10, 0},
		{"TXN-20241207-00006", 165.00, "EUR", "pspc", "sale", 10, 30},
		{"TXN-20241207-00007", 3700.00, "USD", "pspa", "sale", 11, 0},
		{"TXN-20241207-00008", 530.00, "AED", "pspb", "sale", 11, 30},
		{"TXN-20241207-00009", 1490.00, "USD", "pspc", "sale", 12, 0},
		{"TXN-20241207-00010", 385.00, "AED", "pspa", "sale", 12, 30},
		{"TXN-20241207-00011", 4000.00, "USD", "pspb", "sale", 13, 0},
		{"TXN-20241207-00012", 210.00, "EUR", "pspc", "sale", 13, 30},
		{"TXN-20241207-00013", -275.00, "EUR", "pspa", "refund", 14, 0}, // refund
	}
}
