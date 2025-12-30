package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"backend_go/internal/services"
	"backend_go/internal/utils"

	"github.com/go-chi/chi/v5"
)

type API struct {
	Svc *services.TransactionService
}

type healthResp struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, 200, healthResp{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339Nano)})
}

func (a *API) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		utils.WriteJSON(w, 400, utils.APIError{Code: "INVALID_USER_ID", Error: "Invalid user ID format"})
		return
	}
	ctx := r.Context()
	u, err := a.Svc.GetUserDetails(ctx, id)
	if err != nil {
		if err.Error() == "User not found" {
			utils.WriteJSON(w, 404, utils.APIError{Code: "USER_NOT_FOUND", Error: "User not found"})
			return
		}
		utils.WriteJSON(w, 500, utils.APIError{Code: "INTERNAL_ERROR", Error: "Internal Server Error"})
		return
	}
	utils.WriteJSON(w, 200, u)
}

func (a *API) GetUserTransactions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "user_id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		utils.WriteJSON(w, 400, utils.APIError{Code: "INVALID_USER_ID", Error: "Invalid user ID format"})
		return
	}
	txs, err := a.Svc.GetTransactionHistory(r.Context(), id)
	if err != nil {
		utils.WriteJSON(w, 500, utils.APIError{Code: "INTERNAL_ERROR", Error: "Internal Server Error"})
		return
	}
	utils.WriteJSON(w, 200, txs)
}

type payReq struct {
	UserID    int     `json:"user_id"`
	Amount    float64 `json:"amount"`
	Merchant  string  `json:"merchant"`
	UsePoints bool    `json:"use_points"`
}

type actionReq struct {
	UserID              int `json:"user_id"`
	TargetTransactionID int `json:"target_transaction_id"`
}

func writeTxError(w http.ResponseWriter, te *services.TxError) {
	status := te.HTTP
	if status == 0 {
		status = 400
	}
	code := te.Code
	if code == "" {
		code = "TX_ERROR"
	}
	utils.WriteJSON(w, status, utils.APIError{
		Code:  code,
		Error: te.Msg,
		Logs:  te.Logs,
	})
}

func (a *API) Pay(w http.ResponseWriter, r *http.Request) {
	var req payReq
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteJSON(w, 400, utils.APIError{Code: "BAD_JSON", Error: "Invalid JSON body"})
		return
	}
	if req.UserID <= 0 || req.Amount <= 0 || req.Merchant == "" {
		utils.WriteJSON(w, 400, utils.APIError{Code: "VALIDATION_FAILED", Error: "Validation failed"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	res, err := a.Svc.ProcessPayment(ctx, req.UserID, req.Amount, req.Merchant, req.UsePoints)
	if err != nil {
		if te, ok := err.(*services.TxError); ok {
			writeTxError(w, te)
			return
		}
		utils.WriteJSON(w, 500, utils.APIError{Code: "INTERNAL_ERROR", Error: "Internal Server Error"})
		return
	}
	utils.WriteJSON(w, 201, res)
}

func (a *API) VoidTx(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteJSON(w, 400, utils.APIError{Code: "BAD_JSON", Error: "Invalid JSON body"})
		return
	}
	if req.UserID <= 0 || req.TargetTransactionID <= 0 {
		utils.WriteJSON(w, 400, utils.APIError{Code: "VALIDATION_FAILED", Error: "Validation failed"})
		return
	}
	res, err := a.Svc.VoidTransaction(r.Context(), req.UserID, req.TargetTransactionID)
	if err != nil {
		if te, ok := err.(*services.TxError); ok {
			writeTxError(w, te)
			return
		}
		utils.WriteJSON(w, 500, utils.APIError{Code: "INTERNAL_ERROR", Error: "Internal Server Error"})
		return
	}
	utils.WriteJSON(w, 200, res)
}

func (a *API) RefundTx(w http.ResponseWriter, r *http.Request) {
	var req actionReq
	if err := utils.ReadJSON(r, &req); err != nil {
		utils.WriteJSON(w, 400, utils.APIError{Code: "BAD_JSON", Error: "Invalid JSON body"})
		return
	}
	if req.UserID <= 0 || req.TargetTransactionID <= 0 {
		utils.WriteJSON(w, 400, utils.APIError{Code: "VALIDATION_FAILED", Error: "Validation failed"})
		return
	}
	res, err := a.Svc.RefundTransaction(r.Context(), req.UserID, req.TargetTransactionID)
	if err != nil {
		if te, ok := err.(*services.TxError); ok {
			writeTxError(w, te)
			return
		}
		utils.WriteJSON(w, 500, utils.APIError{Code: "INTERNAL_ERROR", Error: "Internal Server Error"})
		return
	}
	utils.WriteJSON(w, 200, res)
}
