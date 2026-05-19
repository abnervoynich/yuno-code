package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/ingestion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPSPACSVParser_ValidInput(t *testing.T) {
	csv := `transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type
TXN-20241201-00001,2024-12-01T08:15:00Z,2024-12-02,2499.00,72.57,2426.43,USD,settled,sale
TXN-20241201-00002,2024-12-01T09:10:00Z,2024-12-02,150.00,4.65,145.35,USD,settled,sale`

	parser := &ingestion.PSPACSVParser{}
	records, err := parser.Parse(strings.NewReader(csv))

	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, "pspa", records[0].PSPName)
	assert.Equal(t, "TXN-20241201-00001", records[0].OriginalTxnRef)
	assert.Equal(t, "TXN-20241201-00001", records[0].PSPTransactionRef)
	assert.InDelta(t, 2499.00, records[0].GrossAmount, 0.001)
	assert.InDelta(t, 72.57, records[0].Fee, 0.001)
	assert.Equal(t, "USD", records[0].Currency)
	assert.Equal(t, "sale", records[0].Type)
	assert.Equal(t, "settled", records[0].Status)

	expected, _ := time.Parse("2006-01-02", "2024-12-02")
	assert.Equal(t, expected.UTC(), records[0].SettlementDate.UTC())
}

func TestPSPACSVParser_MissingRequiredColumn(t *testing.T) {
	csv := `transaction_id,gross_amount,currency
TXN-001,100.00,USD`

	parser := &ingestion.PSPACSVParser{}
	_, err := parser.Parse(strings.NewReader(csv))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required column")
}

func TestPSPACSVParser_InvalidAmount(t *testing.T) {
	csv := `transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type
TXN-001,2024-12-01,2024-12-02,not_a_number,4.65,145.35,USD,settled,sale`

	parser := &ingestion.PSPACSVParser{}
	_, err := parser.Parse(strings.NewReader(csv))
	assert.Error(t, err)
}

func TestPSPACSVParser_EmptyInput(t *testing.T) {
	csv := `transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type`
	parser := &ingestion.PSPACSVParser{}
	records, err := parser.Parse(strings.NewReader(csv))
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestPSPBJSONParser_ValidInput(t *testing.T) {
	jsonData := `{
  "report": {
    "period": {"from": "2024-12-01", "to": "2024-12-07"},
    "merchant_id": "LUXECART_001",
    "settlements": [
      {
        "reference": "PSPB_TXN-20241201-00002",
        "transaction_details": {
          "amount": {"value": 750.00, "currency": "AED"},
          "fee": {"value": 18.75, "currency": "AED"},
          "net": {"value": 731.25, "currency": "AED"},
          "type": "SALE",
          "processed_at": "2024-12-01T08:42:00Z"
        },
        "settlement_details": {
          "date": "2024-12-06",
          "status": "COMPLETED"
        }
      }
    ]
  }
}`

	parser := &ingestion.PSPBJSONParser{}
	records, err := parser.Parse(strings.NewReader(jsonData))

	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.Equal(t, "pspb", records[0].PSPName)
	assert.Equal(t, "PSPB_TXN-20241201-00002", records[0].PSPTransactionRef)
	assert.Equal(t, "TXN-20241201-00002", records[0].OriginalTxnRef) // prefix stripped
	assert.InDelta(t, 750.00, records[0].GrossAmount, 0.001)
	assert.Equal(t, "AED", records[0].Currency)
	assert.Equal(t, "sale", records[0].Type)
}

func TestPSPBJSONParser_InvalidJSON(t *testing.T) {
	parser := &ingestion.PSPBJSONParser{}
	_, err := parser.Parse(strings.NewReader("{invalid json}"))
	assert.Error(t, err)
}

func TestPSPCCustomParser_ValidInput(t *testing.T) {
	input := `PSPC_REPORT|VERSION:1.0|MERCHANT:LUXECART|DATE:20241201
PERIOD|2024-12-01|2024-12-03
TXN|PSPC-241201-0001|20241201|102345|1200.00|AED|37.45|1162.55|sale|SETTLED
TXN|PSPC-241201-0002|20241201|143000|89.99|USD|3.04|86.95|sale|SETTLED
SUMMARY|2
END_REPORT`

	parser := &ingestion.PSPCCustomParser{}
	records, err := parser.Parse(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, "pspc", records[0].PSPName)
	assert.Equal(t, "PSPC-241201-0001", records[0].PSPTransactionRef)
	assert.Empty(t, records[0].OriginalTxnRef) // PSP C has no original ref
	assert.InDelta(t, 1200.00, records[0].GrossAmount, 0.001)
	assert.Equal(t, "AED", records[0].Currency)

	// Transaction date should be 2024-12-01 10:23:45
	assert.Equal(t, 10, records[0].TransactionDate.Hour())
	assert.Equal(t, 23, records[0].TransactionDate.Minute())
}

func TestPSPCCustomParser_MalformedTXNLine(t *testing.T) {
	input := `PSPC_REPORT|VERSION:1.0
PERIOD|2024-12-01|2024-12-03
TXN|PSPC-001|20241201|102345|only_5_fields
END_REPORT`

	parser := &ingestion.PSPCCustomParser{}
	_, err := parser.Parse(strings.NewReader(input))
	assert.Error(t, err)
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename    string
		hint        string
		wantPSP     string
		wantFormat  string
		expectError bool
	}{
		{"pspa_settlement.csv", "", "pspa", "csv", false},
		{"pspb_settlement.json", "", "pspb", "json", false},
		{"pspc_settlement.txt", "", "pspc", "custom", false},
		{"report.csv", "pspa", "pspa", "csv", false},
		{"report.json", "pspb", "pspb", "json", false},
		{"unknown.xyz", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			psp, format, err := ingestion.DetectFormat(tt.filename, tt.hint)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPSP, psp)
				assert.Equal(t, tt.wantFormat, format)
			}
		})
	}
}
