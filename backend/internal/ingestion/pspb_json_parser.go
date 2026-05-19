package ingestion

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/google/uuid"
)

// PSPBJSONParser parses PSP B settlement reports.
// Format: JSON, weekly settlement, prepends "PSPB_" to LuxeCart transaction IDs.
type PSPBJSONParser struct{}

type pspbReport struct {
	Report struct {
		Period struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"period"`
		MerchantID   string         `json:"merchant_id"`
		Settlements  []pspbSettle   `json:"settlements"`
	} `json:"report"`
}

type pspbSettle struct {
	Reference    string `json:"reference"`
	TransactionDetails struct {
		Amount struct {
			Value    float64 `json:"value"`
			Currency string  `json:"currency"`
		} `json:"amount"`
		Fee struct {
			Value    float64 `json:"value"`
			Currency string  `json:"currency"`
		} `json:"fee"`
		Net struct {
			Value    float64 `json:"value"`
			Currency string  `json:"currency"`
		} `json:"net"`
		Type        string `json:"type"`
		ProcessedAt string `json:"processed_at"`
	} `json:"transaction_details"`
	SettlementDetails struct {
		Date   string `json:"date"`
		Status string `json:"status"`
	} `json:"settlement_details"`
}

func (p *PSPBJSONParser) Parse(r io.Reader) ([]domain.SettlementRecord, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var report pspbReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	var records []domain.SettlementRecord
	for i, s := range report.Report.Settlements {
		settleDate, err := parseFlexibleDate(s.SettlementDetails.Date)
		if err != nil {
			return nil, fmt.Errorf("settlement %d: invalid settlement date: %w", i, err)
		}

		txnDate := settleDate
		if s.TransactionDetails.ProcessedAt != "" {
			if d, err := time.Parse(time.RFC3339, s.TransactionDetails.ProcessedAt); err == nil {
				txnDate = d.UTC()
			} else if d, err := parseFlexibleDate(s.TransactionDetails.ProcessedAt); err == nil {
				txnDate = d
			}
		}

		ref := s.Reference
		// PSP B prepends "PSPB_"; derive original ref by stripping it
		origRef := strings.TrimPrefix(ref, "PSPB_")

		recType := strings.ToLower(s.TransactionDetails.Type)
		if recType == "" || recType == "sale" || recType == "purchase" {
			recType = "sale"
		}

		currency := strings.ToUpper(s.TransactionDetails.Amount.Currency)

		records = append(records, domain.SettlementRecord{
			ID:                uuid.NewString(),
			PSPName:           "pspb",
			PSPTransactionRef: ref,
			OriginalTxnRef:    origRef,
			GrossAmount:       s.TransactionDetails.Amount.Value,
			Fee:               s.TransactionDetails.Fee.Value,
			NetAmount:         s.TransactionDetails.Net.Value,
			Currency:          currency,
			SettlementDate:    settleDate,
			TransactionDate:   txnDate,
			Status:            strings.ToLower(s.SettlementDetails.Status),
			Type:              recType,
		})
	}

	return records, nil
}
