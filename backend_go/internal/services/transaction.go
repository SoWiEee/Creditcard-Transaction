package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"backend_go/internal/models"
	"backend_go/internal/repo"
	"backend_go/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionService struct {
	Pool *pgxpool.Pool
	Risk *RiskEngine
}

var merchantRates = map[string]float64{
	"7-11":        1,
	"Steam":       2,
	"Apple Store": 3,
	"Amazon":      1.5,
}

type TxResult struct {
	TransactionID  int64    `json:"transactionId"`
	FinalAmount    float64  `json:"finalAmount"`
	PointsEarned   int      `json:"pointsEarned"`
	PointsRedeemed int      `json:"pointsRedeemed"`
	Logs           []string `json:"logs"`
}

type VoidResult struct {
	Success        bool     `json:"success"`
	VoidedAmount   float64  `json:"voidedAmount"`
	RestoredPoints int      `json:"restoredPoints"`
	Logs           []string `json:"logs,omitempty"`
}

type RefundResult struct {
	RefundTransactionID int64    `json:"refundTransactionId"`
	Logs                []string `json:"logs"`
}

// ---- New TxError with HTTP + Code ----
type TxError struct {
	HTTP int
	Code string
	Msg  string
	Logs []string
}

func (e *TxError) Error() string { return e.Msg }

func NewTxError(httpStatus int, code, msg string) *TxError {
	return &TxError{HTTP: httpStatus, Code: code, Msg: msg}
}

func asTxError(err error) (*TxError, bool) {
	var te *TxError
	if errors.As(err, &te) {
		return te, true
	}
	return nil, false
}

// ---- Transaction wrapper ----
func (s *TransactionService) withTransaction(ctx context.Context, fn func(tx pgx.Tx, log *utils.TxLogger) (any, error)) (any, []string, error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Release()

	log := utils.NewTxLogger()
	log.SQL("START TRANSACTION;")
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, log.Logs, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	res, err := fn(tx, log)
	if err != nil {
		log.SQL("ROLLBACK; -- Error occurred")
		_ = tx.Rollback(ctx)
		return nil, log.Logs, err
	}
	log.SQL("COMMIT;")
	if err := tx.Commit(ctx); err != nil {
		return nil, log.Logs, err
	}
	return res, log.Logs, nil
}

// ---- Query APIs ----
func (s *TransactionService) GetUserDetails(ctx context.Context, userID int) (*models.User, error) {
	u, err := repo.GetUserByID(ctx, s.Pool, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// handlers.go 目前用字串比對 User not found 做 404，所以保留
			return nil, errors.New("User not found")
		}
		return nil, err
	}
	return u, nil
}

func (s *TransactionService) GetTransactionHistory(ctx context.Context, userID int) ([]models.Transaction, error) {
	return repo.GetTransactionsByUserID(ctx, s.Pool, userID)
}

// ---- PAY ----
func (s *TransactionService) ProcessPayment(ctx context.Context, userID int, amount float64, merchant string, usePoints bool) (*TxResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: PAY at %s, User: %d, Total: $%.2f\n", merchant, userID, amount))

		// Merchant whitelist
		mult, ok := merchantRates[merchant]
		if !ok {
			return nil, NewTxError(http.StatusBadRequest, "INVALID_MERCHANT", "Invalid merchant")
		}

		// Risk
		if err := s.Risk.EvaluatePaymentRisk(ctx, tx, userID, amount, merchant, log); err != nil {
			return nil, err
		}

		// Lock user row
		log.Info(fmt.Sprintf("[PAY] Starting transaction logic for User %d.", userID))
		log.SQL(fmt.Sprintf("SELECT * FROM Users WHERE user_id = %d FOR UPDATE;", userID))

		var user models.User
		row := tx.QueryRow(ctx, `SELECT user_id, username, balance, current_points, credit_limit FROM Users WHERE user_id=$1 FOR UPDATE`, userID)
		if err := row.Scan(&user.UserID, &user.Username, &user.Balance, &user.CurrentPoints, &user.CreditLimit); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, NewTxError(http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			}
			return nil, err
		}

		finalAmount := amount
		pointsRedeemed := 0
		discountAmount := 0.0

		// Points redemption: 100 pts = $1
		if usePoints && user.CurrentPoints >= 100 {
			log.Info(fmt.Sprintf("[Points Redemption] User has %d pts. Calculating discount...", user.CurrentPoints))
			maxDiscount := math.Min(float64(user.CurrentPoints/100), math.Floor(finalAmount))
			if maxDiscount > 0 {
				pointsRedeemed = int(maxDiscount) * 100
				discountAmount = float64(int(maxDiscount))
				finalAmount = finalAmount - discountAmount
				log.Info(fmt.Sprintf("Redeeming %d pts for $%.2f discount.", pointsRedeemed, discountAmount))
			} else {
				log.Info("Points insufficient for minimum $1 discount or amount is too small.")
			}
		} else {
			log.Info("No points redemption applied.")
		}

		log.Info(fmt.Sprintf("Final Payment: $%.2f - $%.2f (Points) = $%.2f (Cash)", amount, discountAmount, finalAmount))

		// Credit limit check
		if (user.Balance + finalAmount) > user.CreditLimit {
			return nil, NewTxError(http.StatusConflict, "INSUFFICIENT_CREDIT", "Insufficient credit")
		}

		pointsEarned := int(math.Floor(finalAmount * mult))
		log.Info(fmt.Sprintf("[Rewards] Merchant: %s (x%g). Points Earned: floor(%.2f)*%g = %d.", merchant, mult, finalAmount, mult, pointsEarned))
		netPointChange := pointsEarned - pointsRedeemed

		// 1. Create Transaction with 'Pending' status
		// We do NOT update user balance or points yet. This happens at settlement.
		log.SQL(fmt.Sprintf(
			"INSERT INTO Transactions (user_id, amount, status, point_change, merchant, source_transaction_id) VALUES (%d, %.2f, 'Pending', %d, '%s', NULL) RETURNING transaction_id;",
			userID, finalAmount, netPointChange, merchant,
		))

		newTxID, err := repo.CreateTransactionReturningID(ctx, tx, userID, finalAmount, "Pending", netPointChange, merchant, nil)
		if err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Transaction %d created (Pending). Settlement in 10s.", newTxID))
		return &TxResult{TransactionID: newTxID, FinalAmount: finalAmount, PointsEarned: pointsEarned, PointsRedeemed: pointsRedeemed}, nil
	})

	if err != nil {
		// preserve TxError (risk/business) with logs
		if te, ok := asTxError(err); ok {
			te.Logs = logs
			return nil, te
		}
		// unknown/internal
		ie := NewTxError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error")
		ie.Logs = logs
		return nil, ie
	}

	res := anyRes.(*TxResult)
	res.Logs = logs

	// Start background settlement
	go s.SettleTransaction(res.TransactionID)

	return res, nil
}

// SettleTransaction waits 10s then finalizes the transaction
func (s *TransactionService) SettleTransaction(txID int64) {
	time.Sleep(10 * time.Second)

	ctx := context.Background()

	// Reuse withTransaction for consistent logging and transaction management
	_, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: SETTLE Transaction: %d\n", txID))

		// 1. Lock Transaction
		t, err := repo.GetTransactionByIDForUpdate(ctx, tx, int(txID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				log.Info("Transaction not found during settlement.")
				return nil, nil
			}
			return nil, err
		}

		if t.Status != "Pending" {
			log.Info(fmt.Sprintf("Transaction %d is '%s', skipping settlement.", txID, t.Status))
			return nil, nil
		}

		// 2. Lock User & Check Limit

		user, err := repo.GetUserByID(ctx, tx, t.UserID)
		if err != nil {
			return nil, err
		}

		if user.Balance+t.Amount > user.CreditLimit {
			log.Info(fmt.Sprintf("Insufficient credit (Bal: %.2f + Amt: %.2f > Lim: %.2f). Voiding.", user.Balance, t.Amount, user.CreditLimit))
			if err := repo.UpdateTransactionStatus(ctx, tx, int(txID), "Voided"); err != nil {
				return nil, err
			}
			return nil, nil // Commit the Voided status
		}

		// 3. Apply Changes
		mult := merchantRates[t.Merchant]
		if mult == 0 {
			mult = 1
		}
		pointsEarned := int(math.Floor(t.Amount * mult))
		pointsRedeemed := pointsEarned - t.PointChange

		log.SQL(fmt.Sprintf("UPDATE Users SET balance += %.2f, points += %d", t.Amount, t.PointChange))
		if _, err := repo.UpdateUserBalanceAndPoints(ctx, tx, t.UserID, t.Amount, t.PointChange); err != nil {
			return nil, err
		}

		// 4. Insert Points Logs
		if pointsRedeemed > 0 {
			log.SQL(fmt.Sprintf("INSERT INTO Points (Redeemed: -%d)", pointsRedeemed))
			if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, t.UserID, t.TransactionID, -pointsRedeemed, "Redeemed"); err != nil {
				return nil, err
			}
		}
		if pointsEarned > 0 {
			reason := fmt.Sprintf("Earned (%s x%g)", t.Merchant, mult)
			log.SQL(fmt.Sprintf("INSERT INTO Points (Earned: +%d)", pointsEarned))
			if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, t.UserID, t.TransactionID, pointsEarned, reason); err != nil {
				return nil, err
			}
		}

		// 5. Update Status
		log.SQL("UPDATE Transactions SET status='Paid'")
		if err := repo.UpdateTransactionStatus(ctx, tx, int(txID), "Paid"); err != nil {
			return nil, err
		}

		log.Info("Settlement successful.")
		return nil, nil
	})

	if err != nil {
		log.Printf("[SETTLE] Error settling tx %d: %v", txID, err)
		for _, l := range logs {
			log.Println(l)
		}
	}
}

// ---- VOID ----
func (s *TransactionService) VoidTransaction(ctx context.Context, userID int, targetTxID int) (*VoidResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: VOID, Target Transaction: %d\n", targetTxID))

		t, err := repo.GetTransactionByIDForUpdate(ctx, tx, targetTxID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, NewTxError(http.StatusNotFound, "TX_NOT_FOUND", "Transaction not found")
			}
			return nil, err
		}
		if t.UserID != userID {
			return nil, NewTxError(http.StatusForbidden, "TX_FORBIDDEN", "Unauthorized access")
		}

		if t.Status == "Pending" {
			// Voiding a pending transaction: just update status, no money movement
			log.Info("Voiding PENDING transaction. No balance/points reverted.")
			if err := repo.UpdateTransactionStatus(ctx, tx, targetTxID, "Voided"); err != nil {
				return nil, err
			}
			return &VoidResult{Success: true, VoidedAmount: 0, RestoredPoints: 0}, nil
		} else if t.Status == "Paid" {
			// Existing logic for Paid
			log.SQL(fmt.Sprintf("UPDATE Transactions SET status='Voided' WHERE transaction_id=%d;", targetTxID))
			if err := repo.UpdateTransactionStatus(ctx, tx, targetTxID, "Voided"); err != nil {
				return nil, err
			}

		log.Info(fmt.Sprintf("Restoring Balance: +$%.2f", t.Amount))
		if _, err := tx.Exec(ctx, `UPDATE Users SET balance = balance + $1 WHERE user_id=$2`, t.Amount, userID); err != nil {
				return nil, err
			}

			reversePointChange := -1 * t.PointChange
			if reversePointChange != 0 {
				log.Info(fmt.Sprintf("Restoring Points: %d", reversePointChange))
				if _, err := tx.Exec(ctx, `UPDATE Users SET current_points = current_points + $1 WHERE user_id=$2`, reversePointChange, userID); err != nil {
					return nil, err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, userID, targetTxID, reversePointChange, "Void Reversal"); err != nil {
					return nil, err
				}
			}
			return &VoidResult{Success: true, VoidedAmount: t.Amount, RestoredPoints: reversePointChange}, nil
		} else {
			return nil, NewTxError(http.StatusConflict, "TX_INVALID_STATUS", fmt.Sprintf("Cannot void transaction with status: %s", t.Status))
		}
	})

	if err != nil {
		if te, ok := asTxError(err); ok {
			te.Logs = logs
			return nil, te
		}
		ie := NewTxError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error")
		ie.Logs = logs
		return nil, ie
	}

	res := anyRes.(*VoidResult)
	res.Logs = logs
	return res, nil
}

// ---- REFUND ----
func (s *TransactionService) RefundTransaction(ctx context.Context, userID int, targetTxID int) (*RefundResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: REFUND, Target Transaction: %d\n", targetTxID))

		t, err := repo.GetTransactionByIDForUpdate(ctx, tx, targetTxID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, NewTxError(http.StatusNotFound, "TX_NOT_FOUND", "Transaction not found")
			}
			return nil, err
		}
		if t.UserID != userID {
			return nil, NewTxError(http.StatusForbidden, "TX_FORBIDDEN", "Unauthorized access")
		}
		if t.Status != "Paid" {
			return nil, NewTxError(http.StatusConflict, "TX_INVALID_STATUS", fmt.Sprintf("Cannot refund transaction with status: %s", t.Status))
		}

		u, err := repo.GetUserByID(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if u.CurrentPoints < t.PointChange {
			return nil, NewTxError(http.StatusConflict, "INSUFFICIENT_POINTS", "Insufficient points to rollback transaction")
		}

		if err := repo.UpdateTransactionStatus(ctx, tx, targetTxID, "Refunded"); err != nil {
			return nil, err
		}

		refundAmount := -t.Amount
		refundPoints := -t.PointChange
		src := int64(targetTxID)

		log.SQL(fmt.Sprintf(
			"INSERT INTO Transactions (user_id, amount, status, point_change, merchant, source_transaction_id) VALUES (%d, %.2f, 'Refunded', %d, '%s', %d) RETURNING transaction_id;",
			userID, refundAmount, refundPoints, t.Merchant, targetTxID,
		))

		refundTxID, err := repo.CreateTransactionReturningID(ctx, tx, userID, refundAmount, "Refunded", refundPoints, t.Merchant, &src)
		if err != nil {
			return nil, err
		}

		if _, err := tx.Exec(ctx,
			`UPDATE Users SET balance = balance + $1, current_points = current_points + $2 WHERE user_id=$3`,
			refundAmount, refundPoints, userID,
		); err != nil {
			return nil, err
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`,
			userID, refundTxID, refundPoints, "Refund",
		); err != nil {
			return nil, err
		}

		return &RefundResult{RefundTransactionID: refundTxID}, nil
	})

	if err != nil {
		if te, ok := asTxError(err); ok {
			te.Logs = logs
			return nil, te
		}
		ie := NewTxError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error")
		ie.Logs = logs
		return nil, ie
	}

	res := anyRes.(*RefundResult)
	res.Logs = logs
	return res, nil
}
