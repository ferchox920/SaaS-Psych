package main

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sessionflow/apps/api/internal/config"
	"sessionflow/apps/api/internal/http"
	httphandlers "sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/infra/db"
	redisinfra "sessionflow/apps/api/internal/infra/redis"
	"sessionflow/apps/api/internal/observability"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
	sessionnoteusecase "sessionflow/apps/api/internal/usecase/sessionnote"
	tenantusecase "sessionflow/apps/api/internal/usecase/tenant"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx, cfg, logger)
	cancel()
	if err != nil {
		logger.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	serverDeps := http.ServerDeps{
		RequestLoggingMiddleware: httpmiddleware.RequestLogging(logger),
	}
	var redisCloser func() error
	var tracerShutdown func(context.Context) error

	tracerProvider, shutdown, err := observability.NewTracerProvider(ctx, cfg)
	if err != nil {
		return err
	}
	serverDeps.RequestTracingMiddleware = httpmiddleware.RequestTracing(tracerProvider.Tracer("sessionflow/http"))
	tracerShutdown = shutdown

	registry := prometheus.NewRegistry()
	httpMetrics, err := observability.NewHTTPMetrics(registry)
	if err != nil {
		return err
	}
	serverDeps.RequestMetricsMiddleware = httpMetrics.Middleware()
	serverDeps.MetricsHandler = echo.WrapHandler(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	var poolCloser func()
	if cfg.DatabaseURL != "" {
		pool, err := db.NewPostgresPoolWithTracing(ctx, cfg.DatabaseURL, db.PoolTracingConfig{
			Tracer:             tracerProvider.Tracer("sessionflow/db"),
			DBStatementEnabled: cfg.OTELDBStatement,
		})
		if err != nil {
			return err
		}

		tenantRepo := db.NewTenantRepository(pool)
		tenantService := tenantusecase.NewService(tenantRepo)
		serverDeps.TenantMiddleware = httpmiddleware.RequireTenant(tenantService)

		authRepo := db.NewAuthRepository(pool)
		auditRepo := db.NewAuditRepository(pool)
		clientRepo := db.NewClientRepository(pool)
		appointmentRepo := db.NewAppointmentRepository(pool)
		sessionNoteRepo := db.NewSessionNoteRepository(pool)
		auditService := auditusecase.NewService(auditRepo)
		clientService := clientusecase.NewService(clientRepo, auditRepo)
		appointmentService := appointmentusecase.NewService(appointmentRepo, auditRepo)
		sessionNoteService := sessionnoteusecase.NewService(sessionNoteRepo, auditRepo)
		tokenService := authusecase.NewTokenService(cfg.JWTAccessSecret, cfg.AccessTTL())
		authService := authusecase.NewService(authRepo, tokenService, cfg.RefreshTTL(), auditRepo)
		serverDeps.AuthHandler = httphandlers.NewAuthHandler(authService)
		serverDeps.AuditHandler = httphandlers.NewAuditHandler(auditService)
		serverDeps.ClientHandler = httphandlers.NewClientHandler(clientService)
		serverDeps.AppointmentHandler = httphandlers.NewAppointmentHandler(appointmentService)
		serverDeps.SessionNoteHandler = httphandlers.NewSessionNoteHandler(sessionNoteService)
		serverDeps.AuthMiddleware = httpmiddleware.RequireAuth(cfg.JWTAccessSecret)

		poolCloser = pool.Close
		logger.Info("database connection established")
	} else {
		logger.Warn("DATABASE_URL is empty, auth endpoints will be disabled")
	}

	if cfg.RedisURL != "" && cfg.RateLimitLoginPerMin > 0 {
		redisClient, err := redisinfra.NewClient(ctx, cfg.RedisURL)
		if err != nil {
			return err
		}

		store := redisinfra.NewLoginRateLimitStoreWithTracer(redisClient, tracerProvider.Tracer("sessionflow/redis"))
		serverDeps.AuthLoginRateLimit = httpmiddleware.RequireLoginRateLimit(store, cfg.RateLimitLoginPerMin, time.Minute)
		redisCloser = redisClient.Close
		logger.Info("redis connection established", slog.Int("rate_limit_login_per_min", cfg.RateLimitLoginPerMin))
	}

	if poolCloser != nil {
		defer poolCloser()
	}
	if redisCloser != nil {
		defer func() {
			if err := redisCloser(); err != nil {
				logger.Warn("redis close failed", slog.String("error", err.Error()))
			}
		}()
	}
	if tracerShutdown != nil {
		defer func() {
			if err := tracerShutdown(context.Background()); err != nil {
				logger.Warn("tracer shutdown failed", slog.String("error", err.Error()))
			}
		}()
	}

	e := http.NewServer(serverDeps)
	serverErr := make(chan error, 1)

	go func() {
		if err := e.Start(":" + cfg.HTTPPort); err != nil {
			if err == stdhttp.ErrServerClosed {
				return
			}
			serverErr <- err
		}
	}()

	logger.Info("http server started", slog.String("port", cfg.HTTPPort), slog.String("app_env", cfg.AppEnv))

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		return e.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}
