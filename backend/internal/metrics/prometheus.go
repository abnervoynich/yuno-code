package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ReconciliationRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reconciliation_runs_total",
		Help: "Total number of reconciliation runs",
	}, []string{"status"})

	ReconciliationMatchRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "reconciliation_match_rate",
		Help: "Match rate percentage for the most recent reconciliation run",
	}, []string{"run_id"})

	ReconciliationDiscrepancyUSD = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "reconciliation_discrepancy_usd",
		Help: "Total discrepancy in USD for a reconciliation run",
	}, []string{"run_id"})

	SettlementRecordsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "settlement_records_ingested_total",
		Help: "Number of settlement records ingested per PSP",
	}, []string{"psp_name", "format"})

	SettlementParseErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "settlement_parse_errors_total",
		Help: "Number of parsing errors per PSP",
	}, []string{"psp_name"})

	TransactionsImported = promauto.NewCounter(prometheus.CounterOpts{
		Name: "transactions_imported_total",
		Help: "Total number of transactions imported",
	})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	MissingTransactions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "missing_transactions_total",
		Help: "Number of transactions missing from settlement per PSP",
	}, []string{"psp_name", "run_id"})
)
