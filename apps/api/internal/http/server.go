package http

import (
	"net"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
)

type ServerDeps struct {
	TrustedProxyRanges          []*net.IPNet
	ClinicalIngestionHandler    *handlers.ClinicalIngestionHandler
	ClinicalConsentHandler      *handlers.ClinicalConsentHandler
	WebOrigin                   string
	RequestTracingMiddleware    echo.MiddlewareFunc
	RequestLoggingMiddleware    echo.MiddlewareFunc
	RequestMetricsMiddleware    echo.MiddlewareFunc
	TenantMiddleware            echo.MiddlewareFunc
	AuthMiddleware              echo.MiddlewareFunc
	AuthLoginRateLimit          echo.MiddlewareFunc
	AuthHandler                 *handlers.AuthHandler
	AuditHandler                *handlers.AuditHandler
	ClientHandler               *handlers.ClientHandler
	AppointmentHandler          *handlers.AppointmentHandler
	SessionNoteHandler          *handlers.SessionNoteHandler
	ClinicalAccessHandler       *handlers.ClinicalAccessHandler
	ClinicalSessionHandler      *handlers.ClinicalSessionHandler
	SessionReportHandler        *handlers.SessionReportHandler
	ClinicalAnalysisHandler     *handlers.ClinicalAnalysisHandler
	ClinicalMemoryHandler       *handlers.ClinicalMemoryHandler
	ClinicalLongitudinalHandler *handlers.ClinicalLongitudinalHandler
	TranscriptionHandler        *handlers.TranscriptionHandler
	GoogleCalendarHandler       *handlers.GoogleCalendarHandler
	MetricsHandler              echo.HandlerFunc
	ReadinessChecks             map[string]handlers.ReadinessCheck
}

func NewServer(deps ServerDeps) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	if len(deps.TrustedProxyRanges) > 0 {
		options := []echo.TrustOption{echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false)}
		for _, proxyRange := range deps.TrustedProxyRanges {
			options = append(options, echo.TrustIPRange(proxyRange))
		}
		e.IPExtractor = echo.ExtractIPFromXFFHeader(options...)
	}
	e.HideBanner = true
	e.HidePort = true
	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.ReadTimeout = 60 * time.Second
	e.Server.IdleTimeout = 120 * time.Second
	// Clinical analysis responses use SSE; a server-wide write deadline would cut them off.
	e.Server.WriteTimeout = 0
	e.Use(httpmiddleware.JSONBodyLimit(1 << 20))
	if deps.WebOrigin != "" {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     []string{deps.WebOrigin},
			AllowMethods:     []string{echo.GET, echo.HEAD, echo.POST, echo.PUT, echo.PATCH, echo.DELETE, echo.OPTIONS},
			AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Tenant-ID", "X-Clinical-Audio-Consent"},
			AllowCredentials: true,
		}))
	}

	if deps.RequestTracingMiddleware != nil {
		e.Use(deps.RequestTracingMiddleware)
	}
	if deps.RequestLoggingMiddleware != nil {
		e.Use(deps.RequestLoggingMiddleware)
	}
	e.Use(middleware.Recover())
	if deps.RequestMetricsMiddleware != nil {
		e.Use(deps.RequestMetricsMiddleware)
	}

	e.GET("/health", handlers.Health)
	e.GET("/ready", handlers.Readiness(deps.ReadinessChecks))
	if deps.MetricsHandler != nil {
		e.GET("/metrics", deps.MetricsHandler)
	}
	e.GET("/docs", handlers.DocsUI)
	e.GET("/docs/openapi.yaml", handlers.OpenAPISpec)
	api := e.Group("/api/v1")
	if deps.GoogleCalendarHandler != nil {
		api.GET("/integrations/google-calendar/callback", deps.GoogleCalendarHandler.Callback)
	}

	if deps.AuthHandler != nil && deps.TenantMiddleware != nil {
		auth := api.Group("/auth", deps.TenantMiddleware)
		if deps.AuthLoginRateLimit != nil {
			auth.POST("/login", deps.AuthHandler.Login, deps.AuthLoginRateLimit)
		} else {
			auth.POST("/login", deps.AuthHandler.Login)
		}
		auth.POST("/refresh", deps.AuthHandler.Refresh)
		auth.POST("/logout", deps.AuthHandler.Logout)

		if deps.AuthMiddleware != nil {
			auth.GET("/me", deps.AuthHandler.Me, deps.AuthMiddleware)
			auth.GET("/admin-check", deps.AuthHandler.Me, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin"))
		}
	}

	if deps.AuditHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		audit := api.Group("/audit", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin"))
		audit.GET("", deps.AuditHandler.List)
	}

	if deps.ClientHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		clients := api.Group("/clients", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		clients.POST("", deps.ClientHandler.Create)
		clients.GET("", deps.ClientHandler.List)
		clients.GET("/archived", deps.ClientHandler.ListArchived)
		clients.GET("/:id", deps.ClientHandler.Get)
		clients.PUT("/:id", deps.ClientHandler.Update)
		clients.POST("/:id/archive", deps.ClientHandler.Archive)
		clients.POST("/:id/restore", deps.ClientHandler.Restore)
		clients.DELETE("/:id", deps.ClientHandler.Delete)
	}

	if deps.ClinicalAccessHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		assignments := api.Group("/clients/:client_id/assignments", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin"))
		assignments.POST("", deps.ClinicalAccessHandler.Grant)
		assignments.GET("", deps.ClinicalAccessHandler.List)
		assignments.DELETE("/:assignment_id", deps.ClinicalAccessHandler.End)
		exceptions := api.Group("/clients/:client_id/access-exceptions", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin"))
		exceptions.POST("", deps.ClinicalAccessHandler.GrantException)
		exceptions.GET("", deps.ClinicalAccessHandler.ListExceptions)
		exceptions.DELETE("/:exception_id", deps.ClinicalAccessHandler.RevokeException)
	}

	if deps.ClinicalSessionHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		sessions := api.Group("/clinical-sessions", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		sessions.POST("", deps.ClinicalSessionHandler.Create)
		sessions.GET("/:id", deps.ClinicalSessionHandler.Get)
		sessions.POST("/:id/complete", deps.ClinicalSessionHandler.Complete)
		sessions.POST("/:id/void", deps.ClinicalSessionHandler.Void)
		api.GET("/clients/:client_id/clinical-sessions", deps.ClinicalSessionHandler.ListByClient, deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		if deps.SessionReportHandler != nil {
			sessions.POST("/:id/reports/generate", deps.SessionReportHandler.Generate)
			sessions.GET("/:id/reports", deps.SessionReportHandler.List)
		}
	}
	if deps.SessionReportHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		reports := api.Group("/session-reports", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		reports.GET("/:report_id", deps.SessionReportHandler.Get)
		reports.PUT("/:report_id", deps.SessionReportHandler.Update)
		reports.POST("/:report_id/approve", deps.SessionReportHandler.Approve)
	}

	if deps.AppointmentHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		appointments := api.Group("/appointments", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		appointments.POST("", deps.AppointmentHandler.Create)
		appointments.GET("", deps.AppointmentHandler.List)
		appointments.PUT("/:id", deps.AppointmentHandler.Update)
		appointments.POST("/:id/cancel", deps.AppointmentHandler.Cancel)

		if deps.SessionNoteHandler != nil {
			appointments.POST("/:appointment_id/notes", deps.SessionNoteHandler.Create)
			appointments.GET("/:appointment_id/notes", deps.SessionNoteHandler.ListByAppointment)
		}
	}

	if deps.SessionNoteHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		notes := api.Group("/notes", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		notes.GET("/:id", deps.SessionNoteHandler.Get)
		notes.PUT("/:id", deps.SessionNoteHandler.Update)
		notes.POST("/:id/sign", deps.SessionNoteHandler.Sign)
		notes.POST("/:id/addenda", deps.SessionNoteHandler.Addendum)
		notes.GET("/:id/versions", deps.SessionNoteHandler.ListVersions)
	}

	if deps.ClinicalAnalysisHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		clinicalAI := api.Group("/clinical-ai", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		clinicalAI.GET("/status", deps.ClinicalAnalysisHandler.Status)
		clinicalAI.GET("/models", deps.ClinicalAnalysisHandler.Models)
		clinicalAI.POST("/warm", deps.ClinicalAnalysisHandler.Warm, httpmiddleware.RequireRole("owner", "admin"))
		clinicalAI.POST("/unload", deps.ClinicalAnalysisHandler.Unload, httpmiddleware.RequireRole("owner", "admin"))
		clinicalAI.POST("/analyze-live", deps.ClinicalAnalysisHandler.AnalyzeLive)
		clinicalAI.POST("/review-session", deps.ClinicalAnalysisHandler.ReviewSession)
	}

	if deps.ClinicalMemoryHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		memory := api.Group("", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		memory.GET("/appointments/:appointment_id/ai-suggestions", deps.ClinicalMemoryHandler.ListSuggestions)
		memory.PUT("/clinical-ai/suggestions/:suggestion_id/decision", deps.ClinicalMemoryHandler.DecideSuggestion)
		memory.GET("/clients/:client_id/formulations", deps.ClinicalMemoryHandler.ListSnapshots)
		memory.POST("/clients/:client_id/formulations", deps.ClinicalMemoryHandler.CreateSnapshot)
		memory.POST("/clients/:client_id/formulations/:snapshot_id/approve", deps.ClinicalMemoryHandler.ApproveSnapshot)
	}
	if deps.ClinicalLongitudinalHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		longitudinalAPI := api.Group("", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		longitudinalAPI.POST("/clinical-sessions/:id/longitudinal-analysis", deps.ClinicalLongitudinalHandler.Analyze)
		longitudinalAPI.GET("/clients/:id/longitudinal-state", deps.ClinicalLongitudinalHandler.State)
		longitudinalAPI.GET("/clients/:id/approved-clinical-context", deps.ClinicalLongitudinalHandler.ApprovedContext)
		longitudinalAPI.GET("/clients/:id/clinical-runs/:run_id/provenance", deps.ClinicalLongitudinalHandler.RunProvenance)
		longitudinalAPI.GET("/clients/:id/clinical-history/:kind/:entity_id", deps.ClinicalLongitudinalHandler.History)
		longitudinalAPI.GET("/clients/:id/evidence", deps.ClinicalLongitudinalHandler.Evidence)
		longitudinalAPI.GET("/clients/:id/evidence-page", deps.ClinicalLongitudinalHandler.EvidencePage)
		longitudinalAPI.GET("/clients/:id/events", deps.ClinicalLongitudinalHandler.Events)
		longitudinalAPI.GET("/clients/:id/processes", deps.ClinicalLongitudinalHandler.Processes)
		longitudinalAPI.GET("/clients/:id/hypotheses", deps.ClinicalLongitudinalHandler.Hypotheses)
		longitudinalAPI.GET("/clients/:id/targets", deps.ClinicalLongitudinalHandler.Targets)
		longitudinalAPI.GET("/clients/:id/goals", deps.ClinicalLongitudinalHandler.Goals)
		longitudinalAPI.GET("/clients/:id/giras", deps.ClinicalLongitudinalHandler.GIRAs)
		longitudinalAPI.GET("/giras/:id", deps.ClinicalLongitudinalHandler.GetGIRA)
		longitudinalAPI.GET("/therapeutic-approaches", deps.ClinicalLongitudinalHandler.Approaches)
		longitudinalAPI.GET("/therapeutic-techniques", deps.ClinicalLongitudinalHandler.Techniques)
		longitudinalAPI.POST("/processes/:id/gira-analysis", deps.ClinicalLongitudinalHandler.AnalyzeGIRA)
		longitudinalAPI.GET("/clients/:id/clinical-diffs", deps.ClinicalLongitudinalHandler.Diffs)
		longitudinalAPI.GET("/clients/:id/project-exports", deps.ClinicalLongitudinalHandler.ProjectExports)
		longitudinalAPI.POST("/clients/:id/project-exports", deps.ClinicalLongitudinalHandler.ExportProject)
		longitudinalAPI.GET("/clients/:id/project-exports/:export_id", deps.ClinicalLongitudinalHandler.GetProjectExport)
		longitudinalAPI.POST("/clients/:id/project-imports", deps.ClinicalLongitudinalHandler.ImportProject)
		longitudinalAPI.GET("/clients/:id/project-proposals/:proposal_id", deps.ClinicalLongitudinalHandler.GetProjectProposal)
		longitudinalAPI.GET("/clinical-diffs/:id", deps.ClinicalLongitudinalHandler.GetDiff)
		longitudinalAPI.PUT("/clinical-diffs/:id/operations/:operation_id/decision", deps.ClinicalLongitudinalHandler.Decide)
		longitudinalAPI.POST("/clinical-diffs/:id/merge", deps.ClinicalLongitudinalHandler.Merge)
	}
	if deps.ClinicalIngestionHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		ingestionAPI := api.Group("", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		h := deps.ClinicalIngestionHandler
		ingestionAPI.GET("/clinical-ingestion/health", h.Health)
		ingestionAPI.POST("/clinical-sessions/:id/audio", h.Upload)
		ingestionAPI.GET("/clinical-sessions/:id/artifacts", h.Artifacts)
		ingestionAPI.GET("/clinical-sessions/:id/jobs", h.Jobs)
		ingestionAPI.POST("/clinical-sessions/:id/transcriptions", h.Transcribe)
		ingestionAPI.GET("/clinical-sessions/:id/transcripts", h.Transcripts)
		ingestionAPI.POST("/clinical-sessions/:id/analysis-jobs", h.Analyze)
		ingestionAPI.GET("/clinical-transcripts/:id", h.Transcript)
		ingestionAPI.DELETE("/clinical-transcripts/:id", h.DeleteTranscript)
		ingestionAPI.POST("/clinical-transcripts/:id/revisions", h.CorrectTranscript)
		ingestionAPI.DELETE("/clinical-artifacts/:id", h.DeleteArtifact)
		ingestionAPI.GET("/clinical-jobs/:id", h.Job)
		ingestionAPI.POST("/clinical-jobs/:id/retry", h.Retry)
		ingestionAPI.POST("/clinical-jobs/:id/cancel", h.Cancel)
	}
	if deps.ClinicalConsentHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		consents := api.Group("", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		consents.GET("/clients/:id/consents", deps.ClinicalConsentHandler.List)
		consents.POST("/clients/:id/consents", deps.ClinicalConsentHandler.Grant)
		consents.POST("/clients/:id/consents/:grant_id/revoke", deps.ClinicalConsentHandler.Revoke)
	}

	if deps.TranscriptionHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		transcriptionAPI := api.Group("", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		transcriptionAPI.GET("/clinical-transcription/status", deps.TranscriptionHandler.Status)
		transcriptionAPI.POST("/appointments/:appointment_id/transcription", deps.TranscriptionHandler.Transcribe)
	}

	if deps.GoogleCalendarHandler != nil && deps.TenantMiddleware != nil && deps.AuthMiddleware != nil {
		calendar := api.Group("/integrations/google-calendar", deps.TenantMiddleware, deps.AuthMiddleware, httpmiddleware.RequireRole("owner", "admin", "member"))
		calendar.GET("/status", deps.GoogleCalendarHandler.Status)
		calendar.POST("/authorize", deps.GoogleCalendarHandler.BeginAuthorization)
		calendar.GET("/events", deps.GoogleCalendarHandler.ListCandidates)
		calendar.POST("/events/:event_id/import", deps.GoogleCalendarHandler.ImportEvent)
		calendar.POST("/appointments/:appointment_id/push", deps.GoogleCalendarHandler.PushAppointment)
		calendar.DELETE("/connection", deps.GoogleCalendarHandler.Disconnect)
	}

	return e
}
