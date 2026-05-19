package api

import (
	"encoding/json"
	"net/http"

	"github.com/abnervoynich/yuno-code/backend/internal/api/handlers"
	"github.com/abnervoynich/yuno-code/backend/internal/api/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router struct {
	txnHandler    *handlers.TransactionHandler
	settleHandler *handlers.SettlementHandler
	recoHandler   *handlers.ReconciliationHandler
}

func NewRouter(
	txnHandler *handlers.TransactionHandler,
	settleHandler *handlers.SettlementHandler,
	recoHandler *handlers.ReconciliationHandler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS)
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.Metrics)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/transactions", txnHandler.Routes())
		r.Route("/settlements", settleHandler.Routes())
		r.Route("/reconciliation", recoHandler.Routes())
	})

	return r
}
