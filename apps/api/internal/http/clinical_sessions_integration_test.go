package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	domainclinicalsession "sessionflow/apps/api/internal/domain/clinicalsession"
	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	clinicalsession "sessionflow/apps/api/internal/usecase/clinicalsession"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type httpClinicalSessionRepo struct{ item domainclinicalsession.Entity }

func (r *httpClinicalSessionRepo) ClientExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (r *httpClinicalSessionRepo) AppointmentClient(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return r.item.ClientID, nil
}
func (r *httpClinicalSessionRepo) Create(_ context.Context, item domainclinicalsession.Entity, _ uuid.UUID) (domainclinicalsession.Entity, error) {
	r.item = item
	return item, nil
}
func (r *httpClinicalSessionRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (domainclinicalsession.Entity, error) {
	return r.item, nil
}
func (r *httpClinicalSessionRepo) ListByClient(context.Context, uuid.UUID, uuid.UUID) ([]domainclinicalsession.Entity, error) {
	return []domainclinicalsession.Entity{r.item}, nil
}
func (r *httpClinicalSessionRepo) Transition(_ context.Context, item domainclinicalsession.Entity, _ uuid.UUID) (domainclinicalsession.Entity, error) {
	r.item = item
	return item, nil
}

type httpClinicalAccess struct{ allowed bool }

func (a httpClinicalAccess) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

func TestClinicalSessionHTTPRequiresAuthenticationAndReturnsEnvelope(t *testing.T) {
	tenantID, userID, clientID := uuid.New(), uuid.New(), uuid.New()
	repo := &httpClinicalSessionRepo{item: domainclinicalsession.Entity{ClientID: clientID}}
	service := clinicalsession.NewService(repo, httpClinicalAccess{true})
	handler := handlers.NewClinicalSessionHandler(service)
	secret := "clinical-session-test-secret"
	tenantMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := httpmiddleware.WithTenantID(c.Request().Context(), tenantID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
	server := NewServer(ServerDeps{TenantMiddleware: tenantMiddleware, AuthMiddleware: httpmiddleware.RequireAuth(secret), ClinicalSessionHandler: handler})
	body := `{"client_id":"` + clientID.String() + `","started_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	unauth := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-sessions", strings.NewReader(body))
	unauth.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, unauth)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope["error"] == nil {
		t.Fatalf("missing error envelope: %s", rec.Body.String())
	}
	token, _, err := authusecase.NewTokenService(secret, time.Hour).IssueAccessToken(userID, tenantID, "member")
	if err != nil {
		t.Fatal(err)
	}
	auth := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-sessions", strings.NewReader(body))
	auth.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	auth.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, auth)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestClinicalSessionHTTPEnforcesClinicalWriteAccess(t *testing.T) {
	tenantID, userID, clientID := uuid.New(), uuid.New(), uuid.New()
	repo := &httpClinicalSessionRepo{item: domainclinicalsession.Entity{ClientID: clientID}}
	secret := "clinical-session-test-secret"
	service := clinicalsession.NewService(repo, httpClinicalAccess{false})
	tenantMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := httpmiddleware.WithTenantID(c.Request().Context(), tenantID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
	server := NewServer(ServerDeps{TenantMiddleware: tenantMiddleware, AuthMiddleware: httpmiddleware.RequireAuth(secret), ClinicalSessionHandler: handlers.NewClinicalSessionHandler(service)})
	token, _, _ := authusecase.NewTokenService(secret, time.Hour).IssueAccessToken(userID, tenantID, "member")
	body := `{"client_id":"` + clientID.String() + `","started_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-sessions", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type httpSessionReportRepo struct {
	item    sessionreport.Report
	details sessionreport.SessionDetails
}

func (r *httpSessionReportRepo) SessionDetails(context.Context, uuid.UUID, uuid.UUID) (sessionreport.SessionDetails, error) {
	return r.details, nil
}
func (r *httpSessionReportRepo) CreateDraft(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, *uuid.UUID, sessionreport.ReportV1) (sessionreport.Report, error) {
	return r.item, nil
}
func (r *httpSessionReportRepo) List(context.Context, uuid.UUID, uuid.UUID) ([]sessionreport.Report, error) {
	return []sessionreport.Report{r.item}, nil
}
func (r *httpSessionReportRepo) Get(context.Context, uuid.UUID, uuid.UUID) (sessionreport.Report, error) {
	return r.item, nil
}
func (r *httpSessionReportRepo) Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, sessionreport.ReportV1) (sessionreport.Report, error) {
	return r.item, nil
}
func (r *httpSessionReportRepo) Approve(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (sessionreport.Report, error) {
	return r.item, nil
}

func TestSessionReportHTTPRequiresAuthAndClinicalReadAccess(t *testing.T) {
	tenantID, userID, clientID, sessionID, reportID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	secret := "session-report-test-secret"
	repo := &httpSessionReportRepo{item: sessionreport.Report{ID: reportID, TenantID: tenantID, ClinicalSessionID: sessionID, Status: "approved"}, details: sessionreport.SessionDetails{ID: sessionID, ClientID: clientID}}
	tenantMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := httpmiddleware.WithTenantID(c.Request().Context(), tenantID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
	newServer := func(allowed bool) *echo.Echo {
		service := sessionreport.NewService(repo, httpClinicalAccess{allowed}, nil, nil, "", "", nil)
		return NewServer(ServerDeps{TenantMiddleware: tenantMiddleware, AuthMiddleware: httpmiddleware.RequireAuth(secret), SessionReportHandler: handlers.NewSessionReportHandler(service)})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/session-reports/"+reportID.String(), nil)
	rec := httptest.NewRecorder()
	newServer(true).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	token, _, _ := authusecase.NewTokenService(secret, time.Hour).IssueAccessToken(userID, tenantID, "member")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/session-reports/"+reportID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	newServer(false).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/session-reports/"+reportID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	newServer(true).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
