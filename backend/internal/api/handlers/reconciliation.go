package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
	"github.com/abnervoynich/yuno-code/backend/internal/metrics"
	"github.com/abnervoynich/yuno-code/backend/internal/reconciliation"
	"github.com/abnervoynich/yuno-code/backend/internal/repository"
	"github.com/go-chi/chi/v5"
)

type ReconciliationHandler struct {
	svc  *reconciliation.Service
	repo *repository.ReconciliationRepo
}

func NewReconciliationHandler(svc *reconciliation.Service, repo *repository.ReconciliationRepo) *ReconciliationHandler {
	return &ReconciliationHandler{svc: svc, repo: repo}
}

func (h *ReconciliationHandler) Routes() func(r chi.Router) {
	return func(r chi.Router) {
		r.Post("/run", h.Run)
		r.Get("/runs", h.ListRuns)
		r.Get("/runs/{id}", h.GetRun)
		r.Get("/runs/{id}/results", h.ListResults)
		r.Get("/runs/{id}/summary", h.Summary)
	}
}

type runRequest struct {
	Name        string `json:"name"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

func (h *ReconciliationHandler) Run(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	start, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "period_start must be YYYY-MM-DD")
		return
	}
	end, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DATE", "period_end must be YYYY-MM-DD")
		return
	}
	end = end.Add(24*time.Hour - time.Second) // inclusive end of day

	run, err := h.svc.Run(r.Context(), reconciliation.RunRequest{
		Name:        req.Name,
		PeriodStart: start,
		PeriodEnd:   end,
	})
	if err != nil {
		metrics.ReconciliationRunsTotal.WithLabelValues("failed").Inc()
		writeError(w, http.StatusInternalServerError, "RECONCILIATION_FAILED", err.Error())
		return
	}

	metrics.ReconciliationRunsTotal.WithLabelValues("completed").Inc()
	metrics.ReconciliationMatchRate.WithLabelValues(run.ID).Set(run.MatchRate)
	metrics.ReconciliationDiscrepancyUSD.WithLabelValues(run.ID).Set(run.Discrepancy)

	writeCreated(w, run)
}

func (h *ReconciliationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.repo.ListRuns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if runs == nil {
		runs = []domain.ReconciliationRun{}
	}
	writeOK(w, runs)
}

func (h *ReconciliationHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run, err := h.repo.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "run not found")
		return
	}
	writeOK(w, run)
}

func (h *ReconciliationHandler) ListResults(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status := r.URL.Query().Get("status")

	results, err := h.repo.ListResults(r.Context(), id, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if results == nil {
		results = []domain.MatchResult{}
	}
	writeOK(w, results)
}

func (h *ReconciliationHandler) Summary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	summary, err := h.svc.Summary(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SUMMARY_ERROR", err.Error())
		return
	}
	writeOK(w, summary)
}
