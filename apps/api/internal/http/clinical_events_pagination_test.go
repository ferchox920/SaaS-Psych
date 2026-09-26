package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"sessionflow/apps/api/internal/http/handlers"
	httpmiddleware "sessionflow/apps/api/internal/http/middleware"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

type httpEventPageRepo struct {
	longitudinal.Repository
	items      []longitudinal.Event
	hypotheses []longitudinal.Hypothesis
	targets    []longitudinal.Target
	goals      []longitudinal.Goal
	giras      []longitudinal.GIRA
	processes  []longitudinal.Process
	reads      int
}

func (r *httpEventPageRepo) ListHypothesesPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.Hypothesis, error) {
	r.reads++
	if offset >= len(r.hypotheses) {
		return []longitudinal.Hypothesis{}, nil
	}
	end := offset + limit
	if end > len(r.hypotheses) {
		end = len(r.hypotheses)
	}
	return r.hypotheses[offset:end], nil
}

func (r *httpEventPageRepo) ListTargetsPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.Target, error) {
	r.reads++
	if offset >= len(r.targets) {
		return []longitudinal.Target{}, nil
	}
	end := offset + limit
	if end > len(r.targets) {
		end = len(r.targets)
	}
	return r.targets[offset:end], nil
}

func (r *httpEventPageRepo) ListGoalsPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.Goal, error) {
	r.reads++
	if offset >= len(r.goals) {
		return []longitudinal.Goal{}, nil
	}
	end := offset + limit
	if end > len(r.goals) {
		end = len(r.goals)
	}
	return r.goals[offset:end], nil
}

func (r *httpEventPageRepo) ListGIRAsPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.GIRA, error) {
	r.reads++
	if offset >= len(r.giras) {
		return []longitudinal.GIRA{}, nil
	}
	end := offset + limit
	if end > len(r.giras) {
		end = len(r.giras)
	}
	return r.giras[offset:end], nil
}

func (r *httpEventPageRepo) ListProcessesPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.Process, error) {
	r.reads++
	if offset >= len(r.processes) {
		return []longitudinal.Process{}, nil
	}
	end := offset + limit
	if end > len(r.processes) {
		end = len(r.processes)
	}
	return r.processes[offset:end], nil
}

func (r *httpEventPageRepo) ListEventsPage(_ context.Context, _, _ uuid.UUID, limit, offset int) ([]longitudinal.Event, error) {
	r.reads++
	if offset >= len(r.items) {
		return []longitudinal.Event{}, nil
	}
	end := offset + limit
	if end > len(r.items) {
		end = len(r.items)
	}
	return r.items[offset:end], nil
}

func TestClinicalEventsHTTPPaginationAndAccess(t *testing.T) {
	tenantID, userID, clientID := uuid.New(), uuid.New(), uuid.New()
	repo := &httpEventPageRepo{items: []longitudinal.Event{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, EventType: "other", ObservedAt: time.Now().UTC()},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, EventType: "other", ObservedAt: time.Now().UTC()},
	}, hypotheses: []longitudinal.Hypothesis{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Statement: "Primera", ConfidenceLevel: "yellow"},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Statement: "Segunda", ConfidenceLevel: "yellow"},
	}, targets: []longitudinal.Target{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Primero", TargetType: "other"},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Segundo", TargetType: "other"},
	}, goals: []longitudinal.Goal{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Primero", GoalType: "other"},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Segundo", GoalType: "other"},
	}, giras: []longitudinal.GIRA{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Primera", GIRAVersion: 2},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Segunda", GIRAVersion: 1},
	}, processes: []longitudinal.Process{
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Primero"},
		{ID: uuid.New(), TenantID: tenantID, ClientID: clientID, Title: "Segundo"},
	}}
	secret := "event-page-secret"
	tenantMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(httpmiddleware.WithTenantID(c.Request().Context(), tenantID)))
			return next(c)
		}
	}
	makeServer := func(allowed bool) *echo.Echo {
		service := longitudinal.NewService(repo, httpClinicalAccess{allowed}, nil, nil, nil, nil, "", "", nil)
		return NewServer(ServerDeps{TenantMiddleware: tenantMiddleware, AuthMiddleware: httpmiddleware.RequireAuth(secret), ClinicalLongitudinalHandler: handlers.NewClinicalLongitudinalHandler(service)})
	}
	token, _, err := authusecase.NewTokenService(secret, time.Hour).IssueAccessToken(userID, tenantID, "member")
	if err != nil {
		t.Fatal(err)
	}
	get := func(server *echo.Echo, path, query string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clients/"+clientID.String()+path+query, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return rec.Code, body
	}
	server := makeServer(true)
	status, body := get(server, "/events", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/events", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/events", "?limit=101")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid limit status=%d", status)
	}
	status, body = get(server, "/hypotheses", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first hypothesis page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/hypotheses", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last hypothesis page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/hypotheses", "?offset=-1")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid hypothesis offset status=%d", status)
	}
	status, body = get(server, "/targets", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first target page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/targets", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last target page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/targets", "?limit=101")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid target limit status=%d", status)
	}
	status, body = get(server, "/goals", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first goal page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/goals", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last goal page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/goals", "?offset=-1")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid goal offset status=%d", status)
	}
	status, body = get(server, "/giras", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first GIRA page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/giras", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last GIRA page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/giras", "?limit=101")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid GIRA limit status=%d", status)
	}
	status, body = get(server, "/processes", "?limit=1&offset=0")
	if status != http.StatusOK || body["next_offset"] != float64(1) || len(body["items"].([]any)) != 1 {
		t.Fatalf("first process page status=%d body=%#v", status, body)
	}
	status, body = get(server, "/processes", "?limit=1&offset=1")
	if status != http.StatusOK || body["next_offset"] != nil || len(body["items"].([]any)) != 1 {
		t.Fatalf("last process page status=%d body=%#v", status, body)
	}
	status, _ = get(server, "/processes", "?limit=101")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid process limit status=%d", status)
	}
	reads := repo.reads
	status, _ = get(makeServer(false), "/events", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
	status, _ = get(makeServer(false), "/hypotheses", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied hypothesis request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
	status, _ = get(makeServer(false), "/targets", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied target request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
	status, _ = get(makeServer(false), "/goals", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied goal request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
	status, _ = get(makeServer(false), "/giras", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied GIRA request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
	status, _ = get(makeServer(false), "/processes", "?limit=1")
	if status != http.StatusForbidden || repo.reads != reads {
		t.Fatalf("denied process request status=%d reads=%d before=%d", status, repo.reads, reads)
	}
}
