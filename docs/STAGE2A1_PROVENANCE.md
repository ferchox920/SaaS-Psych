# Stage 2A.1 provenance contract

## ClinicalAIRun

An AI run is inserted with status `running` after input validation, clinical authorization, and context resolution, but before the provider is invoked. The same run is then completed as `succeeded`, `failed`, or `cancelled`.

`input_hash` covers only the operation's primary input. For live analysis this explicitly excludes browser-supplied longitudinal fields that the server discards; `context_hash` covers the canonical ordered set of approved server-owned source references. Build metadata (`app_version`, `build_revision`) is recorded separately and never changes either clinical content hash. Runtime defaults are `development` and `unknown`; Git is not required at runtime.

Structural sources contain only tenant, run, artifact type, artifact ID, optional version, and timestamps. They never duplicate transcripts, prompts, reports, names, emails, or clinical notes. Database validation permits only currently approved formulation snapshots, anchors belonging to an approved snapshot, and approved session reports. Once recorded, a source remains historical provenance if its artifact is later superseded.

## Approved context ordering and budget

The current composition is deterministic:

1. current approved formulation snapshot;
2. its anchors ordered by `created_at ASC, id ASC`;
3. current approved session reports ordered by clinical session `started_at DESC, id DESC`, then report `version DESC, id DESC`.

The service currently selects at most 10 approved reports. Thus histories of 10, 50, 100, or 500 reports all contribute no more than the 10 most recent current approvals. This is a simple fixed cap, not semantic selection, RAG, or a token-aware context policy. A future stage should replace the cap with an explicit context-budget policy before expanding the history.

## Session report item identity

New reports use `session-report-v1.1`. Every list item has a report-local, non-PHI ID such as `fact-001` or `hypothesis-001`; IDs must be unique and do not depend on array position. Edits carry existing IDs forward, while the service assigns an ID above every identity present in the stored report to a new item whose ID is omitted. Deleted identities are therefore never recycled within that report's edit lineage.

`inference_candidates[].evidence_refs` and `hypothesis_candidates[].evidence_refs` may point only to existing `fact`, `change`, `response`, or `affect` items in the same report. Interventions describe treatment activity, not evidence about the patient. Duplicate or dangling references are rejected on generation and edit; an item still referenced cannot be deleted until its references are revised. Interpretive candidates cannot be evidence for one another. This is structural integrity, not validation of clinical truth. Stored legacy reports remain readable without retroactive mutation.

Stored `session-report-v1` documents remain readable without rewriting their clinical JSON. Editing an old draft upgrades the stored schema marker and JSON to v1.1. This avoids a bulk clinical-data migration while giving all newly generated or edited reports stable references.
