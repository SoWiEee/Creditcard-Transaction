package models

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Querier interface {
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
}

// insert 由 DB identity 產生 transaction_id，並用 RETURNING 拿回來
func CreateTransactionReturningID(
	ctx context.Context,
	q Querier,
	userID int,
	amount float64,
	status string,
	pointChange int,
	merchant string,
	sourceID *int64,
) (int64, error) {
	var newID int64
	err := q.QueryRow(ctx, `
		INSERT INTO Transactions (user_id, amount, status, point_change, merchant, source_transaction_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING transaction_id
	`, userID, amount, status, pointChange, merchant, sourceID).Scan(&newID)
	return newID, err
}

func CreateTransaction(ctx context.Context, q Querier, txID, userID int, amount float64, status string, pointChange int, merchant string, sourceID *int) error {
	_, err := q.Exec(ctx, `INSERT INTO Transactions (transaction_id, user_id, amount, status, point_change, merchant, source_transaction_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`, txID, userID, amount, status, pointChange, merchant, sourceID)
	return err
}

func GetTransactionByIDForUpdate(ctx context.Context, q Querier, txID int) (*Transaction, error) {
	row := q.QueryRow(ctx, `SELECT transaction_id, user_id, amount, status, point_change, merchant, source_transaction_id, created_at FROM Transactions WHERE transaction_id=$1 FOR UPDATE`, txID)
	var t Transaction
	var source sql.NullInt64
	if err := row.Scan(&t.TransactionID, &t.UserID, &t.Amount, &t.Status, &t.PointChange, &t.Merchant, &source, &t.CreatedAt); err != nil {
		return nil, err
	}
	if source.Valid {
		v := int(source.Int64)
		t.SourceTransactionID = &v
	}
	return &t, nil
}

func GetTransactionsByUserID(ctx context.Context, q Querier, userID int) ([]Transaction, error) {
	rows, err := q.Query(ctx, `SELECT transaction_id, user_id, amount, status, point_change, merchant, source_transaction_id, created_at FROM Transactions WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Transaction, 0)
	for rows.Next() {
		var t Transaction
		var source sql.NullInt64
		if err := rows.Scan(&t.TransactionID, &t.UserID, &t.Amount, &t.Status, &t.PointChange, &t.Merchant, &source, &t.CreatedAt); err != nil {
			return nil, err
		}
		if source.Valid {
			v := int(source.Int64)
			t.SourceTransactionID = &v
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func UpdateTransactionStatus(ctx context.Context, q Querier, txID int, newStatus string) error {
	_, err := q.Exec(ctx, `UPDATE Transactions SET status=$1 WHERE transaction_id=$2`, newStatus, txID)
	return err
}

func GetMaxTransactionID(ctx context.Context, q Querier) (int, error) {
	row := q.QueryRow(ctx, `SELECT COALESCE(MAX(transaction_id), 0) FROM Transactions`)
	var max int
	if err := row.Scan(&max); err != nil {
		return 0, err
	}
	return max, nil
}
