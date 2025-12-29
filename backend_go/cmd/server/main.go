package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend_go/internal/config"
	httpapi "backend_go/internal/http"
	"backend_go/internal/services"
)

func main() {
	env := config.LoadEnv()
	ctx := context.Background()

	pg, err := config.NewPGPool(ctx, env.PgxConnString())
	if err != nil {
		log.Fatalf("PostgreSQL connection failed: %v", err)
	}
	defer pg.Close()
	log.Printf("[V] PostgreSQL connected")

	rdb, err := config.NewRedis(ctx, env.RedisAddr, env.RedisPassword, env.RedisDB)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer func() { _ = rdb.Close() }()
	log.Printf("[V] Redis connected")

	risk := &services.RiskEngine{Redis: rdb, Rules: services.DefaultRules(env.LoadTest)}
	svc := &services.TransactionService{Pool: pg, Risk: risk}
	api := &httpapi.API{Svc: svc}

	h := httpapi.NewRouter(api)

	srv := &http.Server{
		Addr:              ":" + env.Port,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShutdown)
	log.Printf("Server stopped")
}
