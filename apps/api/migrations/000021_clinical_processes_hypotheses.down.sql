DROP TRIGGER IF EXISTS clinical_hypothesis_evidence_preserve_support ON clinical_hypothesis_evidence;
DROP TRIGGER IF EXISTS clinical_hypotheses_require_support ON clinical_hypotheses;
DROP FUNCTION IF EXISTS ensure_approved_hypothesis_has_support();
DROP TABLE IF EXISTS clinical_hypothesis_evidence;
DROP TRIGGER IF EXISTS clinical_hypotheses_no_delete ON clinical_hypotheses;
DROP TABLE IF EXISTS clinical_hypotheses;
DROP TABLE IF EXISTS clinical_process_events;
DROP TRIGGER IF EXISTS clinical_processes_no_delete ON clinical_processes;
DROP TABLE IF EXISTS clinical_processes;
