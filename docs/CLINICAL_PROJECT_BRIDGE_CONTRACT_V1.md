# Manual Clinical Project Bridge — V1 contract

The backend exposes a local preview/export and strict manual import. All routes use existing authentication, tenant middleware and clinical relationship permissions. There is no automated connection to ChatGPT.

## Endpoints

| Method/path | Access | Result |
| --- | --- | --- |
| POST `/api/v1/clients/:id/project-exports` | treating | 201 local export and preview |
| GET `/api/v1/clients/:id/project-exports/:export_id` | treating/supervisor | 200 stored export |
| POST `/api/v1/clients/:id/project-imports` | treating | 201 `ClinicalDiff pending_review` |
| GET `/api/v1/clients/:id/project-proposals/:proposal_id` | treating/supervisor | 200 original external proposal/provenance |

Export body: `{"process_id":"<approved-process-uuid>"}`. The response includes local tenant/client/user metadata, export ID, generation time, state version, content hash, artifact and Markdown. Transfer only the inspected artifact/Markdown plus its export ID/hash receipt. Backend-only source mappings and the canonical local snapshot are not serialized into the portable artifact.

The export ID is an opaque receipt identifying this export, not a patient UUID. `process_1`, `evidence_1`, `hypothesis_1`, `existing_goal_1` and related aliases are stable for the same deterministic selected snapshot. They are scoped to an export; never reuse them with another export. Order follows deterministic timestamp/UUID ordering and existing canonical strategy ordering. Hash is SHA-256 over Go's canonical JSON serialization of `artifact` (sorted map keys); timestamps and receipt IDs are excluded so the same selected content/version has the same hash. Do not independently hash pretty-printed Markdown.

Default privacy mode is `minimized`. Every evidence retains its epistemic type; inference is additionally labeled `AI_INFERENCE`, and hypotheses `CLINICAL_HYPOTHESIS`. Approval describes local review status, not certainty. Hypotheses retain confidence and supporting/contradicting references. Source excerpts may be minimized; the original database statements remain unchanged.

The runtime profile contains selected process/status, hypothesis/goal/GIRA references and exploratory follow-up questions for non-green hypotheses. Unselected formulation/reports and unavailable alliance/pattern sections are named rather than fabricated. Export does not include pending diffs, raw model interpretations or unrelated patient processes.

## Manual import envelope

Synthetic example; replace receipt fields with those from the generated export:

```json
{
  "schema_version": "clinical-project-import-v1",
  "source_export_id": "11111111-1111-4111-8111-111111111111",
  "source_export_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "provenance": {
    "source_type": "manual_external_ai",
    "provenance_assertion": "user_supplied",
    "provider_name": "therapist-supplied",
    "model_name": "therapist-supplied",
    "surface": "ChatGPT Project"
  },
  "operations": [
    {
      "id": "proposal_1",
      "operation_type": "create_hypothesis",
      "target_ref": "new_hypothesis_1",
      "expected_entity_version": null,
      "source_refs": ["evidence_1"],
      "rationale": "Synthetic alternative interpretation requiring human review.",
      "uncertainty": "The available observations do not establish a unique explanation.",
      "proposal": {
        "process_ref": "process_1",
        "statement": "Possible avoidance pattern in the selected context.",
        "hypothesis_type": null,
        "confidence_level": "yellow",
        "supporting_evidence_refs": ["evidence_1"],
        "contradicting_evidence_refs": []
      }
    }
  ],
  "open_questions": [
    {"type": "explore", "question": "Which competing explanation fits the observations?", "evidence_refs": ["evidence_1"]}
  ],
  "supervision_observations": ["Synthetic observation for therapist review; not patient evidence."]
}
```

Core operation types: `create_process`, `update_process`, `create_hypothesis`, `update_hypothesis`, `strengthen_hypothesis`, `weaken_hypothesis`, `retire_hypothesis`, `link_supporting_evidence`, `link_contradicting_evidence`.

Their `proposal` is the existing strict longitudinal payload with `_ref`/`_refs` replacing `_id`/`_ids`. Internal UUID payload keys are rejected. Existing targets require their expected version; creates require a new lowercase portable reference and null/absent expected version. New references may be used only after their creation in operation order. Supporting and contradicting evidence cannot be invented or rewritten.

Optional `strategy` uses the frozen semantic GIRA contract: `targets`, `goals`, `indicators`, `rationales`, `gira`, `phases`, `indicator_links`, `uncertainties`. Its full machine-readable schema is embedded as `ClinicalProjectStrategy` in `openapi.yaml`; reproduce that schema locally with `go run ./cmd/clinical-project-schema` from `apps/api`. It compiles through the existing deterministic compiler and typed GIRA validator. A revision specifies `supersedes_gira_ref`; the backend derives the next version and preserves the previous GIRA. No AI goal achievement, phase completion or historical overwrite operation is allowed.

At least one valid clinical operation must result from `operations` or `strategy`. Questions and supervision notes can accompany it and remain in the immutable external proposal. They do not mutate the clinical state or become factual evidence. Standalone note-only imports are rejected, preventing an empty diff with no decidable operations.

## Validation and errors

Input is bounded to 1 MiB. Unknown fields, duplicate JSON keys at any depth, trailing JSON and invalid typed values are rejected. Core operations, questions and supervision collections are limited to 100 each; optional provenance metadata is limited to 200 bytes per value. Exact source references and exported entity versions are required. No fuzzy matching occurs.

| Status | Meaning |
| --- | --- |
| 400 | Invalid JSON/schema, unsafe operation or unresolved reference |
| 401 | Authentication missing |
| 403 | Existing clinical relationship permission denied |
| 404 | Selected approved process/export/provenance absent within this tenant/patient |
| 409 | Stale state/version, wrong source hash or duplicate import |

After a successful import, use the existing operation decision endpoint and merge endpoint with `expected_diff_revision`. Each operation is reviewed once; partial review/approval/rejection/modification behavior is unchanged. Export/import/review do not mutate longitudinal or therapeutic entities. Merge applies approved/modified operations in one transaction and advances the patient revision once. A repeated merge returns the stored result.

## Provenance and deployment

Migration 24 creates exports, exact source mappings and external proposals. Provenance is immutable and protected against hard deletion. A source mapping must identify an existing entity with the same tenant/client/version. The external proposal and diff are persisted atomically under the patient lock; no fake `ClinicalAIRun` is created. Audit stores only IDs, counts, versions and hashes.

Apply migration 24 before deploying the changed backend. It extends `ClinicalDiff` with `source_external_proposal_id`. Use the existing migration deployment procedure; this stage tested migrations only on isolated synthetic databases.

Human preview is provided by the backend response and read endpoint; the complete copy/download/import UI is deferred. No current GPT Project source or clinical skill is edited. Fuller export policy, broader source selection and durable external-processing consent require later product/privacy integration.
