package db

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	domainclient "sessionflow/apps/api/internal/domain/client"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	material "sessionflow/apps/api/internal/domain/manualsource"
	clientusecase "sessionflow/apps/api/internal/usecase/client"
	"sessionflow/apps/api/internal/usecase/manualsource"
)

func TestManualSourcesIdempotencyHistoryAndIsolationPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tenant, actor, supervisor := uuid.New(), uuid.New(), uuid.New()
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO tenants(id,name) VALUES($1,'Synthetic material test')`, tenant)
	for _, id := range []uuid.UUID{actor, supervisor} {
		mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO users(id,tenant_id,email,password_hash) VALUES($1,$2,$3,'disabled-test')`, id, tenant, id.String()+"@example.invalid")
	}
	clients := NewClientRepository(pool).WithTransactionalAudit()
	entity, _ := domainclient.NewEntity(tenant, "Synthetic material patient", "", "", time.Now())
	entity, err = clients.Create(ctx, entity, actor)
	if err != nil {
		t.Fatal(err)
	}
	access := NewClinicalAccessRepository(pool)
	clientService := clientusecase.NewService(clients, NewAuditRepository(pool)).WithClinicalAccess(access)
	p := domainclient.DefaultProfile()
	p.ChatGPTProjects = []domainclient.ChatGPTProjectReference{{ProjectID: "g-p-11111111111111111111111111111111", ProjectName: "Synthetic case"}}
	if _, err = clientService.SaveProfile(ctx, tenant, entity.ID, actor, p); err != nil {
		t.Fatal(err)
	}
	repo := NewManualSourceRepository(pool)
	svc := manualsource.New(repo, access)
	v := material.Import{Kind: "chatgpt_project", ProjectID: p.ChatGPTProjects[0].ProjectID, SourceID: "stable-conversation", Timezone: "America/Buenos_Aires", Content: material.Content{Title: "Synthetic conversation", Messages: []material.Message{{ID: "u1", Role: "user", Text: "Synthetic dictation"}, {ID: "a1", Role: "assistant", Text: "Synthetic supervision"}}}}
	var wg sync.WaitGroup
	var created atomic.Int32
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, fresh, e := svc.Import(ctx, tenant, entity.ID, actor, v)
			if fresh {
				created.Add(1)
			}
			if e != nil {
				failures <- e
			}
		}()
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		t.Error(e)
	}
	if created.Load() != 1 {
		t.Fatalf("duplicate concurrent revisions: %d", created.Load())
	}
	first, fresh, err := svc.Import(ctx, tenant, entity.ID, actor, v)
	if err != nil || fresh || first.ReviewStatus != "pending" || first.SessionID != nil {
		t.Fatalf("idempotent pending import: %#v %v", first, err)
	}
	v.Content.Messages = append([]material.Message(nil), v.Content.Messages...)
	v.Content.Messages[1].Text = "New synthetic supervision"
	second, fresh, err := svc.Import(ctx, tenant, entity.ID, actor, v)
	if err != nil || !fresh || first.RevisionID == second.RevisionID || second.VersionCount != 2 {
		t.Fatalf("version history: %#v %v", second, err)
	}
	old, err := svc.Get(ctx, tenant, entity.ID, actor, first.ID, &first.RevisionID)
	versions, versionErr := svc.Versions(ctx, tenant, entity.ID, actor, first.ID, 100, 0)
	if versionErr != nil || len(versions) != 2 || versions[0].RevisionID != second.RevisionID {
		t.Fatalf("revision list: %#v %v", versions, versionErr)
	}
	if err != nil || old.Content.Messages[1].Text != "Synthetic supervision" {
		t.Fatalf("old source lost: %#v %v", old, err)
	}
	v.Content.Messages[1].Text = "Synthetic supervision"
	if _, fresh, err = svc.Import(ctx, tenant, entity.ID, actor, v); err != nil || fresh {
		t.Fatalf("old version retry: %v", err)
	}
	latest, err := svc.Get(ctx, tenant, entity.ID, actor, first.ID, nil)
	if err != nil || latest.RevisionID != second.RevisionID {
		t.Fatal("retry rolled back current source", err)
	}
	page, err := svc.List(ctx, tenant, entity.ID, actor, 100, 0)
	if err != nil || len(page) != 1 || page[0].Content.Title != v.Content.Title {
		t.Fatalf("list material: %#v %v", page, err)
	}
	if _, err = repo.Get(ctx, uuid.New(), entity.ID, first.ID, nil); !errors.Is(err, domainerrors.ErrNotFound) {
		t.Fatalf("cross tenant read: %v", err)
	}
	if _, _, err = svc.Import(ctx, uuid.New(), entity.ID, actor, v); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("cross tenant import: %v", err)
	}
	if _, err = svc.Get(ctx, tenant, entity.ID, supervisor, first.ID, nil); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("unassigned read: %v", err)
	}
	mustExecIntegrationSQL(t, pool, ctx, `INSERT INTO client_clinical_assignments(id,tenant_id,client_id,user_id,relationship,granted_by_user_id,starts_at) VALUES($1,$2,$3,$4,'supervisor',$5,now()-interval '1 minute')`, uuid.New(), tenant, entity.ID, supervisor, actor)
	if _, err = svc.Get(ctx, tenant, entity.ID, supervisor, first.ID, nil); err != nil {
		t.Fatal("supervisor read", err)
	}
	if _, _, err = svc.Import(ctx, tenant, entity.ID, supervisor, v); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatalf("supervisor write: %v", err)
	}
	v.ProjectID = "g-p-22222222222222222222222222222222"
	if _, _, err = svc.Import(ctx, tenant, entity.ID, actor, v); !errors.Is(err, domainerrors.ErrValidation) {
		t.Fatalf("unassociated project: %v", err)
	}
	var audit int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE tenant_id=$1 AND entity_id=$2 AND action='manual_source.imported'`, tenant, first.ID).Scan(&audit); err != nil || audit != 2 {
		t.Fatalf("audit count=%d err=%v", audit, err)
	}
}
