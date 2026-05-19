package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/api"
	"github.com/abnervoynich/yuno-code/backend/internal/api/handlers"
	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/reconciliation"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test (set TEST_DATABASE_URL=postgres://yuno:yuno@localhost:5432/reconciliation?sslmode=disable)")
	}

	db, err := repository.Open(dsn)
	require.NoError(t, err)

	// Truncate tables for test isolation; psp_fee_configs is left intact (seeded by migrate)
	_, err = db.Exec(`TRUNCATE TABLE match_results, reconciliation_runs, settlement_records, settlement_batches, transactions CASCADE`)
	require.NoError(t, err)

	txnRepo := repository.NewTransactionRepo(db)
	settleRepo := repository.NewSettlementRepo(db)
	recoRepo := repository.NewReconciliationRepo(db)
	recoSvc := reconciliation.NewService(txnRepo, settleRepo, recoRepo)

	txnHandler := handlers.NewTransactionHandler(txnRepo)
	settleHandler := handlers.NewSettlementHandler(settleRepo, 10)
	recoHandler := handlers.NewReconciliationHandler(recoSvc, recoRepo)

	router := api.NewRouter(txnHandler, settleHandler, recoHandler)
	srv := httptest.NewServer(router)

	return srv, func() {
		srv.Close()
		db.Close()
	}
}

func TestHealth(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestImportAndListTransactions(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	txns := []domain.Transaction{
		{
			ExternalID:         "TXN-001",
			Amount:             100.00,
			Currency:           "USD",
			Status:             "completed",
			Type:               "sale",
			PSPName:            "pspa",
			CustomerRef:        "CUST-001",
			CreatedAt:          time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC),
			ExpectedSettleDate: time.Date(2024, 12, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			ExternalID:         "TXN-002",
			Amount:             500.00,
			Currency:           "AED",
			Status:             "completed",
			Type:               "sale",
			PSPName:            "pspb",
			CustomerRef:        "CUST-002",
			CreatedAt:          time.Date(2024, 12, 1, 11, 0, 0, 0, time.UTC),
			ExpectedSettleDate: time.Date(2024, 12, 6, 0, 0, 0, 0, time.UTC),
		},
	}

	body, _ := json.Marshal(txns)
	resp, err := http.Post(srv.URL+"/api/v1/transactions/bulk", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var importResult map[string]int
	json.NewDecoder(resp.Body).Decode(&importResult)
	assert.Equal(t, 2, importResult["imported"])

	// List transactions
	resp2, err := http.Get(srv.URL + "/api/v1/transactions/")
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	var listed []domain.Transaction
	json.NewDecoder(resp2.Body).Decode(&listed)
	assert.Len(t, listed, 2)
}

func TestUploadPSPASettlement(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	csvData := `transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type
TXN-001,2024-12-01T10:00:00Z,2024-12-02,100.00,3.20,96.80,USD,settled,sale
TXN-002,2024-12-01T11:00:00Z,2024-12-02,250.00,7.55,242.45,USD,settled,sale`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("psp_name", "pspa")
	part, _ := writer.CreateFormFile("file", "pspa_settlement.csv")
	part.Write([]byte(csvData))
	writer.Close()

	resp, err := http.Post(srv.URL+"/api/v1/settlements/upload", writer.FormDataContentType(), body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "pspa", result["psp_name"])
	assert.Equal(t, float64(2), result["record_count"])
}

func TestFullReconciliationFlow(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	_ = ctx

	// Step 1: Import transactions
	txns := []domain.Transaction{
		{
			ExternalID:         "TXN-REC-001",
			Amount:             100.00,
			Currency:           "USD",
			Status:             "completed",
			Type:               "sale",
			PSPName:            "pspa",
			CreatedAt:          time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC),
			ExpectedSettleDate: time.Date(2024, 12, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			ExternalID:         "TXN-REC-MISSING",
			Amount:             200.00,
			Currency:           "USD",
			Status:             "completed",
			Type:               "sale",
			PSPName:            "pspa",
			CreatedAt:          time.Date(2024, 12, 1, 11, 0, 0, 0, time.UTC),
			ExpectedSettleDate: time.Date(2024, 12, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	body, _ := json.Marshal(txns)
	resp, err := http.Post(srv.URL+"/api/v1/transactions/bulk", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Step 2: Upload settlement (only TXN-REC-001, TXN-REC-MISSING is absent)
	csvData := fmt.Sprintf(`transaction_id,transaction_date,settlement_date,gross_amount,fee_amount,net_amount,currency,status,type
TXN-REC-001,2024-12-01T10:00:00Z,2024-12-02,100.00,3.20,96.80,USD,settled,sale
GHOST-001,2024-12-01T12:00:00Z,2024-12-02,500.00,14.80,485.20,USD,settled,sale`)

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	writer.WriteField("psp_name", "pspa")
	part, _ := writer.CreateFormFile("file", "pspa_settlement.csv")
	part.Write([]byte(csvData))
	writer.Close()

	resp2, err := http.Post(srv.URL+"/api/v1/settlements/upload", writer.FormDataContentType(), buf)
	require.NoError(t, err)
	resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	// Step 3: Run reconciliation
	runReq := map[string]string{
		"name":         "Test Run",
		"period_start": "2024-12-01",
		"period_end":   "2024-12-01",
	}
	runBody, _ := json.Marshal(runReq)
	resp3, err := http.Post(srv.URL+"/api/v1/reconciliation/run", "application/json", bytes.NewReader(runBody))
	require.NoError(t, err)
	defer resp3.Body.Close()

	assert.Equal(t, http.StatusCreated, resp3.StatusCode)

	var run domain.ReconciliationRun
	json.NewDecoder(resp3.Body).Decode(&run)
	assert.Equal(t, "completed", run.Status)
	assert.Equal(t, 1, run.MatchedCount)
	assert.Equal(t, 1, run.MissingCount)
	assert.Equal(t, 1, run.UnexpectedCount)

	// Step 4: Get summary
	resp4, err := http.Get(srv.URL + "/api/v1/reconciliation/runs/" + run.ID + "/summary")
	require.NoError(t, err)
	defer resp4.Body.Close()

	assert.Equal(t, http.StatusOK, resp4.StatusCode)

	var summary domain.ReconciliationSummary
	json.NewDecoder(resp4.Body).Decode(&summary)
	assert.Len(t, summary.MissingItems, 1)
	assert.Len(t, summary.UnexpectedItems, 1)
	assert.NotEmpty(t, summary.MissingItems[0].TransactionID)
}

func TestReconciliationWithTestDataFiles(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found; run 'go run ./cmd/seed/...' first")
	}

	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Import expected transactions
	txnFile, err := os.ReadFile(filepath.Join(testdataDir, "expected_transactions.json"))
	require.NoError(t, err)

	resp, err := http.Post(srv.URL+"/api/v1/transactions/bulk", "application/json", bytes.NewReader(txnFile))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Upload all 3 PSP files
	files := []struct {
		name string
		psp  string
	}{
		{"pspa_settlement.csv", "pspa"},
		{"pspb_settlement.json", "pspb"},
		{"pspc_settlement.txt", "pspc"},
	}

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(testdataDir, f.name))
		require.NoError(t, err, "read %s", f.name)

		buf := &bytes.Buffer{}
		writer := multipart.NewWriter(buf)
		writer.WriteField("psp_name", f.psp)
		part, _ := writer.CreateFormFile("file", f.name)
		part.Write(data)
		writer.Close()

		resp, err := http.Post(srv.URL+"/api/v1/settlements/upload", writer.FormDataContentType(), buf)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode, "upload %s", f.name)
	}

	// Run reconciliation over the full week
	runReq := map[string]string{
		"name":         "Dec 2024 Full Week",
		"period_start": "2024-12-01",
		"period_end":   "2024-12-07",
	}
	runBody, _ := json.Marshal(runReq)
	resp2, err := http.Post(srv.URL+"/api/v1/reconciliation/run", "application/json", bytes.NewReader(runBody))
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	var run domain.ReconciliationRun
	json.NewDecoder(resp2.Body).Decode(&run)
	assert.Equal(t, "completed", run.Status)
	assert.Equal(t, 100, run.MatchedCount+run.MissingCount+run.MismatchCount+run.FeeErrorCount)

	assert.GreaterOrEqual(t, run.MissingCount, 7, "should find at least 7 missing transactions")
	assert.GreaterOrEqual(t, run.UnexpectedCount, 3, "should find at least 3 unexpected records")
	assert.GreaterOrEqual(t, run.MismatchCount+run.FeeErrorCount, 5, "should find at least 5 amount/fee issues")

	t.Logf("Reconciliation results: matched=%d missing=%d unexpected=%d mismatches=%d fees=%d matchRate=%.1f%%",
		run.MatchedCount, run.MissingCount, run.UnexpectedCount, run.MismatchCount, run.FeeErrorCount, run.MatchRate)
}
