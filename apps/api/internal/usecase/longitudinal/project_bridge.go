package longitudinal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	domainerrors "sessionflow/apps/api/internal/domain/errors"
	"sessionflow/apps/api/internal/usecase/consent"
)

const ProjectExportVersion = "clinical-project-export-v1"
const ProjectImportVersion = "clinical-project-import-v1"

// ProjectSource is a LOCAL provenance mapping. It never appears in the portable artifact.
type ProjectSource struct {
	Ref        string    `json:"ref"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	Version    int       `json:"version"`
}
type CaseRuntimeProfile struct {
	Scope               string   `json:"scope"`
	ProcessRef          string   `json:"process_ref"`
	ProcessStatus       string   `json:"process_status"`
	HypothesisRefs      []string `json:"hypothesis_refs"`
	GoalRefs            []string `json:"goal_refs"`
	GIRARefs            []string `json:"gira_refs"`
	OpenQuestions       []string `json:"open_questions"`
	UnavailableSections []string `json:"unavailable_sections"`
}
type ProjectArtifact struct {
	SchemaVersion   string             `json:"schema_version"`
	StateVersion    int64              `json:"state_version"`
	PrivacyMode     string             `json:"privacy_mode"`
	ApprovalLabel   string             `json:"approval_label"`
	EpistemicLabels map[string]string  `json:"epistemic_labels"`
	RuntimeProfile  CaseRuntimeProfile `json:"case_runtime_profile"`
	Sources         RemoteGIRAContext  `json:"selected_clinical_sources"`
	Warnings        []string           `json:"warnings"`
}
type ProjectExport struct {
	ID           uuid.UUID        `json:"export_id"`
	TenantID     uuid.UUID        `json:"tenant_id"`
	ClientID     uuid.UUID        `json:"client_id"`
	GeneratedBy  uuid.UUID        `json:"generated_by_user_id"`
	GeneratedAt  time.Time        `json:"generated_at"`
	StateVersion int64            `json:"state_version"`
	ContentHash  string           `json:"content_hash"`
	Lifecycle    string           `json:"lifecycle"`
	Artifact     ProjectArtifact  `json:"artifact"`
	Markdown     string           `json:"markdown"`
	Sources      []ProjectSource  `json:"-"`
	Snapshot     GIRABuilderInput `json:"-"`
}
type ManualProvenance struct {
	SourceType            string `json:"source_type"`
	ProviderName          string `json:"provider_name,omitempty"`
	ModelName             string `json:"model_name,omitempty"`
	Surface               string `json:"surface,omitempty"`
	ProjectContextVersion string `json:"project_context_version,omitempty"`
	Assertion             string `json:"provenance_assertion"`
}
type PortableProjectOperation struct {
	ID                    string          `json:"id"`
	OperationType         string          `json:"operation_type"`
	TargetRef             string          `json:"target_ref"`
	ExpectedEntityVersion *int            `json:"expected_entity_version"`
	Proposal              json.RawMessage `json:"proposal"`
	SourceRefs            []string        `json:"source_refs"`
	Rationale             string          `json:"rationale"`
	Uncertainty           string          `json:"uncertainty,omitempty"`
}
type ClinicalProjectImportV1 struct {
	SchemaVersion           string                     `json:"schema_version"`
	SourceExportID          uuid.UUID                  `json:"source_export_id"`
	SourceExportHash        string                     `json:"source_export_hash"`
	Provenance              ManualProvenance           `json:"provenance"`
	Operations              []PortableProjectOperation `json:"operations"`
	Strategy                *GIRASemanticProposal      `json:"strategy,omitempty"`
	OpenQuestions           []GIRASemanticUncertainty  `json:"open_questions"`
	SupervisionObservations []string                   `json:"supervision_observations"`
}
type ProjectImportRecord struct {
	ID          uuid.UUID               `json:"id"`
	TenantID    uuid.UUID               `json:"tenant_id"`
	ClientID    uuid.UUID               `json:"client_id"`
	ExportID    uuid.UUID               `json:"source_export_id"`
	ImportedBy  uuid.UUID               `json:"imported_by_user_id"`
	ContentHash string                  `json:"content_hash"`
	Proposal    ClinicalProjectImportV1 `json:"proposal"`
}
type ProjectRepository interface {
	SaveProjectExport(context.Context, ProjectExport) (ProjectExport, error)
	GetProjectExport(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ProjectExport, error)
	ImportProjectProposal(context.Context, ProjectImportRecord, CreateDiffInput) (Diff, error)
	GetProjectProposal(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (ProjectImportRecord, error)
}

func projectHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// BuildProjectExport never reads a provider. Its bounded snapshot contains only the selected approved process.
func BuildProjectExport(t, c, a uuid.UUID, state State, processID uuid.UUID, approaches []ApproachDefinition, techniques []TechniqueDefinition) (ProjectExport, error) {
	if t == uuid.Nil || c == uuid.Nil || a == uuid.Nil {
		return ProjectExport{}, domainerrors.NewValidation("export tenant, client and human actor are required")
	}
	var p Process
	for _, candidate := range state.Processes {
		if candidate.ID == processID {
			p = candidate
			break
		}
	}
	if p.ID == uuid.Nil || p.ApprovalStatus != "approved" {
		return ProjectExport{}, domainerrors.ErrNotFound
	}
	if err := validateGIRAOutboundOwnership(t, c, p, state); err != nil {
		return ProjectExport{}, err
	}
	// Copy through JSON before filtering: callers' approved state must remain unchanged.
	raw, _ := json.Marshal(p)
	_ = json.Unmarshal(raw, &p)
	p.Events = filterProjectEvents(p.Events)
	hs := []Hypothesis{}
	for _, h := range p.Hypotheses {
		if h.ApprovalStatus == "approved" && h.ClinicalStatus != "superseded" && h.ClinicalStatus != "retired" {
			hs = append(hs, h)
		}
	}
	p.Hypotheses = hs
	strategy := TherapeuticStrategy{Targets: []Target{}, Goals: []Goal{}, Rationales: []TherapeuticRationale{}, GIRAs: []GIRA{}}
	if p.TherapeuticStrategy != nil {
		for _, x := range p.TherapeuticStrategy.Targets {
			if x.ApprovalStatus == "approved" {
				strategy.Targets = append(strategy.Targets, x)
			}
		}
		for _, x := range p.TherapeuticStrategy.Goals {
			if x.ApprovalStatus == "approved" {
				strategy.Goals = append(strategy.Goals, x)
			}
		}
		for _, x := range p.TherapeuticStrategy.Rationales {
			if x.ApprovalStatus == "approved" {
				strategy.Rationales = append(strategy.Rationales, x)
			}
		}
		for _, x := range p.TherapeuticStrategy.GIRAs {
			if x.ApprovalStatus == "approved" && x.ClinicalStatus != "superseded" {
				strategy.GIRAs = append(strategy.GIRAs, x)
			}
		}
	}
	p.TherapeuticStrategy = &strategy
	if err := validateProjectOwnership(p, t, c); err != nil {
		return ProjectExport{}, err
	}
	// Membership comes from approved links; no unrelated patient evidence is exported.
	wanted := map[uuid.UUID]bool{}
	for _, e := range p.Events {
		for _, v := range e.Evidence {
			wanted[v.ID] = true
		}
	}
	for _, h := range hs {
		for _, v := range append(append([]Evidence{}, h.SupportingEvidence...), h.ContradictingEvidence...) {
			wanted[v.ID] = true
		}
	}
	for _, x := range strategy.Targets {
		for _, id := range x.EvidenceIDs {
			wanted[id] = true
		}
	}
	for _, x := range strategy.Rationales {
		for _, id := range x.EvidenceIDs {
			wanted[id] = true
		}
	}
	evidence := []Evidence{}
	for _, e := range state.ActiveEvidence {
		if wanted[e.ID] && e.Status == "active" {
			evidence = append(evidence, e)
		}
	}
	req := BuildGIRAGenerationRequest(GIRABuilderInput{SchemaVersion: GIRAPromptVersion, SelectedProcess: p, ApprovedEvidence: evidence, ApprovedEvents: p.Events, ApprovedHypotheses: hs, CurrentStrategy: strategy, ApproachRegistry: approaches, TechniqueRegistry: techniques, StateVersion: state.StateVersion})
	artifact := ProjectArtifact{SchemaVersion: ProjectExportVersion, StateVersion: state.StateVersion, PrivacyMode: "minimized", ApprovalLabel: "APPROVED", Sources: req.RemoteContext, EpistemicLabels: map[string]string{}, Warnings: []string{"Manual transfer requires therapist preview. Nothing has been uploaded.", "Minimization is not anonymization; clinical narrative may identify a person.", "External output is an untrusted proposal. Human Review and Human Merge are mandatory."}}
	profile := CaseRuntimeProfile{Scope: "selected_process", ProcessRef: "process_1", ProcessStatus: p.ClinicalStatus, HypothesisRefs: []string{}, GoalRefs: []string{}, GIRARefs: []string{}, OpenQuestions: []string{}, UnavailableSections: []string{"unselected_formulation", "unselected_session_reports", "alliance_notes", "therapist_patterns"}}
	sources := []ProjectSource{{Ref: "process_1", EntityType: "process", EntityID: p.ID, Version: p.Version}}
	add := func(ref, kind string, id uuid.UUID, version int, label string) {
		sources = append(sources, ProjectSource{ref, kind, id, version})
		artifact.EpistemicLabels[ref] = label
	}
	artifact.EpistemicLabels["process_1"] = "APPROVED_CLINICAL_PROCESS"
	for i, e := range req.Snapshot.ApprovedEvidence {
		label := strings.ToUpper(e.EpistemicType)
		if e.EpistemicType == "inference" {
			label = "AI_INFERENCE"
		}
		add(req.RemoteContext.Evidence[i].Ref, "evidence", e.ID, e.Version, label)
	}
	for i, e := range req.Snapshot.ApprovedEvents {
		add(req.RemoteContext.Events[i].Ref, "event", e.ID, e.Version, "APPROVED_CLINICAL_EVENT")
	}
	for i, h := range req.Snapshot.ApprovedHypotheses {
		ref := req.RemoteContext.Hypotheses[i].Ref
		add(ref, "hypothesis", h.ID, h.Version, "CLINICAL_HYPOTHESIS")
		profile.HypothesisRefs = append(profile.HypothesisRefs, ref)
		if h.ConfidenceLevel != "green" {
			profile.OpenQuestions = append(profile.OpenQuestions, "Explore supporting and contradicting evidence for "+ref)
		}
	}
	for _, group := range []struct {
		kind string
		refs map[string]uuid.UUID
	}{{"target", req.refs.targets}, {"goal", req.refs.goals}, {"indicator", req.refs.indicators}, {"rationale", req.refs.rationales}, {"gira", req.refs.giras}} {
		for ref, id := range group.refs {
			add(ref, group.kind, id, req.refs.versions[ref], "APPROVED_THERAPEUTIC_STRATEGY")
			if group.kind == "goal" {
				profile.GoalRefs = append(profile.GoalRefs, ref)
			}
			if group.kind == "gira" {
				profile.GIRARefs = append(profile.GIRARefs, ref)
			}
		}
	}
	sort.Strings(profile.GoalRefs)
	sort.Strings(profile.GIRARefs)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Ref < sources[j].Ref })
	artifact.RuntimeProfile = profile
	pretty, _ := json.MarshalIndent(artifact, "", "  ")
	return ProjectExport{ID: uuid.New(), TenantID: t, ClientID: c, GeneratedBy: a, GeneratedAt: time.Now().UTC(), StateVersion: state.StateVersion, Lifecycle: "generated", ContentHash: projectHash(artifact), Artifact: artifact, Markdown: "# Clinical Project Export\n\nInspect before manual transfer. Approval labels describe local sources, not future GPT output.\n\n```json\n" + string(pretty) + "\n```\n", Sources: sources, Snapshot: req.Snapshot}, nil
}

func validateProjectOwnership(v any, t, c uuid.UUID) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return err
	}
	var walk func(any) error
	walk = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for key, value := range x {
				if key == "tenant_id" && value != t.String() || key == "client_id" && value != c.String() {
					return domainerrors.NewValidation("project source ownership mismatch")
				}
				if err := walk(value); err != nil {
					return err
				}
			}
		case []any:
			for _, value := range x {
				if err := walk(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(tree)
}
func filterProjectEvents(items []Event) []Event {
	out := []Event{}
	for _, x := range items {
		if x.ApprovalStatus == "approved" {
			out = append(out, x)
		}
	}
	return out
}

func (s *Service) ExportProject(ctx context.Context, t, c, a, p uuid.UUID) (ProjectExport, error) {
	if err := s.require(ctx, t, a, c, "treating"); err != nil {
		return ProjectExport{}, err
	}
	repo, ok := s.repo.(ProjectRepository)
	if !ok {
		return ProjectExport{}, domainerrors.ErrConflict
	}
	state, err := s.repo.State(ctx, t, c)
	if err != nil {
		return ProjectExport{}, err
	}
	approaches, err := s.repo.ListApproaches(ctx)
	if err != nil {
		return ProjectExport{}, err
	}
	techniques, err := s.repo.ListTechniques(ctx)
	if err != nil {
		return ProjectExport{}, err
	}
	out, err := BuildProjectExport(t, c, a, state, p, approaches, techniques)
	if err != nil {
		return out, err
	}
	if s.consent != nil {
		if _, err = s.consent.Authorize(ctx, t, c, consent.ExternalManual, "project_export", out.ID); err != nil {
			return ProjectExport{}, err
		}
	}
	return repo.SaveProjectExport(ctx, out)
}
func (s *Service) GetProjectExport(ctx context.Context, t, c, a, id uuid.UUID) (ProjectExport, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return ProjectExport{}, err
	}
	repo, ok := s.repo.(ProjectRepository)
	if !ok {
		return ProjectExport{}, domainerrors.ErrConflict
	}
	if s.consent != nil {
		if _, err := s.consent.Authorize(ctx, t, c, consent.ExternalManual, "project_export_download", id); err != nil {
			return ProjectExport{}, err
		}
	}
	return repo.GetProjectExport(ctx, t, c, id)
}
func (s *Service) GetProjectProposal(ctx context.Context, t, c, a, id uuid.UUID) (ProjectImportRecord, error) {
	if err := s.require(ctx, t, a, c, "treating", "supervisor"); err != nil {
		return ProjectImportRecord{}, err
	}
	repo, ok := s.repo.(ProjectRepository)
	if !ok {
		return ProjectImportRecord{}, domainerrors.ErrConflict
	}
	return repo.GetProjectProposal(ctx, t, c, id)
}
func (s *Service) ImportProject(ctx context.Context, t, c, a uuid.UUID, raw []byte) (Diff, error) {
	if err := s.require(ctx, t, a, c, "treating"); err != nil {
		return Diff{}, err
	}
	proposal, err := DecodeProjectImport(raw)
	if err != nil {
		return Diff{}, err
	}
	repo, ok := s.repo.(ProjectRepository)
	if !ok {
		return Diff{}, domainerrors.ErrConflict
	}
	export, err := repo.GetProjectExport(ctx, t, c, proposal.SourceExportID)
	if err != nil {
		return Diff{}, err
	}
	state, err := s.repo.State(ctx, t, c)
	if err != nil {
		return Diff{}, err
	}
	result, err := ValidateProjectImport(export, state, proposal)
	if err != nil {
		return Diff{}, err
	}
	record := ProjectImportRecord{ID: uuid.New(), TenantID: t, ClientID: c, ExportID: export.ID, ImportedBy: a, ContentHash: projectHash(proposal), Proposal: proposal}
	ops := []Operation{}
	for _, op := range result.Operations {
		ops = append(ops, Operation{OperationType: op.OperationType, TargetEntityID: *op.TargetEntityID, ExpectedEntityVersion: op.ExpectedEntityVersion, OriginalProposal: op.Proposal})
	}
	return repo.ImportProjectProposal(ctx, record, CreateDiffInput{TenantID: t, ClientID: c, ActorID: a, ExternalProposalID: record.ID, BaseStateVersion: export.StateVersion, Operations: ops, Uncertainties: result.Uncertainties, OutputHash: record.ContentHash})
}

func DecodeProjectImport(raw []byte) (ClinicalProjectImportV1, error) {
	var p ClinicalProjectImportV1
	if len(raw) > 1024*1024 {
		return p, domainerrors.NewValidation("manual import exceeds 1 MiB")
	}
	if err := rejectProjectDuplicateKeys(raw); err != nil {
		return p, err
	}
	if err := decodeStrict(raw, &p); err != nil {
		return p, err
	}
	if p.SchemaVersion != ProjectImportVersion || p.SourceExportID == uuid.Nil || len(p.SourceExportHash) != 64 || p.Provenance.SourceType != "manual_external_ai" || p.Provenance.Assertion != "user_supplied" {
		return p, domainerrors.NewValidation("invalid manual import schema or provenance")
	}
	if len(p.Operations) > 100 || len(p.OpenQuestions) > 100 || len(p.SupervisionObservations) > 100 {
		return p, domainerrors.NewValidation("manual import collection limit exceeded")
	}
	for _, v := range []string{p.Provenance.ProviderName, p.Provenance.ModelName, p.Provenance.Surface, p.Provenance.ProjectContextVersion} {
		if len(v) > 200 {
			return p, domainerrors.NewValidation("provenance metadata exceeds 200 bytes")
		}
	}
	return p, nil
}

// Reject ambiguous JSON before either Go's struct decoder or PostgreSQL JSONB
// can discard duplicate properties. This includes nested portable proposals.
func rejectProjectDuplicateKeys(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				token, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate property")
				}
				seen[key] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := value(); err != nil {
		return domainerrors.NewValidation("invalid or ambiguous manual import JSON")
	}
	if _, err := d.Token(); err != io.EOF {
		return domainerrors.NewValidation("manual import contains trailing data")
	}
	return nil
}

func ValidateProjectImport(export ProjectExport, state State, p ClinicalProjectImportV1) (InterpreterResult, error) {
	out := InterpreterResult{Operations: []ProposedOperation{}, Uncertainties: []Uncertainty{}}
	if state.ClientID != export.ClientID {
		return out, domainerrors.ErrNotFound
	}
	if p.SourceExportID != export.ID || p.SourceExportHash != export.ContentHash || state.StateVersion != export.StateVersion {
		return out, domainerrors.ErrConflict
	}
	refs := map[string]ProjectSource{}
	for _, source := range export.Sources {
		refs[source.Ref] = source
	}
	seen := map[string]bool{}
	for _, op := range p.Operations {
		if !oneOf(op.OperationType, "create_process", "update_process", "create_hypothesis", "update_hypothesis", "strengthen_hypothesis", "weaken_hypothesis", "retire_hypothesis", "link_supporting_evidence", "link_contradicting_evidence") {
			return out, domainerrors.NewValidation("operation is not permitted for external AI")
		}
		if !semanticRefPattern.MatchString(op.ID) || seen[op.ID] || blank(op.Rationale) || len(op.SourceRefs) == 0 {
			return out, domainerrors.NewValidation("operation requires unique id, rationale and source refs")
		}
		seen[op.ID] = true
		for _, ref := range op.SourceRefs {
			if _, ok := refs[ref]; !ok {
				return out, domainerrors.NewValidation("unknown source ref")
			}
		}
		source, exists := refs[op.TargetRef]
		if strings.HasPrefix(op.OperationType, "create_") {
			if exists || !semanticRefPattern.MatchString(op.TargetRef) || op.ExpectedEntityVersion != nil {
				return out, domainerrors.NewValidation("invalid new target ref")
			}
			source = ProjectSource{Ref: op.TargetRef, EntityType: strings.TrimPrefix(op.OperationType, "create_"), EntityID: uuid.New(), Version: 1}
		} else if !exists || op.ExpectedEntityVersion == nil || *op.ExpectedEntityVersion != source.Version {
			return out, domainerrors.ErrConflict
		}
		var payload any
		if err := json.Unmarshal(op.Proposal, &payload); err != nil {
			return out, domainerrors.NewValidation("invalid portable proposal")
		}
		resolved, err := resolveProjectPayload(payload, refs)
		if err != nil {
			return out, err
		}
		raw, _ := json.Marshal(resolved)
		if err := ValidateProposal(op.OperationType, raw); err != nil {
			return out, err
		}
		id := source.EntityID
		out.Operations = append(out.Operations, ProposedOperation{ID: op.ID, OperationType: op.OperationType, TargetEntityID: &id, ExpectedEntityVersion: op.ExpectedEntityVersion, Proposal: raw})
		refs[op.TargetRef] = source
		if !strings.HasPrefix(op.OperationType, "create_") {
			source.Version++
			refs[op.TargetRef] = source
		}
	}
	if err := ValidateInterpreterReferences(InterpreterInput{CurrentState: state}, out); err != nil {
		return out, err
	}
	req := BuildGIRAGenerationRequest(export.Snapshot)
	if p.Strategy != nil {
		b, _ := json.Marshal(p.Strategy)
		semantic, err := DecodeGIRASemanticProposal(b)
		if err != nil {
			return out, err
		}
		strategy, err := CompileGIRASemanticProposal(req, semantic, uuid.New)
		if err != nil {
			return out, err
		}
		out.Operations = append(out.Operations, strategy.Operations...)
		out.Uncertainties = append(out.Uncertainties, strategy.Uncertainties...)
	}
	for _, q := range p.OpenQuestions {
		if !oneOf(q.Type, "explore", "insufficient_evidence", "open_question") || blank(q.Question) {
			return out, domainerrors.NewValidation("invalid open question")
		}
		ids := []uuid.UUID{}
		for _, ref := range q.EvidenceRefs {
			source, ok := refs[ref]
			if !ok || source.EntityType != "evidence" {
				return out, domainerrors.NewValidation("unknown question evidence ref")
			}
			ids = append(ids, source.EntityID)
		}
		out.Uncertainties = append(out.Uncertainties, Uncertainty{Type: q.Type, Question: q.Question, EvidenceIDs: ids})
	}
	if len(out.Operations) == 0 {
		return out, domainerrors.NewValidation("at least one clinical proposal operation is required")
	}
	return out, nil
}

// Only explicit portable references are accepted. UUID fields cannot bypass export membership.
func resolveProjectPayload(v any, refs map[string]ProjectSource) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range x {
			if strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids") {
				return nil, domainerrors.NewValidation("use portable refs, not internal IDs")
			}
			target := key
			switch {
			case strings.HasSuffix(key, "_ref"):
				target = strings.TrimSuffix(key, "_ref") + "_id"
				if value == nil {
					out[target] = nil
					continue
				}
				ref, ok := value.(string)
				source, exists := refs[ref]
				if !ok || !exists {
					return nil, domainerrors.NewValidation("unresolved reference")
				}
				out[target] = source.EntityID
			case strings.HasSuffix(key, "_refs"):
				target = strings.TrimSuffix(key, "_refs") + "_ids"
				values, ok := value.([]any)
				if !ok {
					return nil, domainerrors.NewValidation("refs must be arrays")
				}
				ids := []uuid.UUID{}
				for _, v := range values {
					ref, ok := v.(string)
					source, exists := refs[ref]
					if !ok || !exists {
						return nil, domainerrors.NewValidation("unresolved reference")
					}
					ids = append(ids, source.EntityID)
				}
				out[target] = ids
			default:
				resolved, err := resolveProjectPayload(value, refs)
				if err != nil {
					return nil, err
				}
				out[target] = resolved
			}
		}
		return out, nil
	case []any:
		out := []any{}
		for _, item := range x {
			resolved, err := resolveProjectPayload(item, refs)
			if err != nil {
				return nil, err
			}
			out = append(out, resolved)
		}
		return out, nil
	default:
		return v, nil
	}
}

// PortableProjectReceipt contains only export metadata suitable for manual copy alongside the artifact.
func (e ProjectExport) PortableProjectReceipt() string {
	return fmt.Sprintf("Export ID: %s\nExport hash: %s\nBase state version: %d\n", e.ID, e.ContentHash, e.StateVersion)
}
