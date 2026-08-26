package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	sessionnoteusecase "sessionflow/apps/api/internal/usecase/sessionnote"
)

type SessionNoteHandler struct {
	service *sessionnoteusecase.Service
}

func NewSessionNoteHandler(service *sessionnoteusecase.Service) *SessionNoteHandler {
	return &SessionNoteHandler{service: service}
}

type createSessionNoteRequest struct {
	Body      string `json:"body"`
	IsPrivate *bool  `json:"is_private"`
}

type updateSessionNoteRequest struct {
	Body      string `json:"body"`
	IsPrivate *bool  `json:"is_private"`
}

type addendumSessionNoteRequest struct {
	Body   string `json:"body"`
	Reason string `json:"reason"`
}

type sessionNoteResponse struct {
	ID             string  `json:"id"`
	TenantID       string  `json:"tenant_id"`
	AppointmentID  string  `json:"appointment_id"`
	AuthorUserID   string  `json:"author_user_id"`
	Body           string  `json:"body"`
	IsPrivate      bool    `json:"is_private"`
	Status         string  `json:"status"`
	CurrentVersion int     `json:"current_version"`
	SignedAt       *string `json:"signed_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type sessionNoteVersionResponse struct {
	ID           string `json:"id"`
	NoteID       string `json:"note_id"`
	Version      int    `json:"version"`
	Body         string `json:"body"`
	IsPrivate    bool   `json:"is_private"`
	ChangeKind   string `json:"change_kind"`
	ChangeReason string `json:"change_reason"`
	ActorUserID  string `json:"actor_user_id"`
	CreatedAt    string `json:"created_at"`
}

func (h *SessionNoteHandler) Create(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "session note service unavailable")
	}

	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}

	appointmentID, err := uuid.Parse(c.Param("appointment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment id must be a valid uuid", map[string]any{"field": "appointment_id"})
	}

	var req createSessionNoteRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	note, err := h.service.Create(c.Request().Context(), sessionnoteusecase.CreateInput{
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  principal.UserID,
		Body:          req.Body,
		IsPrivate:     privateByDefault(req.IsPrivate),
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}

	return c.JSON(http.StatusCreated, toSessionNoteResponse(note))
}

func (h *SessionNoteHandler) ListByAppointment(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "session note service unavailable")
	}

	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}

	appointmentID, err := uuid.Parse(c.Param("appointment_id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "appointment id must be a valid uuid", map[string]any{"field": "appointment_id"})
	}

	notes, err := h.service.ListByAppointment(c.Request().Context(), tenantID, appointmentID, sessionnoteusecase.Viewer{
		UserID: principal.UserID,
		Role:   principal.Role,
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}

	resp := make([]sessionNoteResponse, 0, len(notes))
	for _, note := range notes {
		resp = append(resp, toSessionNoteResponse(note))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": resp})
}

func (h *SessionNoteHandler) Get(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "session note service unavailable")
	}

	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}

	noteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "note id must be a valid uuid", map[string]any{"field": "id"})
	}

	note, err := h.service.Get(c.Request().Context(), tenantID, noteID, sessionnoteusecase.Viewer{
		UserID: principal.UserID,
		Role:   principal.Role,
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}
	return c.JSON(http.StatusOK, toSessionNoteResponse(note))
}

func (h *SessionNoteHandler) Update(c echo.Context) error {
	if h.service == nil {
		return writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "session note service unavailable")
	}

	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return err
	}

	noteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "note id must be a valid uuid", map[string]any{"field": "id"})
	}

	var req updateSessionNoteRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"field": "body"})
	}

	note, err := h.service.Update(c.Request().Context(), sessionnoteusecase.UpdateInput{
		TenantID:    tenantID,
		NoteID:      noteID,
		ActorUserID: principal.UserID,
		ActorRole:   principal.Role,
		Body:        req.Body,
		IsPrivate:   req.IsPrivate,
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}

	return c.JSON(http.StatusOK, toSessionNoteResponse(note))
}

func (h *SessionNoteHandler) Sign(c echo.Context) error {
	tenantID, principal, noteID, err := h.noteRequestContext(c)
	if err != nil {
		return err
	}
	note, err := h.service.Sign(c.Request().Context(), tenantID, noteID, principal.UserID, principal.Role)
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}
	return c.JSON(http.StatusOK, toSessionNoteResponse(note))
}

func (h *SessionNoteHandler) Addendum(c echo.Context) error {
	tenantID, principal, noteID, err := h.noteRequestContext(c)
	if err != nil {
		return err
	}
	var req addendumSessionNoteRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, http.StatusBadRequest, "validation_error", "invalid request body")
	}
	note, err := h.service.Addendum(c.Request().Context(), sessionnoteusecase.AddendumInput{
		TenantID:    tenantID,
		NoteID:      noteID,
		ActorUserID: principal.UserID,
		ActorRole:   principal.Role,
		Body:        req.Body,
		Reason:      req.Reason,
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}
	return c.JSON(http.StatusOK, toSessionNoteResponse(note))
}

func (h *SessionNoteHandler) ListVersions(c echo.Context) error {
	tenantID, principal, noteID, err := h.noteRequestContext(c)
	if err != nil {
		return err
	}
	versions, err := h.service.ListVersions(c.Request().Context(), tenantID, noteID, sessionnoteusecase.Viewer{
		UserID: principal.UserID,
		Role:   principal.Role,
	})
	if err != nil {
		return h.handleSessionNoteError(c, err)
	}
	items := make([]sessionNoteVersionResponse, 0, len(versions))
	for _, version := range versions {
		items = append(items, toSessionNoteVersionResponse(version))
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *SessionNoteHandler) noteRequestContext(c echo.Context) (uuid.UUID, httpmiddleware.Principal, uuid.UUID, error) {
	if h.service == nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, writeAPIError(c, http.StatusServiceUnavailable, "service_unavailable", "session note service unavailable")
	}
	tenantID, principal, err := tenantAndPrincipal(c)
	if err != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, err
	}
	noteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpmiddleware.Principal{}, uuid.Nil, writeAPIError(c, http.StatusBadRequest, "validation_error", "note id must be a valid uuid", map[string]any{"field": "id"})
	}
	return tenantID, principal, noteID, nil
}

func privateByDefault(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func tenantAndPrincipal(c echo.Context) (uuid.UUID, httpmiddleware.Principal, error) {
	tenantID, ok := httpmiddleware.TenantIDFromContext(c.Request().Context())
	if !ok {
		return uuid.Nil, httpmiddleware.Principal{}, writeAPIError(c, http.StatusInternalServerError, "internal_error", "tenant context missing")
	}
	principal, ok := httpmiddleware.PrincipalFromContext(c.Request().Context())
	if !ok {
		return uuid.Nil, httpmiddleware.Principal{}, writeAPIError(c, http.StatusInternalServerError, "internal_error", "auth context missing")
	}
	return tenantID, principal, nil
}

func (h *SessionNoteHandler) handleSessionNoteError(c echo.Context, err error) error {
	return handleDomainError(c, err, defaultSessionNoteErrorMappings())
}

func defaultSessionNoteErrorMappings() []domainErrorMapping {
	return []domainErrorMapping{
		{Target: domainerrors.ErrValidation, Status: http.StatusBadRequest, Code: "validation_error"},
		{Target: domainerrors.ErrNotFound, Status: http.StatusNotFound, Code: "not_found", Message: "session note not found"},
		{Target: domainerrors.ErrForbidden, Status: http.StatusForbidden, Code: "forbidden", Message: "forbidden"},
		{Target: domainerrors.ErrConflict, Status: http.StatusConflict, Code: "conflict", Message: "session note was modified or signed"},
	}
}

func toSessionNoteResponse(note domainsessionnote.Entity) sessionNoteResponse {
	var signedAt *string
	if note.SignedAt != nil {
		formatted := note.SignedAt.UTC().Format(time.RFC3339)
		signedAt = &formatted
	}
	return sessionNoteResponse{
		ID:             note.ID.String(),
		TenantID:       note.TenantID.String(),
		AppointmentID:  note.AppointmentID.String(),
		AuthorUserID:   note.AuthorUserID.String(),
		Body:           note.Body,
		IsPrivate:      note.IsPrivate,
		Status:         note.Status,
		CurrentVersion: note.CurrentVersion,
		SignedAt:       signedAt,
		CreatedAt:      note.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      note.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSessionNoteVersionResponse(version domainsessionnote.Version) sessionNoteVersionResponse {
	return sessionNoteVersionResponse{
		ID:           version.ID.String(),
		NoteID:       version.NoteID.String(),
		Version:      version.Version,
		Body:         version.Body,
		IsPrivate:    version.IsPrivate,
		ChangeKind:   version.ChangeKind,
		ChangeReason: version.ChangeReason,
		ActorUserID:  version.ActorUserID.String(),
		CreatedAt:    version.CreatedAt.UTC().Format(time.RFC3339),
	}
}
