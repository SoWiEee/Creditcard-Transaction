package models

import "time"

type User struct {
	UserID        int     `json:"user_id"`
	Username      string  `json:"username"`
	Balance       float64 `json:"balance"`
	CurrentPoints int     `json:"current_points"`
	CreditLimit   float64 `json:"credit_limit"`
}

type Transaction struct {
	TransactionID       int        `json:"transaction_id"`
	UserID              int        `json:"user_id"`
	Amount              float64    `json:"amount"`
	Status              string     `json:"status"`
	PointChange         int        `json:"point_change"`
	Merchant            string     `json:"merchant,omitempty"`
	SourceTransactionID *int       `json:"source_transaction_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
