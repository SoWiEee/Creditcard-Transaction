package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"backend_go/internal/repo"
	"backend_go/internal/utils"

	"github.com/redis/go-redis/v9"
)

type RiskRules struct {
	MaxAmount          float64
	MinAmount          float64
	VelocityLimit      int64
	VelocityWindow     time.Duration
	DuplicateWindowSQL string
	RefundLimit        int64
	RefundWindowSQL    string
}

func DefaultRules(loadtest bool) RiskRules {
	if loadtest {
		return RiskRules{
			MaxAmount:          10000,
			MinAmount:          1,
			VelocityLimit:      1 << 60,
			VelocityWindow:     time.Second,
			DuplicateWindowSQL: "1 second",
			RefundLimit:        1 << 60,
			RefundWindowSQL:    "24 hours",
		}
	}
	return RiskRules{
		MaxAmount:          10000,
		MinAmount:          1,
		VelocityLimit:      3,
		VelocityWindow:     60 * time.Second,
		DuplicateWindowSQL: "5 minutes",
		RefundLimit:        3,
		RefundWindowSQL:    "24 hours",
	}
}

type RiskEngine struct {
	Redis *redis.Client
	Rules RiskRules
}

func (r *RiskEngine) EvaluatePaymentRisk(ctx context.Context, q repo.Querier, userID int, amount float64, merchant string, log *utils.TxLogger) error {
	log.Info(fmt.Sprintf("[RISK] Starting Risk Evaluation for User %d...", userID))

	// Amount bounds
	if amount > r.Rules.MaxAmount {
		log.Info(fmt.Sprintf("[RISK] FAIL: Amount $%.2f exceeds limit $%.2f.", amount, r.Rules.MaxAmount))
		return NewTxError(http.StatusBadRequest, "RISK_AMOUNT_TOO_HIGH", "Transaction amount exceeds maximum limit")
	}
	if amount < r.Rules.MinAmount {
		log.Info(fmt.Sprintf("[RISK] FAIL: Amount $%.2f is below minimum $%.2f.", amount, r.Rules.MinAmount))
		return NewTxError(http.StatusBadRequest, "RISK_AMOUNT_TOO_LOW", "Transaction amount is too low")
	}
	log.Info("[RISK] PASS: Amount limits check.")

	// Velocity check via Redis
	velocityKey := fmt.Sprintf("risk:velocity:user:%d", userID)
	count, err := r.Redis.Incr(ctx, velocityKey).Result()
	if err != nil {
		log.Info(fmt.Sprintf("[RISK] ERROR: redis incr failed: %v", err))
		return NewTxError(http.StatusServiceUnavailable, "REDIS_UNAVAILABLE", "Risk system temporarily unavailable")
	}
	if count == 1 {
		if err := r.Redis.Expire(ctx, velocityKey, r.Rules.VelocityWindow).Err(); err != nil {
			log.Info(fmt.Sprintf("[RISK] WARN: redis expire failed: %v", err))
		}
	}
	if count > r.Rules.VelocityLimit {
		log.Info(fmt.Sprintf("[RISK] FAIL: Velocity limit reached (Redis: %d tx in window).", count))
		return NewTxError(http.StatusTooManyRequests, "RISK_VELOCITY_LIMIT", "Too many transactions in short period")
	}
	log.Info(fmt.Sprintf("[RISK] PASS: Velocity check (Redis: %d/%d).", count, r.Rules.VelocityLimit))

	// Refund abuse (DB)
	var refundCount int64
	refundSQL := fmt.Sprintf(
		`SELECT COUNT(*) FROM Transactions WHERE user_id = $1 AND status = 'Refunded' AND created_at > NOW() - INTERVAL '%s'`,
		r.Rules.RefundWindowSQL,
	)
	if err := q.QueryRow(ctx, refundSQL, userID).Scan(&refundCount); err != nil {
		log.Info(fmt.Sprintf("[RISK] ERROR: refund count query failed: %v", err))
		return NewTxError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error")
	}
	if refundCount >= r.Rules.RefundLimit {
		log.Info(fmt.Sprintf("[RISK] FAIL: User has %d refunds in 24h. Account temporarily frozen.", refundCount))
		return NewTxError(http.StatusForbidden, "RISK_REFUND_ABUSE", "Account temporarily frozen due to excessive refunds")
	}
	log.Info(fmt.Sprintf("[RISK] PASS: Refund history check (%d refunds in 24h).", refundCount))

	// Duplicate transaction check (DB)
	var dupCount int64
	dupSQL := fmt.Sprintf(
		`SELECT COUNT(*) FROM Transactions WHERE user_id = $1 AND merchant = $2 AND amount = $3 AND created_at > NOW() - INTERVAL '%s'`,
		r.Rules.DuplicateWindowSQL,
	)
	if err := q.QueryRow(ctx, dupSQL, userID, merchant, amount).Scan(&dupCount); err != nil {
		log.Info(fmt.Sprintf("[RISK] ERROR: duplicate count query failed: %v", err))
		return NewTxError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error")
	}
	if dupCount > 0 {
		log.Info("[RISK] FAIL: Duplicate transaction detected.")
		return NewTxError(http.StatusConflict, "RISK_DUPLICATE", "Potential duplicate transaction detected")
	}

	log.Info("[RISK] PASS: Duplicate transaction check.")
	log.Info("[RISK] [V] All Risk Checks Passed.")
	return nil
}
