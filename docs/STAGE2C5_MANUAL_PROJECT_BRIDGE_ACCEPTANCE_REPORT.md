# STAGE 2C.5 — MANUAL GPT PROJECT BRIDGE & FINAL STAGE 2C ACCEPTANCE REPORT

Date: 2026-09-08. Scope: backend manual bridge, architecture rebaseline, synthetic acceptance and regression. No real patient data was used in tests or transmitted externally.

## 1. Verdict

```text
STAGE 2C = VERIFIED PASS
STAGE 2C.5 = VERIFIED PASS
GIRA GENERATION MODE = MANUAL GPT PROJECT TRIANGULATION
AUTOMATED LOCAL GIRA MODEL = NOT REQUIRED
AUTOMATED REMOTE GIRA API = DISABLED BY DEFAULT / NOT REQUIRED
STAGE 2D READINESS = READY
```

This verdict applies to the selected human-mediated architecture and the V1 scope described below. It does not certify any automated GIRA model or claim that external clinical narrative is anonymous. Stage 2D was not started.

## 2. Architecture rebaseline

The therapist bridges the local clinical system and a patient-specific GPT Project. The backend generates a local preview and accepts manually supplied structured output. Live/review/longitudinal retain their existing local Ollama paths.

```text
approved patient/process state
  → local minimized export and preview
  → THERAPIST transfers selected artifact to GPT Project
  → THERAPIST inspects external output
  → manual strict import
  → ClinicalDiff pending_review
  → granular Human Review
  → transactional Human Merge
```

No upload, download, Project synchronization, webhook, API inference or background OpenAI job is part of this workflow. Prior model limitations remain documented but do not block this architecture.

## 3. ADR

`ADR_MANUAL_CLINICAL_PROJECT_BRIDGE.md` records the accepted product decision, semantic provenance choice, default privacy boundary, concurrency model, dormant infrastructure and future Project organization. `CLINICAL_PROJECT_BRIDGE_CONTRACT_V1.md` provides endpoint details, a synthetic import example, errors and deployment requirements.

## 4. Export contract

Implemented `ProjectExport` and `ProjectArtifact`, schema `clinical-project-export-v1`. The request selects one approved process under an explicitly scoped patient. Default output is minimized, with structured JSON and a deterministic human-readable Markdown preview. It includes approved selected events, their linked active evidence, approved non-retired/non-superseded hypotheses and approved therapeutic strategy.

Unrelated processes, pending/unmerged proposals, rejected hypotheses and raw model interpretations are excluded. Nothing automatically selects reports or the entire clinical history. Optional fuller context is not enabled in V1.

Local provenance stores export ID, tenant, client, generating user, generation time, state version, artifact hash, included source IDs/versions and canonical snapshot. Export/provenance updates and hard deletion are prohibited. The lifecycle implemented is `generated`; manual `marked_as_used` is not exposed.

Hash and Markdown stability are tested against the same selected snapshot, including addition of unrelated/rejected material. Receipt ID/time are outside the content hash. The API's local metadata is distinct from the portable artifact.

## 5. CASE_RUNTIME_PROFILE

Implemented a compact selected-process runtime profile: process reference/status, hypothesis references, goal references, current GIRA references and follow-up questions for non-green hypotheses. The detailed evidence stays in the source section. Missing/unselected formulation, session reports, alliance notes and therapist patterns are explicitly named rather than synthesized.

The V1 profile is process-scoped; it is not an automatically assembled whole-patient formulation. Report selection and a broader profile remain future enhancements permitted by the specification's optional source scope.

## 6. Portable references

Portable aliases include `process_1`, `evidence_1`, `hypothesis_1`, `existing_target_1`, `existing_goal_1`, indicators, rationales and GIRAs. They are deterministic within the selected export. Database entity UUIDs stay in the local source mapping, which is not serialized into the artifact.

PostgreSQL validates source identity against a closed table allow-list with exact tenant/client/entity/version. Alias resolution is exact, with no textual or fuzzy matching. The opaque export receipt itself may be transferred to associate a later import with the correct local artifact.

## 7. Manual GPT output contract

Implemented `ClinicalProjectImportV1`, with export ID/hash, user-supplied provenance, ordered portable core operations, optional semantic strategy, open questions and therapist supervision observations.

Core operations support process creation/update and hypothesis creation/update/strengthen/weaken/retire/supporting/contradicting links. Strategy proposals reuse the existing semantic target/goal/indicator/rationale/GIRA/phase/link contracts and frozen deterministic compiler. A revision references the previous GIRA rather than overwriting it.

Input is bounded to 1 MiB. Core operations/questions/supervision arrays are limited to 100 each, and optional provenance values to 200 bytes. Unknown fields, duplicate JSON properties, trailing values, invalid schemas and internal UUID payload keys are rejected. At least one valid clinical operation is required; standalone notes cannot create an empty diff. Full strategy schema is embedded in OpenAPI and reproducible with the local schema-printing command.

## 8. Import pipeline

Implemented: existing CA-W treating access → strict decoder → tenant/client-scoped export lookup → state/hash validation → exact reference resolution → typed longitudinal validation or semantic strategy compiler/validator → atomic external provenance plus pending ClinicalDiff.

The compiler reserves new UUIDs locally. Existing entities require expected versions; dependency order is validated. Unsafe goal achievement, phase completion, evidence rewriting, unknown operations, unsupported registry mechanisms and GIRA overwrite attempts fail before a manual diff is persisted.

A GIRA in V1 targets the selected approved process. A newly proposed process must first be merged and exported before building its GIRA. Questions/supervision accompanying operations remain external proposal content; they do not become patient evidence.

## 9. External provenance model

Selected dedicated `clinical_external_proposals`, not an extended/fabricated AIRun. Required semantics are `source_type=manual_external_ai` and `provenance_assertion=user_supplied`. Provider/model/surface/context-version values are optional therapist assertions.

The immutable record preserves the portable original, including source refs, rationale, uncertainty and supervision observations. `ClinicalDiff.source_external_proposal_id` points to it; the schema forbids simultaneous AI-run and external-proposal provenance. Normal manual import never invokes a provider or creates a `ClinicalAIRun`.

Provenance reads use existing clinical CA-R authorization. Audit emits export-generated and external-proposal-imported events with only IDs, state versions, source/operation counts and hashes. Clinical text is retained only in the intended clinical artifacts, not audit metadata.

## 10. ClinicalDiff integration

The existing diff transaction helper now supports automated run provenance or external provenance. Manual import wraps external-record creation and the same diff/operation/audit insertion helper in one transaction. Existing granular review, separate human modifications, partial review, approval/rejection, optimistic concurrency, transactional merge and merge idempotency remain in use.

The diff's original operation is the resolved/compiled typed proposal. The complete pre-resolution external proposal remains separately retrievable. Human changes do not overwrite either original. No parallel approval workflow or longitudinal/strategy CRUD was introduced.

## 11. Stale-state protection

Import checks source export ID/hash, global patient revision and exported expected entity versions. It rechecks the patient head and export under the same tenant/patient advisory lock used for merge before saving anything. Stale imports return conflict. Existing stale-diff and entity-version validation still applies during merge.

Tested export at v5 and reimport after merge to v6: rejected with conflict. Cross-patient or cross-tenant export/provenance lookups return not found. Duplicate export/hash imports cannot create duplicate external records. Merge repetition returns the original stored v6 result.

## 12. Privacy/minimization

The bridge reuses local context selection, minimization and aliases. Synthetic names, email, phone, address and document identifiers were tested for removal. Unrelated process material and internal clinical UUIDs are absent from the portable artifact.

Minimization is not anonymization. Narrative detail can remain identifying and pattern-based redaction can miss names or identifiers. The therapist must inspect the generated preview before transferring it. Manual copy/paste still involves external processing when performed. Fuller context and durable external-processing consent require later product/privacy integration; local consent must not silently imply external or audio/transcription consent.

The backend provides the preview/read endpoints now. Full copy/download/import UI is deferred as requested. No real clinical content was sent to a remote service.

## 13. E2E triangulation

`TestManualProjectTriangulationNoDirectMergeAndReverseProvenancePostgres` passed against isolated PostgreSQL 17. The scenario creates P1, E1/E2 and H1 through existing merge helpers and reaches state v5. It exports those four source identities, imports a synthetic semantic GIRA, reviews all operations and merges to v6.

The reverse join was verified:

```text
new GIRA
  → longitudinal transition
  → ClinicalDiff
  → manual external proposal
  → export X
  → process/evidence/hypothesis source refs and versions
```

This is synthetic workflow acceptance, not model certification. No claim is made that ChatGPT itself produced the fixture.

## 14. No-AI-Direct-Merge

The central PostgreSQL test checks clinical counts/versions, links, transition history and patient revision before/after every boundary; GIRA count remains zero until merge.

```text
EXPORT → unchanged (v5)
IMPORT → unchanged (v5), ClinicalDiff pending_review only
REVIEW → unchanged (v5), after every operation decision
MERGE  → changed (v6), GIRA created by human-reviewed transaction
REPEAT MERGE → unchanged (v6), stored idempotent result
```

AIRun count before/after manual import is identical. Invalid imports create neither an external proposal nor a manual diff. Existing actor and CA-W merge guards remain covered by regression.

## 15. Tenant/client isolation

PASS: nested foreign tenant/client sources rejected during export construction; exact export/provenance lookup predicates; existing clinical access checks; composite FKs between export, external proposal and diff; insert-time source identity/version validation; foreign source mapping rejection; immutable provenance and no hard deletion.

HTTP tests cover permission denials, strict request parsing and body bounds; route tests verify all four endpoints. PostgreSQL tests cover positive manual workflow and foreign lookup/source attacks. The existing repository tenant query guard passes with the new repository methods.

## 16. C1–C25

All deterministic Stage 2C invariants remain PASS in unit/service/PostgreSQL regression. The new entry path reuses these validators and the same merge implementation.

| Invariant | Result | Evidence |
| --- | --- | --- |
| C1 Process differs from Target | PASS | Typed contracts and strategy schema/FKs |
| C2 Target differs from Goal | PASS | Distinct payloads and goal-target relations |
| C3 Goal differs from Indicator | PASS | Indicator ownership and validation |
| C4 Hypothesis differs from Rationale | PASS | Separate core/strategy epistemic types |
| C5 Formulation differs from Strategy | PASS | Separate state projection; no formulation mutation |
| C6 Approach differs from Technique | PASS | Versioned registries and compatibility checks |
| C7 Approach selection requires rationale | PASS | Semantic compiler and GIRA validation |
| C8 Rationale requires Target + Goal | PASS | Chain validation and PostgreSQL tests |
| C9 Technique/approach version compatibility | PASS | Exact registry checks and FK tests |
| C10 No unjustified eclecticism | PASS | Existing semantic same-target guard and fixtures |
| C11 Multiple approaches are optional | PASS | Existing A/C deterministic fixtures |
| C12 Unsupported mechanisms blocked | PASS | Existing safeguards and new manual rejection case |
| C13 AI Target is only a proposal | PASS | Manual import creates diff only |
| C14 AI Goal is not active approved state | PASS | Compiler proposals and human merge lifecycle |
| C15 AI cannot achieve Goal | PASS | Manual forbidden-operation and existing tests |
| C16 AI GIRA is not approved GIRA | PASS | Zero GIRA count before human merge |
| C17 AI cannot complete Phase | PASS | Strict semantic schema and forbidden-operation tests |
| C18 Historical GIRA is preserved | PASS | Existing supersession tests; overwrite rejected |
| C19 Deprecated registry versions remain resolvable | PASS | Existing historical registry PostgreSQL tests |
| C20 Evidence does not auto-change Goal | PASS | Existing progress/regression link fixtures |
| C21 Review does not mutate strategy | PASS | Central test checks every decision |
| C22 Human Merge required | PASS | Shared review/merge service, no direct entity write path |
| C23 Merge is transactional | PASS | Existing rollback suite and shared transaction |
| C24 Stale state cannot overwrite | PASS | Export/import check, patient lock and merge versions |
| C25 No AI direct merge | PASS | Central manual E2E and existing actor/access tests |

## 17. Regression

Final execution on 2026-09-08:

| Gate | Result |
| --- | --- |
| `go test -count=1 ./...` | PASS 286 / FAIL 0 / SKIP 46 |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| PostgreSQL opt-in HTTP/handlers/middleware + DB | PASS 114 / FAIL 0 / SKIP 0 |
| Ollama live, existing fictitious fixture | PASS; 50.49 s, no repair |
| Ollama review, existing fictitious fixture | PASS; 70.42 s, no repair |
| Ollama longitudinal A–G | PASS 7/7; 354.80 s total |
| Web lint | PASS |
| Web production build | PASS; 13 generated pages |
| OpenAPI | PASS; strict YAML parse, embedded schemas and all references resolve |
| Tenant query guard | PASS in Go/PostgreSQL suites |
| Migration roundtrip | PASS: actual SQL 16 → 24 → 16 → 24; zero checked residual bridge/strategy/diff objects and bridge functions at 16 |
| Git diff whitespace check | PASS |
| Automated remote GIRA certification | NOT REQUIRED BY SELECTED ARCHITECTURE; not executed |

Counts include Go named subtests and are not directly comparable to reports counting only top-level tests. Standard-run skips are opt-in suites; relevant PostgreSQL and local Ollama gates were executed explicitly. GIRA model benchmark/remote certification skips are not relabeled as executed passes.

The project-configured database endpoint was unavailable. Testing used an isolated temporary PostgreSQL 17 cluster on loopback port 55435 with synthetic databases `stage2c5_test` and `stage2c5_roundtrip`. The product database and its credentials/configuration were not changed. Migration roundtrip executed the actual ordered SQL files in per-file transactions; it did not use a migrate CLI version tracker. Temporary test cluster is stopped after verification and retained locally for inspection.

Git inventory (relative paths; the original dirty tree was preserved):

**PREEXISTING:** all previously dirty Stage 2B/2C core, strategy, Ollama, remote provider/certification, metrics, migration 22/23 and prior report files. In particular, `internal/infra/remotegira`, `internal/certification`, strategy implementation/tests and prior clinical prompts were retained. Migration 24 existed as an unfinished draft when work resumed.

**MODIFIED in this stage:**

- `apps/api/cmd/server/main.go`: additional explicit experimental guard on dormant remote composition.
- `apps/api/internal/config/config.go`, `config_test.go`: experimental flag/default safety, retained prior config contracts.
- `apps/api/internal/http/server.go`: four manual bridge routes on existing middleware.
- `apps/api/internal/infra/db/clinical_longitudinal_repository.go`: shared diff insertion and external provenance read.
- `apps/api/internal/usecase/longitudinal/types.go`: optional external provenance on Diff/input.
- `apps/api/migrations/000024_clinical_project_bridge.up.sql` and `.down.sql`: completed draft schema, ownership/version validation and immutable provenance.
- `docs/openapi.yaml`: manual bridge endpoints/contracts and external diff provenance.

**NEW in this stage:**

- `apps/api/cmd/clinical-project-schema/main.go`.
- `apps/api/internal/config/project_bridge_test.go`.
- `apps/api/internal/http/clinical_project_routes_test.go`.
- `apps/api/internal/http/handlers/clinical_project.go`, `clinical_project_test.go`, `clinical_project_openapi_test.go`.
- `apps/api/internal/infra/db/clinical_project_repository.go`, `clinical_project_postgres_integration_test.go`.
- `apps/api/internal/usecase/longitudinal/project_bridge.go`, `project_bridge_test.go`.
- `docs/ADR_MANUAL_CLINICAL_PROJECT_BRIDGE.md`, `CLINICAL_PROJECT_BRIDGE_CONTRACT_V1.md`, this report.

No commit, reset, destructive revert, live database migration, clinical skill change or GPT Project source change was performed.

## 18. Dormant API infrastructure

Stage 2C.4A/4B code is retained, tested and documented as DORMANT / EXPERIMENTAL / NOT SELECTED PRODUCT PATH. Default provider remains `none`; remote activation additionally requires `EXPERIMENTAL_REMOTE_GIRA=true`. Normal manual bridge operation does not need an API credential.

Remote certification was not run. Its absence is not a product-path blocker. No weak-model fallback was added and live/review/longitudinal prompts were not edited during this stage.

## 19. Remaining debt

No blocking acceptance defect remains in the implemented V1 backend bridge.

Non-blocking/product integration work:

- Build the therapist-facing preview/copy/download/import UI; current support is backend-first.
- Decide and implement durable consent for human-mediated external processing separately from local processing and audio/transcription.
- Add explicit optional approved-report/formulation selection and fuller runtime sections only after scope/privacy decisions; no whole-history export by default.
- Consider standalone supervision/question storage only with a defined review lifecycle; V1 requires at least one clinical operation per imported diff.
- Broaden strategy construction to newly proposed processes only through a deliberately specified dependency flow; V1 uses an approved selected process.
- `marked_as_used` is not exposed; provenance is immutable in V1.
- Define final patient Project source distribution/versioning without overwriting existing skills or patient sources.

Deployment requirement: apply migration 24 through the normal deployment process before starting this backend version. This report does not imply it has been applied to the user's development/product database.

## 20. Stage 2D readiness

```text
READY
```

The deterministic Stage 2C core and manual bridge pass their acceptance gates. High-capability reasoning is human-mediated; automated GIRA model/API certification is not required. Future Stage 2D must carry forward explicit privacy/consent, preview, source versioning and human review/merge guarantees. Stage 2D, audio/transcription persistence, jobs, synchronization, RAG and embeddings were not started.

STOP after this Stage 2C.5 report.
