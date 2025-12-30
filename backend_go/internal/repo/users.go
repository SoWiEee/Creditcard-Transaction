package repo

import (
	"context"

	"backend_go/internal/models"
)

func GetUserByID(ctx context.Context, q Querier, userID int) (*models.User, error) {
	row := q.QueryRow(ctx, `SELECT user_id, username, balance, current_points, credit_limit FROM Users WHERE user_id=$1`, userID)
	var u models.User
	if err := row.Scan(&u.UserID, &u.Username, &u.Balance, &u.CurrentPoints, &u.CreditLimit); err != nil {
		return nil, err
	}
	return &u, nil
}

func UpdateUserBalanceAndPoints(ctx context.Context, q Querier, userID int, balanceChange float64, pointChange int) (*models.User, error) {
	row := q.QueryRow(ctx, `UPDATE Users SET balance = balance + $1, current_points = current_points + $2 WHERE user_id = $3 RETURNING user_id, username, balance, current_points, credit_limit`, balanceChange, pointChange, userID)
	var u models.User
	if err := row.Scan(&u.UserID, &u.Username, &u.Balance, &u.CurrentPoints, &u.CreditLimit); err != nil {
		return nil, err
	}
	return &u, nil
}
