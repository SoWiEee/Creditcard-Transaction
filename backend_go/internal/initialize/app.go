package initialize

import (
	"context"
	"net/http"

	"backend_go/internal/controller"
	"backend_go/internal/routers"
	service "backend_go/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type App struct {
	Handler http.Handler

	DB    *pgxpool.Pool
	Redis *redis.Client

	Svc *service.TransactionService
}

func Build(ctx context.Context, env Env) (*App, error) {
	pool, err := NewPGPool(ctx, env.DatabaseURL)
	if err != nil {
		return nil, err
	}

	rdb, err := NewRedis(ctx, env.RedisAddr, env.RedisPassword, env.RedisDB)
	if err != nil {
		pool.Close()
		return nil, err
	}

	risk := &service.RiskEngine{
		Redis: rdb,
		Rules: service.DefaultRules(env.LoadTest),
	}

	svc := &service.TransactionService{
		Pool: pool,
		Risk: risk,
	}

	api := &controller.API{Svc: svc}
	h := routers.NewRouter(api)

	return &App{
		Handler: h,
		DB:      pool,
		Redis:   rdb,
		Svc:     svc,
	}, nil
}

// Close releases resources owned by the app.
func (a *App) Close() {
	if a == nil {
		return
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
}
