package main

import (
	"context"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sessionflow/apps/api/internal/config"
	"sessionflow/apps/api/internal/http"
	httphandlers "sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	"sessionflow/apps/api/internal/infra/db"
	googlecalendarinfra "sessionflow/apps/api/internal/infra/googlecalendar"
	ollamainfra "sessionflow/apps/api/internal/infra/ollama"
	redisinfra "sessionflow/apps/api/internal/infra/redis"
	"sessionflow/apps/api/internal/infra/secretbox"
	transcriberinfra "sessionflow/apps/api/internal/infra/transcriber"
	"sessionflow/apps/api/internal/observability"
	appointmentusecase "sessionflow/apps/api/internal/usecase/appointment"
	approvedcontextusecase "sessionflow/apps/api/internal/usecase/approvedcontext"
	auditusecase "sessionflow/apps/api/internal/usecase/audit"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
	clinicalaccessusecase "sessionflow/apps/api/internal/usecase/clinicalaccess"
	clinicalairunusecase "sessionflow/apps/api/internal/usecase/clinicalairun"
	clinicalanalysisusecase "sessionflow/apps/api/internal/usecase/clinicalanalysis"
	clinicalmemoryusecase "sessionflow/apps/api/internal/usecase/clinicalmemory"
	clinicalsessionusecase "sessionflow/apps/api/internal/usecase/clinicalsession"
	googlecalendarusecase "sessionflow/apps/api/internal/usecase/googlecalendar"
	longitudinalusecase "sessionflow/apps/api/internal/usecase/longitudinal"
	sessionnoteusecase "sessionflow/apps/api/internal/usecase/sessionnote"
	sessionreportusecase "sessionflow/apps/api/internal/usecase/sessionreport"
	tenantusecase "sessionflow/apps/api/internal/usecase/tenant"
	transcriptionusecase "sessionflow/apps/api/internal/usecase/transcription"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	config.LoadLocalEnv()
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
	if err := cfg.Validate(); err != nil {
		return err
	}

	serverDeps := http.ServerDeps{
		RequestLoggingMiddleware: httpmiddleware.RequestLogging(logger),
		WebOrigin:                cfg.WebOrigin,
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
	domainMetrics, err := observability.NewDomainMetrics(registry)
	if err != nil {
		return err
	}
	clinicalMetrics, err := observability.NewClinicalMetrics(registry)
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
		clientRepo := db.NewClientRepository(pool).WithTransactionalAudit()
		appointmentRepo := db.NewAppointmentRepository(pool).WithTransactionalAudit()
		sessionNoteRepo := db.NewSessionNoteRepository(pool).WithTransactionalAudit()
		clinicalAccessRepo := db.NewClinicalAccessRepository(pool)
		clinicalMemoryRepo := db.NewClinicalMemoryRepository(pool)
		clinicalSessionRepo := db.NewClinicalSessionRepository(pool)
		clinicalAIRunRepo := db.NewClinicalAIRunRepository(pool)
		sessionReportRepo := db.NewSessionReportRepository(pool)
		longitudinalRepo := db.NewClinicalLongitudinalRepository(pool)
		clinicalAccessService := clinicalaccessusecase.NewService(clinicalAccessRepo)
		clinicalSessionService := clinicalsessionusecase.NewService(clinicalSessionRepo, clinicalAccessRepo).WithMetrics(clinicalMetrics)
		clinicalAIRunService := clinicalairunusecase.NewService(clinicalAIRunRepo).WithMetrics(clinicalMetrics).WithBuildInfo(cfg.AppVersion, cfg.BuildRevision)
		approvedContextService := approvedcontextusecase.NewService(clinicalMemoryRepo, sessionReportRepo, clinicalAccessRepo)
		clinicalMemoryService := clinicalmemoryusecase.NewService(clinicalMemoryRepo, clinicalAccessRepo, auditRepo).WithMetrics(clinicalMetrics)
		transcriberProvider, err := transcriberinfra.NewProvider(cfg.TranscriberBaseURL, time.Duration(cfg.TranscriberTimeoutSec)*time.Second)
		if err != nil {
			return err
		}
		transcriptionService := transcriptionusecase.NewService(
			cfg.ClinicalAudioEnabled, cfg.TranscriberMaxAudioMB*1024*1024,
			transcriberProvider, appointmentRepo, clinicalAccessRepo, auditRepo,
		).WithMetrics(clinicalMetrics)
		ollamaProvider, err := ollamainfra.NewProvider(ollamainfra.Config{
			BaseURL:               cfg.OllamaBaseURL,
			Model:                 cfg.OllamaModel,
			ContextTokens:         cfg.OllamaContextTokens,
			Temperature:           cfg.OllamaTemperature,
			Timeout:               time.Duration(cfg.OllamaTimeoutSeconds) * time.Second,
			KeepAlive:             cfg.OllamaKeepAlive,
			MaxOutputTokens:       cfg.OllamaMaxOutputTokens,
			ReviewContextTokens:   cfg.OllamaReviewContextTokens,
			ReviewTemperature:     cfg.OllamaReviewTemperature,
			ReviewTimeout:         time.Duration(cfg.OllamaReviewTimeoutSeconds) * time.Second,
			ReviewMaxOutputTokens: cfg.OllamaReviewMaxOutputTokens,
			ReviewThink:           cfg.OllamaReviewThink,
		})
		if err != nil {
			return err
		}
		sessionReportService := sessionreportusecase.NewService(sessionReportRepo, clinicalAccessRepo, clinicalAIRunService, ollamaProvider, "ollama", cfg.OllamaModel, map[string]any{"context_tokens": cfg.OllamaReviewContextTokens, "temperature": cfg.OllamaReviewTemperature, "max_output_tokens": cfg.OllamaReviewMaxOutputTokens}).WithMetrics(clinicalMetrics)
		clinicalAnalysisService := clinicalanalysisusecase.NewService(ollamaProvider, appointmentRepo, clinicalAccessRepo, auditRepo).WithLongitudinalMemory(clinicalMemoryRepo).WithMetrics(clinicalMetrics).WithRiskProtocol(cfg.ClinicalRiskProtocol).WithRunTracking(clinicalAIRunService, "ollama", cfg.OllamaModel, map[string]any{"context_tokens": cfg.OllamaContextTokens, "temperature": cfg.OllamaTemperature, "max_output_tokens": cfg.OllamaMaxOutputTokens, "review_context_tokens": cfg.OllamaReviewContextTokens, "review_temperature": cfg.OllamaReviewTemperature, "review_max_output_tokens": cfg.OllamaReviewMaxOutputTokens})
		longitudinalService := longitudinalusecase.NewService(longitudinalRepo, clinicalAccessRepo, approvedContextService, clinicalAIRunService, ollamaProvider, auditRepo, "ollama", cfg.OllamaModel, map[string]any{"context_tokens": cfg.OllamaReviewContextTokens, "temperature": cfg.OllamaReviewTemperature, "max_output_tokens": cfg.OllamaReviewMaxOutputTokens}).WithMetrics(clinicalMetrics)
		auditService := auditusecase.NewService(auditRepo)
		clientService := clientusecase.NewService(clientRepo, auditRepo).WithClinicalAccess(clinicalAccessRepo)
		appointmentService := appointmentusecase.NewService(appointmentRepo, auditRepo).WithMetrics(domainMetrics).WithClinicalAccess(clinicalAccessRepo)
		sessionNoteService := sessionnoteusecase.NewService(sessionNoteRepo, auditRepo).WithClinicalAccess(clinicalAccessRepo)
		tokenService := authusecase.NewTokenService(cfg.JWTAccessSecret, cfg.AccessTTL())
		authService := authusecase.NewService(authRepo, tokenService, cfg.RefreshTTL(), auditRepo).WithMetrics(domainMetrics)
		serverDeps.AuthHandler = httphandlers.NewAuthHandler(authService, httphandlers.AuthCookieConfig{
			Secure: cfg.AuthCookieSecure, SameSite: authCookieSameSite(cfg.AuthCookieSameSite),
			Domain: cfg.AuthCookieDomain, MaxAge: cfg.RefreshTTL(),
		})
		serverDeps.AuditHandler = httphandlers.NewAuditHandler(auditService)
		serverDeps.ClientHandler = httphandlers.NewClientHandler(clientService)
		serverDeps.AppointmentHandler = httphandlers.NewAppointmentHandler(appointmentService)
		serverDeps.SessionNoteHandler = httphandlers.NewSessionNoteHandler(sessionNoteService)
		serverDeps.ClinicalAccessHandler = httphandlers.NewClinicalAccessHandler(clinicalAccessService)
		serverDeps.ClinicalSessionHandler = httphandlers.NewClinicalSessionHandler(clinicalSessionService)
		serverDeps.SessionReportHandler = httphandlers.NewSessionReportHandler(sessionReportService)
		serverDeps.ClinicalAnalysisHandler = httphandlers.NewClinicalAnalysisHandler(clinicalAnalysisService)
		serverDeps.ClinicalMemoryHandler = httphandlers.NewClinicalMemoryHandler(clinicalMemoryService)
		serverDeps.ClinicalLongitudinalHandler = httphandlers.NewClinicalLongitudinalHandler(longitudinalService)
		serverDeps.TranscriptionHandler = httphandlers.NewTranscriptionHandler(transcriptionService, int64(cfg.TranscriberMaxAudioMB)*1024*1024)
		calendarService := googlecalendarusecase.NewService(false, nil, nil, nil, nil, nil, nil, nil)
		if cfg.GoogleCalendarEnabled {
			calendarProvider, err := googlecalendarinfra.NewProvider(googlecalendarinfra.Config{
				ClientID: cfg.GoogleCalendarClientID, ClientSecret: cfg.GoogleCalendarClientSecret,
				RedirectURL: cfg.GoogleCalendarRedirectURL, Timeout: time.Duration(cfg.GoogleCalendarTimeoutSec) * time.Second,
			})
			if err != nil {
				return err
			}
			tokenBox, err := secretbox.New(cfg.GoogleCalendarTokenKey)
			if err != nil {
				return err
			}
			calendarRepo := db.NewGoogleCalendarRepository(pool)
			calendarService = googlecalendarusecase.NewService(true, calendarRepo, calendarProvider, tokenBox, appointmentService, appointmentRepo, clinicalAccessRepo, auditRepo)
		}
		serverDeps.GoogleCalendarHandler = httphandlers.NewGoogleCalendarHandler(calendarService, cfg.WebOrigin)
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

func authCookieSameSite(value string) stdhttp.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return stdhttp.SameSiteStrictMode
	case "none":
		return stdhttp.SameSiteNoneMode
	default:
		return stdhttp.SameSiteLaxMode
	}
}
