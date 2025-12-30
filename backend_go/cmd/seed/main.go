package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	_, err = tx.Exec(`TRUNCATE TABLE Points, Transactions, Users CASCADE`)
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

	log.Println("🔧 Aligning Transactions identity sequence...")
	_, err = tx.Exec(`
		SELECT setval(
			pg_get_serial_sequence('transactions','transaction_id'),
			COALESCE((SELECT MAX(transaction_id) FROM transactions), 0),
			true
		);
	`)
	if err != nil {
		log.Fatal(err)
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

		createdAt, err := parseCreatedAt(row[6])
		if err != nil {
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

var oneDigitHour = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) (\d):(\d{2}:\d{2})$`)

func parseCreatedAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "NULL" {
		return time.Time{}, fmt.Errorf("empty created_at")
	}

	// 1) 先試 RFC3339（萬一未來你換格式）
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// 2) 你目前 CSV 的主要格式：YYYY-MM-DD HH:MM:SS
	//    但 Go 會要求 HH 必須是兩位數
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
		return t, nil
	}

	// 3) 處理單位數小時：2023-01-03 9:15:00 -> 2023-01-03 09:15:00
	if m := oneDigitHour.FindStringSubmatch(s); m != nil {
		fixed := fmt.Sprintf("%s 0%s:%s", m[1], m[2], m[3])
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", fixed, time.UTC); err == nil {
			return t, nil
		}
	}

	// 4) 如果還是失敗，就回報原字串方便你抓錯
	return time.Time{}, fmt.Errorf("unsupported created_at format: %q", s)
}
