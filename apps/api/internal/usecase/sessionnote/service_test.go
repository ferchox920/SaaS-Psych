package sessionnote

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainerrors "sessionflow/apps/api/internal/domain/errors"
	domainsessionnote "sessionflow/apps/api/internal/domain/sessionnote"
)

type fakeRepository struct {
	notes []domainsessionnote.Entity
}

type fakeAuditor struct {
	actions []string
}

func (f *fakeAuditor) RecordDomainEvent(
	_ context.Context,
	_ uuid.UUID,
	_ uuid.UUID,
	action string,
	_ string,
	_ *uuid.UUID,
	_ map[string]any,
) error {
	f.actions = append(f.actions, action)
	return nil
}

func (f fakeRepository) Create(_ context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	return in, nil
}

func (f fakeRepository) Update(_ context.Context, in domainsessionnote.Entity) (domainsessionnote.Entity, error) {
	return in, nil
}

func (f fakeRepository) GetByID(_ context.Context, tenantID, noteID uuid.UUID) (domainsessionnote.Entity, error) {
	for _, note := range f.notes {
		if note.TenantID == tenantID && note.ID == noteID {
			return note, nil
		}
	}
	return domainsessionnote.Entity{}, domainerrors.ErrNotFound
}

func (f fakeRepository) ListByAppointment(_ context.Context, tenantID, appointmentID uuid.UUID) ([]domainsessionnote.Entity, error) {
	out := make([]domainsessionnote.Entity, 0)
	for _, note := range f.notes {
		if note.TenantID == tenantID && note.AppointmentID == appointmentID {
			out = append(out, note)
		}
	}
	return out, nil
}

func (f fakeRepository) AppointmentExists(_ context.Context, tenantID, appointmentID uuid.UUID) (bool, error) {
	return true, nil
}

func TestGetPrivateNoteForbiddenForOtherMember(t *testing.T) {
	tenantID := uuid.New()
	authorID := uuid.New()
	noteID := uuid.New()
	appointmentID := uuid.New()

	svc := NewService(fakeRepository{notes: []domainsessionnote.Entity{{
		ID:            noteID,
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  authorID,
		Body:          "private",
		IsPrivate:     true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}}}, nil)

	_, err := svc.Get(context.Background(), tenantID, noteID, Viewer{UserID: uuid.New(), Role: "member"})
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestListByAppointmentFiltersPrivateNotes(t *testing.T) {
	tenantID := uuid.New()
	appointmentID := uuid.New()
	authorID := uuid.New()

	svc := NewService(fakeRepository{notes: []domainsessionnote.Entity{
		{ID: uuid.New(), TenantID: tenantID, AppointmentID: appointmentID, AuthorUserID: authorID, Body: "public", IsPrivate: false},
		{ID: uuid.New(), TenantID: tenantID, AppointmentID: appointmentID, AuthorUserID: authorID, Body: "private", IsPrivate: true},
	}}, nil)

	notes, err := svc.ListByAppointment(context.Background(), tenantID, appointmentID, Viewer{UserID: uuid.New(), Role: "member"})
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 visible note, got %d", len(notes))
	}
}

func TestCreateRecordsDomainAudit(t *testing.T) {
	auditor := &fakeAuditor{}
	tenantID := uuid.New()
	authorID := uuid.New()
	appointmentID := uuid.New()
	svc := NewService(fakeRepository{}, auditor)

	if _, err := svc.Create(context.Background(), CreateInput{
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  authorID,
		Body:          "test note",
		IsPrivate:     true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(auditor.actions) != 1 || auditor.actions[0] != "session_note.create" {
		t.Fatalf("expected session_note.create action, got %v", auditor.actions)
	}
}

func TestUpdateForbiddenForOtherMember(t *testing.T) {
	tenantID := uuid.New()
	authorID := uuid.New()
	noteID := uuid.New()
	appointmentID := uuid.New()

	svc := NewService(fakeRepository{notes: []domainsessionnote.Entity{{
		ID:            noteID,
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  authorID,
		Body:          "private",
		IsPrivate:     true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}}}, nil)

	_, err := svc.Update(context.Background(), UpdateInput{
		TenantID:    tenantID,
		NoteID:      noteID,
		ActorUserID: uuid.New(),
		ActorRole:   "member",
		Body:        "updated",
		IsPrivate:   false,
	})
	if !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateRecordsDomainAuditAndPreservesImmutableFields(t *testing.T) {
	auditor := &fakeAuditor{}
	tenantID := uuid.New()
	authorID := uuid.New()
	noteID := uuid.New()
	appointmentID := uuid.New()
	createdAt := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	updateNow := createdAt.Add(2 * time.Hour)

	svc := NewService(fakeRepository{notes: []domainsessionnote.Entity{{
		ID:            noteID,
		TenantID:      tenantID,
		AppointmentID: appointmentID,
		AuthorUserID:  authorID,
		Body:          "before",
		IsPrivate:     true,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}}}, auditor)
	svc.now = func() time.Time { return updateNow }

	out, err := svc.Update(context.Background(), UpdateInput{
		TenantID:    tenantID,
		NoteID:      noteID,
		ActorUserID: authorID,
		ActorRole:   "member",
		Body:        "after",
		IsPrivate:   false,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.AppointmentID != appointmentID {
		t.Fatalf("appointment_id must remain immutable, got %s want %s", out.AppointmentID, appointmentID)
	}
	if out.AuthorUserID != authorID {
		t.Fatalf("author_user_id must remain immutable, got %s want %s", out.AuthorUserID, authorID)
	}
	if out.Body != "after" || out.IsPrivate {
		t.Fatalf("unexpected updated payload: body=%q is_private=%v", out.Body, out.IsPrivate)
	}
	if !out.UpdatedAt.Equal(updateNow) {
		t.Fatalf("expected updated_at %s, got %s", updateNow, out.UpdatedAt)
	}
	if len(auditor.actions) != 1 || auditor.actions[0] != "session_note.update" {
		t.Fatalf("expected session_note.update action, got %v", auditor.actions)
	}
}
