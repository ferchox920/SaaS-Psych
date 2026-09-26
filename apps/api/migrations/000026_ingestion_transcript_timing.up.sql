-- Preserve timing already included in the canonical transcript content hash.
-- Legacy NULL values mean timing was not retained; never invent historical data.
ALTER TABLE clinical_transcript_versions
 ADD COLUMN duration_seconds DOUBLE PRECISION CHECK(duration_seconds>=0 AND duration_seconds<'Infinity'::double precision),
 ADD COLUMN processing_seconds DOUBLE PRECISION CHECK(processing_seconds>=0 AND processing_seconds<'Infinity'::double precision);
