package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/ingestion"
	"github.com/abnervoynich/yuno-code/backend/internal/metrics"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

type SettlementHandler struct {
	repo      *repository.SettlementRepo
	maxUpload int64
}

func NewSettlementHandler(repo *repository.SettlementRepo, maxUploadMB int64) *SettlementHandler {
	return &SettlementHandler{repo: repo, maxUpload: maxUploadMB * 1024 * 1024}
}

func (h *SettlementHandler) Routes() func(r chi.Router) {
	return func(r chi.Router) {
		r.Post("/upload", h.Upload)
		r.Get("/batches", h.ListBatches)
		r.Get("/batches/{id}", h.GetBatch)
		r.Get("/batches/{id}/records", h.ListRecords)
	}
}

func (h *SettlementHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(h.maxUpload); err != nil {
		writeError(w, http.StatusBadRequest, "FILE_TOO_LARGE", fmt.Sprintf("file exceeds %d MB limit", h.maxUpload/1024/1024))
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "NO_FILE", "file field is required")
		return
	}
	defer file.Close()

	pspNameHint := r.FormValue("psp_name")
	pspName, format, err := ingestion.DetectFormat(fileHeader.Filename, pspNameHint)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UNKNOWN_FORMAT", err.Error())
		return
	}

	parser, err := ingestion.NewParser(pspName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UNKNOWN_PSP", err.Error())
		return
	}

	records, err := parser.Parse(file)
	if err != nil {
		metrics.SettlementParseErrors.WithLabelValues(pspName).Inc()
		writeError(w, http.StatusUnprocessableEntity, "PARSE_ERROR", err.Error())
		return
	}

	if len(records) == 0 {
		writeError(w, http.StatusBadRequest, "EMPTY_FILE", "no records found in settlement file")
		return
	}

	// Compute batch aggregates
	batch := &domain.SettlementBatch{
		PSPName:   pspName,
		Format:    format,
		Filename:  fileHeader.Filename,
		Status:    "completed",
		CreatedAt: time.Now().UTC(),
	}

	var minDate, maxDate time.Time
	for _, rec := range records {
		batch.TotalGross += rec.GrossAmount
		batch.TotalFees += rec.Fee
		batch.TotalNet += rec.NetAmount
		if minDate.IsZero() || rec.TransactionDate.Before(minDate) {
			minDate = rec.TransactionDate
		}
		if maxDate.IsZero() || rec.TransactionDate.After(maxDate) {
			maxDate = rec.TransactionDate
		}
	}
	if len(records) > 0 {
		batch.Currency = records[0].Currency
	}
	batch.PeriodStart = minDate
	batch.PeriodEnd = maxDate
	batch.RecordCount = len(records)

	if err := h.repo.CreateBatch(r.Context(), batch); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Assign batch ID to all records
	for i := range records {
		records[i].BatchID = batch.ID
	}

	if err := h.repo.BulkInsertRecords(r.Context(), records); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	metrics.SettlementRecordsIngested.WithLabelValues(pspName, format).Add(float64(len(records)))

	writeCreated(w, map[string]any{
		"batch_id":     batch.ID,
		"psp_name":     pspName,
		"format":       format,
		"record_count": len(records),
		"period_start": minDate,
		"period_end":   maxDate,
	})
}

func (h *SettlementHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := h.repo.ListBatches(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if batches == nil {
		batches = []domain.SettlementBatch{}
	}
	writeOK(w, batches)
}

func (h *SettlementHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	batch, err := h.repo.GetBatch(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if batch == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "batch not found")
		return
	}
	writeOK(w, batch)
}

func (h *SettlementHandler) ListRecords(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	records, err := h.repo.ListRecords(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if records == nil {
		records = []domain.SettlementRecord{}
	}
	writeOK(w, records)
}
