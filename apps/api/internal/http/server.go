package http

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
)

type ServerDeps struct {
	RequestTracingMiddleware echo.MiddlewareFunc
	RequestLoggingMiddleware echo.MiddlewareFunc
	RequestMetricsMiddleware echo.MiddlewareFunc
	TenantMiddleware         echo.MiddlewareFunc
	AuthMiddleware           echo.MiddlewareFunc
	AuthLoginRateLimit       echo.MiddlewareFunc
	AuthHandler              *handlers.AuthHandler
	AuditHandler             *handlers.AuditHandler
	ClientHandler            *handlers.ClientHandler
	AppointmentHandler       *handlers.AppointmentHandler
	SessionNoteHandler       *handlers.SessionNoteHandler
	MetricsHandler           echo.HandlerFunc
}

func NewServer(deps ServerDeps) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

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
	if deps.MetricsHandler != nil {
		e.GET("/metrics", deps.MetricsHandler)
	}
	e.GET("/docs", handlers.DocsUI)
	e.GET("/docs/openapi.yaml", handlers.OpenAPISpec)
	api := e.Group("/api/v1")

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
		clients.GET("/:id", deps.ClientHandler.Get)
		clients.PUT("/:id", deps.ClientHandler.Update)
		clients.DELETE("/:id", deps.ClientHandler.Delete)
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
	}

	return e
}
