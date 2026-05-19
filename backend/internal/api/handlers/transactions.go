package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/metrics"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

type TransactionHandler struct {
	repo *repository.TransactionRepo
}

func NewTransactionHandler(repo *repository.TransactionRepo) *TransactionHandler {
	return &TransactionHandler{repo: repo}
}

func (h *TransactionHandler) Routes() func(r chi.Router) {
	return func(r chi.Router) {
		r.Post("/bulk", h.BulkImport)
		r.Get("/", h.List)
		r.Get("/{id}", h.GetByID)
	}
}

func (h *TransactionHandler) BulkImport(w http.ResponseWriter, r *http.Request) {
	var txns []domain.Transaction
	if err := json.NewDecoder(r.Body).Decode(&txns); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if len(txns) == 0 {
		writeError(w, http.StatusBadRequest, "EMPTY_PAYLOAD", "no transactions provided")
		return
	}

	if err := h.repo.BulkInsert(r.Context(), txns); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	metrics.TransactionsImported.Add(float64(len(txns)))
	writeCreated(w, map[string]int{"imported": len(txns)})
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	psp := r.URL.Query().Get("psp")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to time.Time
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24 * time.Hour)
		}
	}

	txns, err := h.repo.List(r.Context(), psp, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if txns == nil {
		txns = []domain.Transaction{}
	}
	writeOK(w, txns)
}

func (h *TransactionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	txn, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if txn == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "transaction not found")
		return
	}
	writeOK(w, txn)
}
