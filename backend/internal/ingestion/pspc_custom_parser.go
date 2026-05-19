package ingestion

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

// PSPCCustomParser parses PSP C settlement reports.
// Format: pipe-delimited custom format, every 3 days.
// PSP C uses its own reference system — transaction IDs cannot be directly matched.
// Matching must be done by amount + transaction timestamp.
//
// File structure:
//   PSPC_REPORT|VERSION:1.0|MERCHANT:{id}|DATE:{date}
//   PERIOD|{from}|{to}
//   TXN|{pspc_ref}|{date_yyyymmdd}|{time_hhmmss}|{amount}|{currency}|{fee}|{net}|{type}|{status}
//   SUMMARY|{count}|{total_gross}|{total_fees}|{total_net}
//   END_REPORT
type PSPCCustomParser struct{}

func (p *PSPCCustomParser) Parse(r io.Reader) ([]domain.SettlementRecord, error) {
	scanner := bufio.NewScanner(r)
	var records []domain.SettlementRecord
	var settleDate string
	lineNum := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) == 0 {
			continue
		}

		switch strings.ToUpper(parts[0]) {
		case "PSPC_REPORT", "HEADER":
			// header line — extract settlement date hint if present
			for _, p := range parts[1:] {
				if strings.HasPrefix(p, "DATE:") {
					settleDate = strings.TrimPrefix(p, "DATE:")
				}
			}

		case "PERIOD":
			// PERIOD|from_date|to_date — use to date as settlement date
			if len(parts) >= 3 {
				settleDate = parts[2]
			}

		case "TXN":
			// TXN|ref|date_yyyymmdd|time_hhmmss|amount|currency|fee|net|type|status
			if len(parts) < 10 {
				return nil, fmt.Errorf("line %d: TXN record has %d fields, need at least 10", lineNum, len(parts))
			}

			ref := strings.TrimSpace(parts[1])
			dateStr := strings.TrimSpace(parts[2])
			timeStr := strings.TrimSpace(parts[3])

			txnDate, err := parseFlexibleDate(fmt.Sprintf("%s %s", formatPSPCDate(dateStr), formatPSPCTime(timeStr)))
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid date/time %q %q: %w", lineNum, dateStr, timeStr, err)
			}

			gross, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid amount: %w", lineNum, err)
			}
			currency := strings.ToUpper(strings.TrimSpace(parts[5]))
			fee, err := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid fee: %w", lineNum, err)
			}
			net, err := strconv.ParseFloat(strings.TrimSpace(parts[7]), 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid net: %w", lineNum, err)
			}

			recType := strings.ToLower(strings.TrimSpace(parts[8]))
			if recType == "purchase" {
				recType = "sale"
			}
			status := strings.ToLower(strings.TrimSpace(parts[9]))

			// Settlement date: use PERIOD end date or fallback to txn date
			sd := txnDate
			if settleDate != "" {
				if d, err := parseFlexibleDate(settleDate); err == nil {
					sd = d
				}
			}

			records = append(records, domain.SettlementRecord{
				ID:                uuid.NewString(),
				PSPName:           "pspc",
				PSPTransactionRef: ref,
				OriginalTxnRef:    "", // PSP C has no mapping to original IDs
				GrossAmount:       gross,
				Fee:               fee,
				NetAmount:         net,
				Currency:          currency,
				SettlementDate:    sd,
				TransactionDate:   txnDate,
				Status:            status,
				Type:              recType,
			})

		case "SUMMARY", "FOOTER", "END_REPORT":
			// skip summary/footer lines
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return records, nil
}

// formatPSPCDate converts YYYYMMDD → 2006-01-02
func formatPSPCDate(s string) string {
	if len(s) == 8 {
		return s[0:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	return s
}

// formatPSPCTime converts HHMMSS → 15:04:05
func formatPSPCTime(s string) string {
	if len(s) == 6 {
		return s[0:2] + ":" + s[2:4] + ":" + s[4:6]
	}
	return s
}
