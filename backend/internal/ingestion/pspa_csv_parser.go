package ingestion

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

// PSPACSVParser parses PSP A settlement reports.
// Format: CSV, daily settlement, uses original LuxeCart transaction IDs.
// Columns: transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type
type PSPACSVParser struct{}

func (p *PSPACSVParser) Parse(r io.Reader) ([]domain.SettlementRecord, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	colIdx := buildColumnIndex(header)
	required := []string{"transaction_id", "gross_amount", "fee_amount", "net_amount", "currency", "settlement_date"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var records []domain.SettlementRecord
	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		lineNum++

		get := func(col string) string {
			if i, ok := colIdx[col]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}

		gross, err := strconv.ParseFloat(get("gross_amount"), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid gross_amount: %w", lineNum, err)
		}
		fee, err := strconv.ParseFloat(get("fee_amount"), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid fee_amount: %w", lineNum, err)
		}
		net, err := strconv.ParseFloat(get("net_amount"), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid net_amount: %w", lineNum, err)
		}

		settleDate, err := parseFlexibleDate(get("settlement_date"))
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid settlement_date: %w", lineNum, err)
		}

		txnDate := settleDate
		if raw := get("transaction_date"); raw != "" {
			if d, err := parseFlexibleDate(raw); err == nil {
				txnDate = d
			}
		}

		txnRef := get("transaction_id")
		recType := strings.ToLower(get("type"))
		if recType == "" {
			recType = "sale"
		}
		status := strings.ToLower(get("status"))
		if status == "" {
			status = "settled"
		}

		records = append(records, domain.SettlementRecord{
			ID:                uuid.NewString(),
			PSPName:           "pspa",
			PSPTransactionRef: txnRef,
			OriginalTxnRef:    txnRef, // PSP A uses the original ID directly
			GrossAmount:       gross,
			Fee:               fee,
			NetAmount:         net,
			Currency:          strings.ToUpper(get("currency")),
			SettlementDate:    settleDate,
			TransactionDate:   txnDate,
			Status:            status,
			Type:              recType,
		})
	}

	return records, nil
}

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(strings.ToLower(col))] = i
	}
	return idx
}

var dateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"01/02/2006",
	"02-01-2006",
}

func parseFlexibleDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, f := range dateFormats {
		if t, err := time.ParseInLocation(f, s, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
