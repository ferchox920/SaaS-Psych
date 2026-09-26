package main

import (
	"context"
	"errors"
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
	"sessionflow/apps/api/internal/infra/artifactstore"
	"sessionflow/apps/api/internal/infra/db"
	demoinfra "sessionflow/apps/api/internal/infra/demoprovider"
	googlecalendarinfra "sessionflow/apps/api/internal/infra/googlecalendar"
	ollamainfra "sessionflow/apps/api/internal/infra/ollama"
	redisinfra "sessionflow/apps/api/internal/infra/redis"
	remotegirainfra "sessionflow/apps/api/internal/infra/remotegira"
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
	consentusecase "sessionflow/apps/api/internal/usecase/consent"
	googlecalendarusecase "sessionflow/apps/api/internal/usecase/googlecalendar"
	"sessionflow/apps/api/internal/usecase/ingestion"
	longitudinalusecase "sessionflow/apps/api/internal/usecase/longitudinal"
	sessionnoteusecase "sessionflow/apps/api/internal/usecase/sessionnote"
	sessionreportusecase "sessionflow/apps/api/internal/usecase/sessionreport"
	tenantusecase "sessionflow/apps/api/internal/usecase/tenant"
	transcriptionusecase "sessionflow/apps/api/internal/usecase/transcription"

	"github.com/google/uuid"
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
	trustedProxyRanges, err := cfg.TrustedProxyRanges()
	if err != nil {
		return err
	}

	serverDeps := http.ServerDeps{
		TrustedProxyRanges:       trustedProxyRanges,
		RequestLoggingMiddleware: httpmiddleware.RequestLogging(logger),
		WebOrigin:                cfg.WebOrigin,
		ReadinessChecks: map[string]httphandlers.ReadinessCheck{
			"postgres": func(context.Context) error { return errors.New("postgres is not configured") },
		},
	}
	if cfg.RateLimitLoginPerMin > 0 {
		serverDeps.ReadinessChecks["redis"] = func(context.Context) error { return errors.New("redis is not configured") }
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
	var workerStop func()
	if cfg.DatabaseURL != "" {
		pool, err := db.NewPostgresPoolWithTracing(ctx, cfg.DatabaseURL, db.PoolTracingConfig{
			Tracer:             tracerProvider.Tracer("sessionflow/db"),
			DBStatementEnabled: cfg.OTELDBStatement,
		})
		if err != nil {
			return err
		}
		serverDeps.ReadinessChecks["postgres"] = pool.Ping

		tenantRepo := db.NewTenantRepository(pool)
		tenantService := tenantusecase.NewService(tenantRepo)
		serverDeps.TenantMiddleware = httpmiddleware.RequireTenant(tenantService)

		authRepo := db.NewAuthRepository(pool)
		auditRepo := db.NewAuditRepository(pool)
		clientRepo := db.NewClientRepository(pool).WithTransactionalAudit()
		appointmentRepo := db.NewAppointmentRepository(pool).WithTransactionalAudit()
		sessionNoteRepo := db.NewSessionNoteRepository(pool).WithTransactionalAudit()
		clinicalAccessRepo := db.NewClinicalAccessRepository(pool)
		consentRepo := db.NewClinicalConsentRepository(pool)
		serverDeps.ClinicalConsentHandler = httphandlers.NewClinicalConsentHandler(consentusecase.NewService(consentRepo, clinicalAccessRepo))
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
			GIRAContextTokens:     cfg.OllamaGIRAContextTokens,
			GIRATemperature:       cfg.OllamaGIRATemperature,
			GIRATimeout:           time.Duration(cfg.OllamaGIRATimeoutSeconds) * time.Second,
			GIRAMaxOutputTokens:   cfg.OllamaGIRAMaxOutputTokens,
			GIRAThink:             cfg.OllamaGIRAThink,
			GIRATopP:              cfg.OllamaGIRATopP,
			GIRATopK:              cfg.OllamaGIRATopK,
			GIRASeed:              cfg.OllamaGIRASeed,
		})
		if err != nil {
			return err
		}
		var reportProvider sessionreportusecase.Provider = ollamaProvider
		var longitudinalProvider longitudinalusecase.Provider = ollamaProvider
		providerName, providerModel := "ollama", cfg.OllamaModel
		providerParameters := map[string]any{"context_tokens": cfg.OllamaReviewContextTokens, "temperature": cfg.OllamaReviewTemperature, "max_output_tokens": cfg.OllamaReviewMaxOutputTokens}
		if cfg.DemoMode {
			fixture := demoinfra.New()
			reportProvider, longitudinalProvider = fixture, fixture
			providerName, providerModel = "demo_fixture", "fixed-fixture-v1"
			providerParameters = map[string]any{"simulated": true}
			logger.Warn("DEMO_MODE enabled: report and longitudinal outputs are fixed synthetic fixtures, not clinical inference")
		}
		sessionReportService := sessionreportusecase.NewService(sessionReportRepo, clinicalAccessRepo, clinicalAIRunService, reportProvider, providerName, providerModel, providerParameters).WithMetrics(clinicalMetrics)
		clinicalAnalysisService := clinicalanalysisusecase.NewService(ollamaProvider, appointmentRepo, clinicalAccessRepo, auditRepo).WithLongitudinalMemory(clinicalMemoryRepo).WithMetrics(clinicalMetrics).WithRiskProtocol(cfg.ClinicalRiskProtocol).WithRunTracking(clinicalAIRunService, "ollama", cfg.OllamaModel, map[string]any{"context_tokens": cfg.OllamaContextTokens, "temperature": cfg.OllamaTemperature, "max_output_tokens": cfg.OllamaMaxOutputTokens, "review_context_tokens": cfg.OllamaReviewContextTokens, "review_temperature": cfg.OllamaReviewTemperature, "review_max_output_tokens": cfg.OllamaReviewMaxOutputTokens})
		longitudinalService := longitudinalusecase.NewService(longitudinalRepo, clinicalAccessRepo, approvedContextService, clinicalAIRunService, longitudinalProvider, auditRepo, providerName, providerModel, providerParameters).WithMetrics(clinicalMetrics)
		// Production boundaries always require durable consent, independently of
		// whether audio ingestion/its worker are enabled. Pure synthetic unit
		// constructors remain available without a database consent dependency.
		transcriptionService.WithConsent(consentRepo)
		sessionReportService.WithConsent(consentRepo)
		clinicalAnalysisService.WithConsent(consentRepo)
		longitudinalService.WithConsent(consentRepo)
		if cfg.ClinicalIngestionEnabled {
			store, e := artifactstore.New(cfg.ClinicalArtifactRoot, cfg.ClinicalArtifactKeyID, cfg.ClinicalArtifactKey, int64(cfg.TranscriberMaxAudioMB)*1024*1024)
			if e != nil {
				return e
			}
			ingestionRepo := db.NewClinicalIngestionRepository(pool)
			ingestionService := ingestion.NewService(ingestionRepo, sessionReportRepo, clinicalAccessRepo, store, cfg.ClinicalArtifactKeyID, time.Duration(cfg.ClinicalAudioRetentionHours)*time.Hour)
			analysisHash := clinicalairunusecase.Hash(map[string]any{"provider": "ollama", "model": cfg.OllamaModel, "prompt_hash": ingestion.Hash(sessionreportusecase.SystemPromptV1()), "schema": sessionreportusecase.SchemaVersion, "context_tokens": cfg.OllamaReviewContextTokens, "temperature": cfg.OllamaReviewTemperature, "max_output_tokens": cfg.OllamaReviewMaxOutputTokens, "think": cfg.OllamaReviewThink})
			serverDeps.ClinicalIngestionHandler = httphandlers.NewClinicalIngestionHandler(ingestionService, int64(cfg.TranscriberMaxAudioMB)*1024*1024, cfg.ClinicalTranscriptionConfigurationHash, analysisHash)
			worker, e := ingestion.NewWorker(ingestionRepo, store, transcriberProvider, ollamaProvider, cfg.OllamaModel, cfg.AppVersion, cfg.BuildRevision, time.Duration(cfg.ClinicalIngestionTimeoutSeconds)*time.Second)
			if e != nil {
				return e
			}
			tenants, e := cfg.IngestionTenants()
			if e != nil {
				return e
			}
			ingestionMetrics, e := observability.NewIngestionMetrics(registry)
			if e != nil {
				return e
			}
			worker.WithMetrics(ingestionMetrics)
			maintenance := ingestion.NewMaintenance(ingestionRepo, store)
			serverDeps.ClinicalIngestionHandler.WithHealth(func(parent context.Context) map[string]any {
				healthCtx, cancelHealth := context.WithTimeout(parent, 2*time.Second)
				defer cancelHealth()
				sidecar, healthErr := transcriberProvider.Health(healthCtx)
				return map[string]any{"worker_configured": len(tenants) > 0, "worker": worker.Health(), "sidecar_available": healthErr == nil && sidecar.Available}
			})
			workerCtx, stop := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				defer close(done)
				cursors := map[uuid.UUID]uuid.UUID{}
				offsets := map[uuid.UUID]int{}
				lastMaintenance := time.Time{}
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-workerCtx.Done():
						return
					case <-ticker.C:
						maintain := time.Since(lastMaintenance) > time.Minute
						for _, tenant := range tenants {
							if workerCtx.Err() != nil {
								return
							}
							_, e := worker.RunOne(workerCtx, tenant)
							if e != nil && workerCtx.Err() == nil {
								logger.Warn("clinical worker attempt did not succeed", slog.String("result", "safe_failure"))
							}
							if maintain && workerCtx.Err() == nil {
								next, offset, cleanupErr := maintenance.RunBatch(workerCtx, tenant, cursors[tenant], offsets[tenant])
								// Advance bounded inventory even after an isolated fault. The
								// cursor wraps, so failed entries are retried without starving others.
								cursors[tenant] = next
								offsets[tenant] = offset
								if cleanupErr != nil {
									logger.Warn("clinical artifact maintenance incomplete", slog.String("result", "safe_failure"))
								}
							}
						}
						if maintain {
							lastMaintenance = time.Now()
						}
					}
				}
			}()
			workerStop = func() { stop(); <-done }
		}
		// Dormant Stage 2C.4 infrastructure. Stage 2C.5 uses manual project import.
		if cfg.ExperimentalRemoteGIRA && strings.EqualFold(strings.TrimSpace(cfg.ClinicalGIRAProvider), "openai") {
			transport, transportErr := remotegirainfra.NewHTTPTransport(remotegirainfra.HTTPTransportConfig{BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey, Timeout: time.Duration(cfg.OpenAIGIRATimeoutSeconds) * time.Second})
			if transportErr != nil {
				return transportErr
			}
			giraProvider, providerErr := remotegirainfra.NewOpenAIGIRABuilderProvider(transport, remotegirainfra.OpenAIConfig{Model: cfg.ClinicalGIRAModel, MaxOutputTokens: cfg.OpenAIGIRAMaxOutputTokens, ProviderRegion: cfg.ClinicalGIRAProviderRegion, DataControlMode: cfg.ClinicalGIRADataControlMode, DataControlStatus: cfg.ClinicalGIRADataControlStatus, Pricing: remotegirainfra.PricingConfig{Version: cfg.GIRAPriceConfigVersion, InputUSDPerMillion: cfg.GIRAInputUSDPerMillion, CachedInputUSDPerMillion: cfg.GIRACachedInputUSDPerMillion, OutputUSDPerMillion: cfg.GIRAOutputUSDPerMillion}})
			if providerErr != nil {
				return providerErr
			}
			longitudinalService.WithConfiguredGIRABuilder(giraProvider, "openai", cfg.ClinicalGIRAModel, map[string]any{"max_output_tokens": cfg.OpenAIGIRAMaxOutputTokens, "repair_attempts": 1, "ai_contract": "semantic_v1", "minimization_policy_version": longitudinalusecase.GIRAPrivacyPolicyVersion, "provider_region": cfg.ClinicalGIRAProviderRegion, "data_control_mode": cfg.ClinicalGIRADataControlMode, "data_control_status": cfg.ClinicalGIRADataControlStatus, "price_config_version": cfg.GIRAPriceConfigVersion})
		}
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
		serverDeps.ReadinessChecks["redis"] = func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }

		store := redisinfra.NewLoginRateLimitStoreWithTracer(redisClient, tracerProvider.Tracer("sessionflow/redis"))
		serverDeps.AuthLoginRateLimit = httpmiddleware.RequireLoginRateLimit(store, cfg.RateLimitLoginPerMin, time.Minute)
		redisCloser = redisClient.Close
		logger.Info("redis connection established", slog.Int("rate_limit_login_per_min", cfg.RateLimitLoginPerMin))
	}

	if poolCloser != nil {
		defer poolCloser()
	}
	if workerStop != nil {
		defer workerStop()
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
