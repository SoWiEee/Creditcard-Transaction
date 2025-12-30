package initialize

import (
	"fmt"
	"os"
	"strconv"
)

type Env struct {
	Port string

	DatabaseURL string
	DBHost      string
	DBPort      int
	DBUser      string
	DBPassword  string
	DBName      string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Optional: if true, relax risk rules for load testing
	LoadTest bool
}

func LoadEnv() Env {
	port := getenv("PORT", "3000")

	databaseURL := os.Getenv("DATABASE_URL")

	dbHost := getenv("DB_HOST", "db")
	dbPort := getenvInt("DB_PORT", 5432)
	dbUser := getenv("DB_USER", "cct_user")
	dbPass := getenv("DB_PASSWORD", "cct_pass")
	dbName := getenv("DB_NAME", "creditcard")

	redisAddr := fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT"))
	redisPass := os.Getenv("REDIS_PASSWORD")
	redisDB := getenvInt("REDIS_DB", 0)

	loadTest := getenvBool("LOADTEST", false)

	return Env{
		Port:          port,
		DatabaseURL:   databaseURL,
		DBHost:        dbHost,
		DBPort:        dbPort,
		DBUser:        dbUser,
		DBPassword:    dbPass,
		DBName:        dbName,
		RedisAddr:     redisAddr,
		RedisPassword: redisPass,
		RedisDB:       redisDB,
		LoadTest:      loadTest,
	}
}

func (e Env) PgxConnString() string {
	if e.DatabaseURL != "" {
		return e.DatabaseURL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s", e.DBUser, e.DBPassword, e.DBHost, e.DBPort, e.DBName)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
