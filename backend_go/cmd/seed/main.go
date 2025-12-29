package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type User struct {
	UserID        int
	Username      string
	Balance       float64
	CurrentPoints int
}

type Transaction struct {
	TransactionID       int
	UserID              int
	Amount              float64
	Status              string
	PointChange         int
	SourceTransactionID sql.NullInt64
	Merchant            string
	CreatedAt           time.Time
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Missing required env: %s", key)
	}
	return v
}

func main() {
	dbURL := mustEnv("DATABASE_URL")

	seedDir := os.Getenv("SEED_DIR")
	if seedDir == "" {
		seedDir = "./db/seed"
	}

	userCSV := filepath.Join(seedDir, "user.csv")
	txCSV := filepath.Join(seedDir, "transaction.csv")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	log.Println("🌱 Starting database seeding...")

	// 1. 清空舊資料
	log.Println("🧹 Cleaning old data...")
	_, err = tx.Exec(`TRUNCATE TABLE Transactions, Users RESTART IDENTITY CASCADE`)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Users
	users, err := loadUsers(userCSV)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("👤 Seeding Users (%d)...", len(users))
	for _, u := range users {
		_, err = tx.Exec(`
			INSERT INTO Users (user_id, username, balance, current_points, credit_limit)
			VALUES ($1,$2,$3,$4,$5)
		`, u.UserID, u.Username, u.Balance, u.CurrentPoints, 10000.0)
		if err != nil {
			log.Fatal(err)
		}
	}

	// 3. Transactions
	txs, err := loadTransactions(txCSV)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("💳 Seeding Transactions (%d)...", len(txs))
	for _, t := range txs {
		_, err = tx.Exec(`
			INSERT INTO Transactions
			(transaction_id, user_id, amount, status, point_change, source_transaction_id, merchant, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`,
			t.TransactionID,
			t.UserID,
			t.Amount,
			t.Status,
			t.PointChange,
			t.SourceTransactionID,
			t.Merchant,
			t.CreatedAt,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err = tx.Commit(); err != nil {
		log.Fatal(err)
	}

	log.Println("🎉 Seeding completed successfully!")
}

/* ---------------- CSV helpers ---------------- */

func loadUsers(path string) ([]User, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var users []User
	for i, row := range rows {
		if i == 0 {
			continue // header
		}

		userID, _ := strconv.Atoi(row[0])
		balance, _ := strconv.ParseFloat(row[2], 64)
		points, _ := strconv.Atoi(row[3])

		users = append(users, User{
			UserID:        userID,
			Username:      row[1],
			Balance:       balance,
			CurrentPoints: points,
		})
	}

	return users, nil
}

func loadTransactions(path string) ([]Transaction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var txs []Transaction
	defaultMerchants := []string{"7-11", "Steam", "Apple Store", "Amazon"}

	for i, row := range rows {
		if i == 0 {
			continue
		}
		// 防呆：空行或欄位不足
		if len(row) < 7 {
			return nil, fmt.Errorf("transaction.csv row %d has %d columns, expected at least 7", i+1, len(row))
		}

		txID, _ := strconv.Atoi(row[0])
		userID, _ := strconv.Atoi(row[1])
		amount, _ := strconv.ParseFloat(row[2], 64)
		status := row[3]
		pointChange, _ := strconv.Atoi(row[4])

		var srcID sql.NullInt64
		if row[5] != "NULL" && row[5] != "" {
			v, _ := strconv.Atoi(row[5])
			srcID = sql.NullInt64{Int64: int64(v), Valid: true}
		}

		createdAt, err := time.Parse(time.RFC3339, row[6])
		if err != nil {
			// 你的 created_at 若不是 RFC3339，可在這裡再加其他格式
			return nil, fmt.Errorf("bad created_at at row %d: %v (value=%q)", i+1, err, row[6])
		}

		merchant := defaultMerchants[(i-1)%len(defaultMerchants)]
		// 如果 CSV 有第 8 欄 merchant，就用它覆蓋預設值
		if len(row) >= 8 && row[7] != "" && row[7] != "NULL" {
			merchant = row[7]
		}

		txs = append(txs, Transaction{
			TransactionID:       txID,
			UserID:              userID,
			Amount:              amount,
			Status:              status,
			PointChange:         pointChange,
			SourceTransactionID: srcID,
			Merchant:            merchant,
			CreatedAt:           createdAt,
		})
	}

	return txs, nil
}
