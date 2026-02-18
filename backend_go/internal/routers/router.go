package routers

import (
	"net/http"

	"backend_go/internal/middlewares"

	"github.com/go-chi/chi/v5"
)

type Handlers interface {
	Health(w http.ResponseWriter, r *http.Request)
	GetUserInfo(w http.ResponseWriter, r *http.Request)
	GetUserTransactions(w http.ResponseWriter, r *http.Request)
	Pay(w http.ResponseWriter, r *http.Request)
	VoidTx(w http.ResponseWriter, r *http.Request)
	RefundTx(w http.ResponseWriter, r *http.Request)
}

func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(middlewares.CORS())

	r.Get("/api/health", h.Health)
	r.Get("/api/users/{id}", h.GetUserInfo)
	r.Get("/api/transactions/{user_id}", h.GetUserTransactions)
	r.Post("/api/transactions/pay", h.Pay)
	r.Post("/api/transactions/void", h.VoidTx)
	r.Post("/api/transactions/refund", h.RefundTx)

	return r
}
