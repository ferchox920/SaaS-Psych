# ADR — High-Capability Clinical Reasoning Integration

Status: ACCEPTED — Stage 2C.5, 2026-09-08.

## Implementation update — 2026-09-26

The manual bridge is now available in the patient-specific **GPT Project** tab. A treating professional can select an approved process, inspect the minimized preview, copy or download the export, and import a therapist-supplied structured proposal. The imported proposal remains a pending `ClinicalDiff`; operation decisions and merge still require explicit human action. Browser tests cover preview, import, consent and failure paths. This is not an automatic ChatGPT integration or a claim that minimization anonymizes clinical text.

External manual processing has its own durable `EXTERNAL_MANUAL_AI_PROCESSING` consent, separate from local AI consent. The backend rechecks authorization and consent when an export is read. Organization-specific privacy approval and the professional's preview remain necessary before any manual transfer. The sections below record the original decision and scope at adoption; statements about deferred implementation are historical.

## Decision

High-capability GPT reasoning is HUMAN-MEDIATED through patient-specific ChatGPT Projects. Automated remote GIRA API execution is NOT a requirement of the current product architecture.

The therapist selects a patient's approved process, generates and inspects a local minimized export, manually transfers the chosen artifact, reviews external output, and manually imports a structured proposal. The backend validates that proposal and creates a `ClinicalDiff`. Existing granular Human Review and transactional Human Merge remain mandatory.

```text
LOCAL approved patient/process state
  → local selection, minimization and portable aliases
  → export preview
  → THERAPIST manually copies to patient-specific GPT Project
  → THERAPIST reviews and copies structured output
  → local strict import, reference and version validation
  → ClinicalDiff pending_review
  → existing Human Review
  → existing LOCAL Human Merge
```

Local Ollama retains its constrained live, review and longitudinal roles. The findings that local models were unsuitable for automated GIRA remain valid; automated GIRA certification is no longer a product acceptance prerequisite.

## Provenance choice

Choose a dedicated `clinical_external_proposals` aggregate, linked to `clinical_project_exports` and `clinical_diffs.source_external_proposal_id`. A manual interaction is not a backend inference run. No `ClinicalAIRun` is created by import. `source_type=manual_external_ai` and `provenance_assertion=user_supplied` are explicit. Optional provider/model/surface/project-context metadata is supplied by the therapist and is not verified by the SaaS.

The original portable proposal, rationale, uncertainty and supervision observations are retained locally and immutably. The diff's original operation is the strictly validated UUID-resolved/compiled proposal; a human modification remains in the existing separate field. The original external text is never reclassified as patient evidence.

## V1 scope

One explicitly selected approved process per export. Export includes its selected approved events, linked active evidence, non-retired approved hypotheses, and approved therapeutic strategy. Context bounds and minimization reuse tested local utilities. The compact runtime profile references the same sources and reports unselected/unavailable sections explicitly.

Formulation, recent reports, alliance notes and therapist patterns are not selected automatically. This initial contract does not enable fuller clinical history export. Core process/hypothesis proposals use portable typed operations; strategy proposals use the existing `semantic_v1` representation and deterministic compiler. A GIRA is built against the selected approved process; proposing a new process and then building its GIRA requires merging that process and generating a new export.

Questions/supervision accompany a proposal with at least one clinical operation. Standalone notes do not create an empty, unreviewable diff. No separate approval workflow is introduced.

## Concurrency and isolation

The export records the global patient state revision, entity IDs/versions, stable aliases and a deterministic artifact hash. The source mapping remains local. Import requires the source export ID/hash and resolves exact aliases without fuzzy matching. A stale export is rejected with 409. Persistence rechecks the head under the same patient advisory lock as merge, eliminating the gap between service validation and persistence. Existing entity-version and stale-diff protections still apply at merge.

Tenant/client FKs, source ownership/version checks, clinical access authorization and immutable provenance prevent cross-patient source reuse. An identical reimport returns conflict rather than creating a duplicate. Repeated merge retains the existing idempotent result.

## Privacy and human preview

Manual copy/paste still constitutes external processing when the therapist transfers data. There is no automatic upload, synchronization, download or background provider request. The generated artifact is available for inspection through the backend response/read endpoint and the patient-specific GPT Project UI.

Minimization removes recognized synthetic direct identifiers and replaces database entity UUIDs with aliases. It is not anonymization: distinctive clinical narratives can identify people, and automated name/address detection is incomplete. A therapist must inspect the artifact before transfer under the product's applicable privacy policy. Local processing permission does not authorize external processing or audio/transcription; external manual processing has a separate durable consent. A broader organization-specific export policy remains outside this contract.

## Dormant remote infrastructure

Stage 2C.4A/4B provider ports, transports, privacy utilities and certification harness remain EXPERIMENTAL / DORMANT / NOT SELECTED PRODUCT PATH. Production defaults keep `CLINICAL_GIRA_PROVIDER=none`. Remote configuration cannot activate the adapter unless the separate `EXPERIMENTAL_REMOTE_GIRA=true` flag is explicitly supplied as well. Normal manual workflow does not require `OPENAI_API_KEY`.

No remote certification was executed for this stage. Remote model certification is not a blocker to the selected product architecture. Existing experimental artifacts remain available for deliberate future technical evaluation.

## Future patient Project organization (design only)

1. GENERAL CLINICAL SKILL: stable methodology.
2. CASE_RUNTIME_PROFILE: current selected case state and references.
3. CLINICAL SOURCES: selected approved reports/evidence/history when explicitly supported.
4. GPT PROJECT OUTPUT CONTRACT: the structured manual proposal format.

This ADR does not generate, edit or synchronize any current clinical skill or patient Project source. Stage 2D was not started by this decision; later implementation is described in the update above.
