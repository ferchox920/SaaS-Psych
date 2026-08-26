package approvedcontext

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	clinicalmemory "sessionflow/apps/api/internal/usecase/clinicalmemory"
	sessionreport "sessionflow/apps/api/internal/usecase/sessionreport"
)

type formulationSourceStub struct {
	context clinicalmemory.ApprovedContext
	err     error
}

func (s formulationSourceStub) GetApprovedContext(context.Context, uuid.UUID, uuid.UUID) (clinicalmemory.ApprovedContext, error) {
	return s.context, s.err
}

type reportSourceStub struct{ reports []sessionreport.Report }

func (s reportSourceStub) ListApprovedByClient(context.Context, uuid.UUID, uuid.UUID, int) ([]sessionreport.Report, error) {
	items := []sessionreport.Report{}
	for _, item := range s.reports {
		if item.Status == "approved" {
			items = append(items, item)
		}
	}
	return items, nil
}

type accessStub struct{ allowed bool }

func (a accessStub) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}
func TestApprovedContextContainsOnlyApprovedSources(t *testing.T) {
	tenant, client, actor := uuid.New(), uuid.New(), uuid.New()
	sources := []sessionreport.Report{{ID: uuid.New(), Version: 1, Status: "draft"}, {ID: uuid.New(), Version: 1, Status: "approved"}, {ID: uuid.New(), Version: 1, Status: "superseded"}}
	formulation := clinicalmemory.ApprovedContext{SnapshotID: uuid.New(), Version: 3, ApprovedSummary: "Approved only"}
	out, err := NewService(formulationSourceStub{context: formulation}, reportSourceStub{sources}, accessStub{true}).Get(context.Background(), tenant, client, actor)
	if err != nil {
		t.Fatal(err)
	}
	if out.Formulation == nil || len(out.SessionReports) != 1 || out.SessionReports[0].Status != "approved" {
		t.Fatalf("context=%#v", out)
	}
	encoded, _ := json.Marshal(out)
	for _, forbidden := range []string{"pending", "discarded", "postponed", "suggestions"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("raw/pending context leaked: %s", encoded)
		}
	}
}
func TestApprovedContextHashIsStableForSameLogicalSources(t *testing.T) {
	tenant, client, actor := uuid.New(), uuid.New(), uuid.New()
	a, b := uuid.New(), uuid.New()
	formulation := clinicalmemory.ApprovedContext{SnapshotID: uuid.New(), Version: 2, ApprovedSummary: "approved", Anchors: []clinicalmemory.Anchor{{ID: a}, {ID: b}}}
	report := sessionreport.Report{ID: uuid.New(), Version: 3, Status: "approved"}
	first, err := NewService(formulationSourceStub{context: formulation}, reportSourceStub{[]sessionreport.Report{report}}, accessStub{true}).Get(context.Background(), tenant, client, actor)
	if err != nil {
		t.Fatal(err)
	}
	formulation.Anchors[0], formulation.Anchors[1] = formulation.Anchors[1], formulation.Anchors[0]
	second, err := NewService(formulationSourceStub{context: formulation}, reportSourceStub{[]sessionreport.Report{report}}, accessStub{true}).Get(context.Background(), tenant, client, actor)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContextHash != second.ContextHash {
		t.Fatalf("hash changed with source order: %s != %s", first.ContextHash, second.ContextHash)
	}
}
func TestApprovedContextOmitsDraftFormulation(t *testing.T) {
	out, err := NewService(formulationSourceStub{err: domainerrors.ErrNotFound}, reportSourceStub{}, accessStub{true}).Get(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if err != nil || out.Formulation != nil {
		t.Fatalf("context=%#v err=%v", out, err)
	}
}
func TestApprovedContextPreservesClinicalAndTenantAccess(t *testing.T) {
	_, err := NewService(formulationSourceStub{}, reportSourceStub{}, accessStub{false}).Get(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}
