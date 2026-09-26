package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"path/filepath"
	"strings"
	"time"
)

func (c Config) validateIngestion() error {
	if !c.ClinicalIngestionEnabled {
		return nil
	}
	if c.DatabaseURL == "" || !filepath.IsAbs(c.ClinicalArtifactRoot) || c.ClinicalArtifactKeyID == "" {
		return fmt.Errorf("durable ingestion requires database, absolute artifact root and key ID")
	}
	key, err := base64.StdEncoding.DecodeString(c.ClinicalArtifactKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("CLINICAL_ARTIFACT_KEY must encode 32 bytes")
	}
	if c.ClinicalAudioRetentionHours < 1 || int64(c.ClinicalAudioRetentionHours) > int64(time.Duration(1<<63-1)/time.Hour) {
		return fmt.Errorf("CLINICAL_AUDIO_RETENTION_HOURS must be explicitly configured within duration limits; retention policy is unconfigured")
	}
	if c.ClinicalTranscriptRetentionPolicy != "explicit_deletion" || c.ClinicalJobMetadataRetentionPolicy != "retain_provenance" {
		return fmt.Errorf("explicit transcript and job metadata retention policies are required before enabling ingestion")
	}
	if c.ClinicalIngestionTimeoutSeconds < 1 || c.ClinicalIngestionTimeoutSeconds > 1500 {
		return fmt.Errorf("ingestion timeout must be 1..1500 seconds")
	}
	hash, err := hex.DecodeString(c.ClinicalTranscriptionConfigurationHash)
	if err != nil || len(hash) != 32 || strings.ToLower(c.ClinicalTranscriptionConfigurationHash) != c.ClinicalTranscriptionConfigurationHash {
		return fmt.Errorf("transcription configuration hash must match local sidecar health")
	}
	_, err = c.IngestionTenants()
	return err
}
func (c Config) IngestionTenants() ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	if strings.TrimSpace(c.ClinicalIngestionWorkerTenants) == "" {
		return out, nil
	}
	for _, v := range strings.Split(c.ClinicalIngestionWorkerTenants, ",") {
		id, err := uuid.Parse(strings.TrimSpace(v))
		if err != nil || id == uuid.Nil || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate ingestion worker tenant")
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) > 100 {
		return nil, fmt.Errorf("worker tenant batch exceeds 100")
	}
	return out, nil
}
