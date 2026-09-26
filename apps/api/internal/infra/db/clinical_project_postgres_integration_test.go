package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func TestManualProjectTriangulationNoDirectMergeAndReverseProvenancePostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	service, repo, _ := newLongitudinalAcceptanceService(pool, nil, true)
	e2, h := uuid.New(), uuid.New()
	mergeAcceptanceOperations(t, pool, f.base, []longitudinal.Operation{
		{OperationType: "create_evidence", TargetEntityID: e2, OriginalProposal: mustJSON(longitudinal.CreateEvidenceProposal{SourceItemID: "fact-003", EpistemicType: "patient_report"})},
		{OperationType: "create_hypothesis", TargetEntityID: h, OriginalProposal: mustJSON(longitudinal.CreateHypothesisProposal{ProcessID: &f.process, Statement: "Exploratory avoidance hypothesis", ConfidenceLevel: "yellow", SupportingEvidenceIDs: []uuid.UUID{f.evidence}, ContradictingEvidenceIDs: []uuid.UUID{e2}})},
	})
	for i := 0; i < 3; i++ {
		p, err := repo.GetProcess(ctx, f.base.tenant, f.process)
		if err != nil {
			t.Fatal(err)
		}
		mergeAcceptanceOperations(t, pool, f.base, []longitudinal.Operation{{OperationType: "update_process", TargetEntityID: f.process, ExpectedEntityVersion: &p.Version, OriginalProposal: mustJSON(longitudinal.UpdateProcessProposal{Title: "Selected process", Description: "Synthetic current process", EvidenceIDs: []uuid.UUID{f.evidence}})}})
	}
	state, err := repo.State(ctx, f.base.tenant, f.base.client)
	if err != nil || state.StateVersion != 5 {
		t.Fatalf("expected v5: %v %+v", err, state)
	}
	fingerprint := longitudinalStateFingerprint(t, pool, f.base.tenant, f.base.client)
	unchanged := func(stage string) {
		t.Helper()
		if got := longitudinalStateFingerprint(t, pool, f.base.tenant, f.base.client); got != fingerprint {
			t.Fatalf("%s mutated state", stage)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM giras WHERE tenant_id=$1 AND client_id=$2`, f.base.tenant, f.base.client).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s created GIRA: %v", stage, err)
		}
	}
	export, err := service.ExportProject(ctx, f.base.tenant, f.base.client, f.base.user, f.process)
	if err != nil {
		t.Fatal(err)
	}
	unchanged("EXPORT")
	loaded, err := repo.GetProjectExport(ctx, f.base.tenant, f.base.client, export.ID)
	if err != nil || loaded.ContentHash != export.ContentHash || len(loaded.Sources) != 4 {
		t.Fatalf("export provenance: %v sources=%d", err, len(loaded.Sources))
	}
	provider := postgresGIRAProvider{pool: pool, tenant: f.base.tenant}
	synthetic, err := provider.BuildGIRA(ctx, "", longitudinal.GIRAProviderRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var strategy longitudinal.GIRASemanticProposal
	if err = json.Unmarshal(synthetic.JSON, &strategy); err != nil {
		t.Fatal(err)
	}
	proposal := longitudinal.ClinicalProjectImportV1{SchemaVersion: longitudinal.ProjectImportVersion, SourceExportID: export.ID, SourceExportHash: export.ContentHash, Provenance: longitudinal.ManualProvenance{SourceType: "manual_external_ai", Assertion: "user_supplied", Surface: "ChatGPT Project", ModelName: "therapist-supplied synthetic model"}, Operations: []longitudinal.PortableProjectOperation{}, Strategy: &strategy, OpenQuestions: []longitudinal.GIRASemanticUncertainty{}, SupervisionObservations: []string{"Synthetic therapist supervision note"}}
	var runsBefore int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_ai_runs WHERE tenant_id=$1`, f.base.tenant).Scan(&runsBefore)
	diff, err := service.ImportProject(ctx, f.base.tenant, f.base.client, f.base.user, mustJSON(proposal))
	if err != nil {
		t.Fatal(err)
	}
	if diff.Status != "pending_review" || diff.SourceAIRunID != nil || diff.SourceExternalProposalID == nil {
		t.Fatalf("wrong manual provenance: %+v", diff)
	}
	unchanged("IMPORT")
	record, err := service.GetProjectProposal(ctx, f.base.tenant, f.base.client, f.base.user, *diff.SourceExternalProposalID)
	if err != nil || record.ExportID != export.ID || record.Proposal.Provenance.Assertion != "user_supplied" {
		t.Fatalf("reverse proposal: %v", err)
	}
	var runsAfter int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_ai_runs WHERE tenant_id=$1`, f.base.tenant).Scan(&runsAfter)
	if runsAfter != runsBefore {
		t.Fatal("manual import fabricated AIRun")
	}
	for _, op := range diff.Operations {
		diff, err = service.Decide(ctx, longitudinal.DecisionInput{TenantID: f.base.tenant, DiffID: diff.ID, OperationID: op.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision, Decision: "approved"})
		if err != nil {
			t.Fatal(err)
		}
		unchanged("REVIEW")
	}
	merged, err := service.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if merged.MergedStateVersion == nil || *merged.MergedStateVersion != 6 {
		t.Fatal("merge did not advance v5 to v6")
	}
	if longitudinalStateFingerprint(t, pool, f.base.tenant, f.base.client) == fingerprint {
		t.Fatal("merge did not change state")
	}
	repeated, err := service.Merge(ctx, longitudinal.MergeInput{TenantID: f.base.tenant, DiffID: diff.ID, ActorID: f.base.user, ExpectedDiffRevision: diff.Revision})
	if err != nil || repeated.ID != merged.ID || *repeated.MergedStateVersion != 6 {
		t.Fatal("merge not idempotent")
	}
	var chain int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM giras g JOIN clinical_longitudinal_transitions tr ON tr.tenant_id=g.tenant_id AND tr.entity_id=g.id JOIN clinical_diffs d ON d.tenant_id=tr.tenant_id AND d.id=tr.diff_id JOIN clinical_external_proposals ep ON ep.tenant_id=d.tenant_id AND ep.id=d.source_external_proposal_id JOIN clinical_project_exports ex ON ex.tenant_id=ep.tenant_id AND ex.id=ep.source_export_id JOIN clinical_project_export_sources src ON src.tenant_id=ex.tenant_id AND src.export_id=ex.id WHERE g.tenant_id=$1 AND g.client_id=$2 AND ex.id=$3`, f.base.tenant, f.base.client, export.ID).Scan(&chain)
	if err != nil || chain < 4 {
		t.Fatalf("GIRA reverse provenance missing: %v count=%d", err, chain)
	}
	if _, err = service.ImportProject(ctx, f.base.tenant, f.base.client, f.base.user, mustJSON(proposal)); !errors.Is(err, domainerrors.ErrConflict) {
		t.Fatalf("stale import: %v", err)
	}
	for _, scope := range [][2]uuid.UUID{{uuid.New(), f.base.client}, {f.base.tenant, uuid.New()}} {
		if _, err = repo.GetProjectExport(ctx, scope[0], scope[1], export.ID); !errors.Is(err, domainerrors.ErrNotFound) {
			t.Fatal("foreign export exposed")
		}
		if _, err = repo.GetProjectProposal(ctx, scope[0], scope[1], record.ID); !errors.Is(err, domainerrors.ErrNotFound) {
			t.Fatal("foreign provenance exposed")
		}
	}
	denied, _, _ := newLongitudinalAcceptanceService(pool, nil, false)
	if _, err = denied.ExportProject(ctx, f.base.tenant, f.base.client, f.base.user, f.process); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatal("unauthorized export")
	}
	if _, err = denied.ImportProject(ctx, f.base.tenant, f.base.client, f.base.user, mustJSON(proposal)); !errors.Is(err, domainerrors.ErrForbidden) {
		t.Fatal("unauthorized import")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM clinical_project_exports WHERE tenant_id=$1 AND id=$2`, f.base.tenant, export.ID); err == nil {
		t.Fatal("export hard deletion accepted")
	}
	if _, err = pool.Exec(ctx, `UPDATE clinical_external_proposals SET surface='altered' WHERE tenant_id=$1 AND id=$2`, f.base.tenant, record.ID); err == nil {
		t.Fatal("external provenance modification accepted")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clinical_project_export_sources(tenant_id,client_id,export_id,export_ref,entity_type,entity_id,entity_version) VALUES($1,$2,$3,'foreign_source','evidence',$4,1)`, f.base.tenant, f.base.client, export.ID, uuid.New()); err == nil {
		t.Fatal("foreign source mapping accepted")
	}
}

func TestManualProjectUnsafeImportCreatesNoArtifactsPostgres(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := NewPostgresPool(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	f := newStrategyFixture(t, pool)
	s, _, _ := newLongitudinalAcceptanceService(pool, nil, true)
	export, err := s.ExportProject(ctx, f.base.tenant, f.base.client, f.base.user, f.process)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"achieve_goal", "complete_gira_phase", "update_gira", "create_evidence", "invalid_operation"} {
		p := longitudinal.ClinicalProjectImportV1{SchemaVersion: longitudinal.ProjectImportVersion, SourceExportID: export.ID, SourceExportHash: export.ContentHash, Provenance: longitudinal.ManualProvenance{SourceType: "manual_external_ai", Assertion: "user_supplied"}, Operations: []longitudinal.PortableProjectOperation{{OperationType: kind}}}
		if _, err = s.ImportProject(ctx, f.base.tenant, f.base.client, f.base.user, mustJSON(p)); err == nil {
			t.Fatalf("unsafe %s accepted", kind)
		}
	}
	var n int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_external_proposals WHERE tenant_id=$1 AND client_id=$2`, f.base.tenant, f.base.client).Scan(&n)
	if err != nil || n != 0 {
		t.Fatal("unsafe import persisted provenance")
	}
	err = pool.QueryRow(ctx, `SELECT count(*) FROM clinical_diffs WHERE tenant_id=$1 AND client_id=$2 AND source_external_proposal_id IS NOT NULL`, f.base.tenant, f.base.client).Scan(&n)
	if err != nil || n != 0 {
		t.Fatal("unsafe import persisted diff")
	}
}
