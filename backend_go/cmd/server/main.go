package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend_go/internal/initialize"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	env := initialize.LoadEnv()

	app, err := initialize.Build(ctx, env)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}
	defer app.Close()

	srv := &http.Server{
		Addr:    ":" + env.Port,
		Handler: app.Handler,
	}

	go func() {
		log.Printf("server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShutdown)
	log.Printf("server stopped")
}
