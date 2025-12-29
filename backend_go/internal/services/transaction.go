package services

import (
	"context"
	"errors"
	"fmt"
	"math"

	"backend_go/internal/models"
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
	TransactionID  int      `json:"transactionId"`
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
	RefundTransactionID int      `json:"refundTransactionId"`
	Logs                []string `json:"logs"`
}

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

func (s *TransactionService) GetUserDetails(ctx context.Context, userID int) (*models.User, error) {
	u, err := models.GetUserByID(ctx, s.Pool, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("User not found")
		}
		return nil, err
	}
	return u, nil
}

func (s *TransactionService) GetTransactionHistory(ctx context.Context, userID int) ([]models.Transaction, error) {
	return models.GetTransactionsByUserID(ctx, s.Pool, userID)
}

func (s *TransactionService) ProcessPayment(ctx context.Context, userID int, amount float64, merchant string, usePoints bool) (*TxResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: PAY at %s, User: %d, Total: $%.2f\n", merchant, userID, amount))

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
				return nil, errors.New("User not found")
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
			return nil, errors.New(fmt.Sprintf("Insufficient credit. New Balance %.2f > Limit %.2f", user.Balance+finalAmount, user.CreditLimit))
		}

		mult := merchantRates[merchant]
		if mult == 0 {
			mult = 1
		}
		pointsEarned := int(math.Floor(finalAmount * mult))
		log.Info(fmt.Sprintf("[Rewards] Merchant: %s (x%g). Points Earned: floor(%.2f)*%g = %d.", merchant, mult, finalAmount, mult, pointsEarned))

		maxID, err := models.GetMaxTransactionID(ctx, tx)
		if err != nil {
			return nil, err
		}
		newTxID := maxID + 1
		netPointChange := pointsEarned - pointsRedeemed

		log.SQL(fmt.Sprintf("INSERT INTO Transactions (...) VALUES (%d, %d, %.2f, 'Paid', %d, '%s');", newTxID, userID, finalAmount, netPointChange, merchant))
		if err := models.CreateTransaction(ctx, tx, newTxID, userID, finalAmount, "Paid", netPointChange, merchant, nil); err != nil {
			return nil, err
		}

		// Points table insert (optional)
		if pointsRedeemed > 0 {
			log.SQL(fmt.Sprintf("INSERT INTO Points (Redeemed: -%d);", pointsRedeemed))
			if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, userID, newTxID, -pointsRedeemed, "Redeemed"); err != nil {
				return nil, err
			}
		}
		if pointsEarned > 0 {
			log.SQL(fmt.Sprintf("INSERT INTO Points (Earned: +%d);", pointsEarned))
			reason := fmt.Sprintf("Earned (%s x%g)", merchant, mult)
			if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, userID, newTxID, pointsEarned, reason); err != nil {
				return nil, err
			}
		}

		log.SQL(fmt.Sprintf("UPDATE Users SET balance = balance + %.2f, current_points = current_points + %d WHERE user_id = %d;", finalAmount, netPointChange, userID))
		if _, err := tx.Exec(ctx, `UPDATE Users SET balance = balance + $1, current_points = current_points + $2 WHERE user_id = $3`, finalAmount, netPointChange, userID); err != nil {
			return nil, err
		}

		log.Info(fmt.Sprintf("Transaction %d completed. Net Points: %d", newTxID, netPointChange))
		return &TxResult{TransactionID: newTxID, FinalAmount: finalAmount, PointsEarned: pointsEarned, PointsRedeemed: pointsRedeemed}, nil
	})
	if err != nil {
		return nil, &TxError{Msg: err.Error(), Logs: logs}
	}
	res := anyRes.(*TxResult)
	res.Logs = logs
	return res, nil
}

type TxError struct {
	Msg  string
	Logs []string
}

func (e *TxError) Error() string { return e.Msg }

func (s *TransactionService) VoidTransaction(ctx context.Context, userID int, targetTxID int) (*VoidResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: VOID, Target Transaction: %d\n", targetTxID))
		t, err := models.GetTransactionByIDForUpdate(ctx, tx, targetTxID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("Transaction not found")
			}
			return nil, err
		}
		if t.UserID != userID {
			return nil, errors.New("Security Alert: Unauthorized access. You do not own this transaction.")
		}
		if t.Status == "Voided" || t.Status == "Refunded" {
			return nil, errors.New(fmt.Sprintf("Cannot void transaction with status: %s", t.Status))
		}

		log.SQL(fmt.Sprintf("UPDATE Transactions SET status='Voided' WHERE transaction_id=%d;", targetTxID))
		if err := models.UpdateTransactionStatus(ctx, tx, targetTxID, "Voided"); err != nil {
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
	})
	if err != nil {
		return nil, &TxError{Msg: err.Error(), Logs: logs}
	}
	res := anyRes.(*VoidResult)
	res.Logs = logs
	return res, nil
}

func (s *TransactionService) RefundTransaction(ctx context.Context, userID int, targetTxID int) (*RefundResult, error) {
	anyRes, logs, err := s.withTransaction(ctx, func(tx pgx.Tx, log *utils.TxLogger) (any, error) {
		log.Raw(fmt.Sprintf("\n> Processing: REFUND, Target Transaction: %d\n", targetTxID))
		t, err := models.GetTransactionByIDForUpdate(ctx, tx, targetTxID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("Transaction not found")
			}
			return nil, err
		}
		if t.UserID != userID {
			return nil, errors.New("Security Alert: Unauthorized access.")
		}
		if t.Status != "Paid" {
			return nil, errors.New(fmt.Sprintf("Cannot refund transaction with status: %s", t.Status))
		}

		// check user points
		u, err := models.GetUserByID(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if u.CurrentPoints < t.PointChange {
			return nil, errors.New("Insufficient points to rollback transaction")
		}

		if err := models.UpdateTransactionStatus(ctx, tx, targetTxID, "Refunded"); err != nil {
			return nil, err
		}
		maxID, err := models.GetMaxTransactionID(ctx, tx)
		if err != nil {
			return nil, err
		}
		refundTxID := maxID + 1
		refundAmount := -t.Amount
		refundPoints := -t.PointChange

		sourceID := targetTxID
		if err := models.CreateTransaction(ctx, tx, refundTxID, userID, refundAmount, "Refunded", refundPoints, t.Merchant, &sourceID); err != nil {
			return nil, err
		}

		if _, err := tx.Exec(ctx, `UPDATE Users SET balance = balance + $1, current_points = current_points + $2 WHERE user_id=$3`, refundAmount, refundPoints, userID); err != nil {
			return nil, err
		}

		if _, err := tx.Exec(ctx, `INSERT INTO Points (user_id, transaction_id, change_amount, reason) VALUES ($1,$2,$3,$4)`, userID, refundTxID, refundPoints, "Refund"); err != nil {
			return nil, err
		}

		return &RefundResult{RefundTransactionID: refundTxID}, nil
	})
	if err != nil {
		return nil, &TxError{Msg: err.Error(), Logs: logs}
	}
	res := anyRes.(*RefundResult)
	res.Logs = logs
	return res, nil
}
