package clinicalsession

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainclinicalsession "sessionflow/apps/api/internal/domain/clinicalsession"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
)

type repoStub struct {
	clientExists      bool
	appointmentClient uuid.UUID
	appointmentErr    error
	item              domainclinicalsession.Entity
	created           bool
}

func (r *repoStub) ClientExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return r.clientExists, nil
}
func (r *repoStub) AppointmentClient(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return r.appointmentClient, r.appointmentErr
}
func (r *repoStub) Create(_ context.Context, item domainclinicalsession.Entity, _ uuid.UUID) (domainclinicalsession.Entity, error) {
	r.created = true
	r.item = item
	return item, nil
}
func (r *repoStub) GetByID(_ context.Context, tenantID, sessionID uuid.UUID) (domainclinicalsession.Entity, error) {
	if r.item.TenantID != tenantID || r.item.ID != sessionID {
		return domainclinicalsession.Entity{}, domainerrors.ErrNotFound
	}
	return r.item, nil
}
func (r *repoStub) ListByClient(context.Context, uuid.UUID, uuid.UUID) ([]domainclinicalsession.Entity, error) {
	return []domainclinicalsession.Entity{r.item}, nil
}
func (r *repoStub) Transition(_ context.Context, item domainclinicalsession.Entity, _ uuid.UUID) (domainclinicalsession.Entity, error) {
	r.item = item
	return item, nil
}

type accessStub struct{ allowed bool }

func (a accessStub) CanAccessClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, ...string) (bool, error) {
	return a.allowed, nil
}

func validInput() CreateInput {
	return CreateInput{TenantID: uuid.New(), ClientID: uuid.New(), TherapistUserID: uuid.New(), StartedAt: time.Now().Add(-time.Hour)}
}
func TestTreatingCanCreateClinicalSession(t *testing.T) {
	input := validInput()
	repo := &repoStub{clientExists: true}
	service := NewService(repo, accessStub{true})
	item, err := service.Create(context.Background(), input)
	if err != nil || !repo.created || item.Status != domainclinicalsession.StatusInProgress {
		t.Fatalf("item=%#v err=%v", item, err)
	}
}
func TestClinicalSessionCreateRequiresCAWrite(t *testing.T) {
	input := validInput()
	_, err := NewService(&repoStub{clientExists: true}, accessStub{false}).Create(context.Background(), input)
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}
func TestClinicalSessionCreateRejectsMissingClient(t *testing.T) {
	input := validInput()
	_, err := NewService(&repoStub{}, accessStub{true}).Create(context.Background(), input)
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
func TestClinicalSessionCreateRejectsAppointmentForOtherClient(t *testing.T) {
	input := validInput()
	appointment := uuid.New()
	input.AppointmentID = &appointment
	_, err := NewService(&repoStub{clientExists: true, appointmentClient: uuid.New()}, accessStub{true}).Create(context.Background(), input)
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}
func TestClinicalSessionCreateRejectsAppointmentFromOtherTenant(t *testing.T) {
	input := validInput()
	appointment := uuid.New()
	input.AppointmentID = &appointment
	_, err := NewService(&repoStub{clientExists: true, appointmentErr: domainerrors.ErrNotFound}, accessStub{true}).Create(context.Background(), input)
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
func TestClinicalSessionCannotCrossTenantOnRead(t *testing.T) {
	input := validInput()
	repo := &repoStub{clientExists: true}
	service := NewService(repo, accessStub{true})
	item, _ := service.Create(context.Background(), input)
	_, err := service.Get(context.Background(), uuid.New(), item.ID, input.TherapistUserID)
	if !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}
