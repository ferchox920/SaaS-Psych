package config

import (
	"encoding/base64"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestIngestionConfigurationFailClosed(t *testing.T) {
	if err := (Config{}).validateIngestion(); err != nil {
		t.Fatal(err)
	}
	c := Config{ClinicalIngestionEnabled: true, DatabaseURL: "synthetic-db", ClinicalArtifactRoot: t.TempDir(), ClinicalArtifactKeyID: "test-key", ClinicalArtifactKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), ClinicalAudioRetentionHours: 24, ClinicalIngestionTimeoutSeconds: 600, ClinicalTranscriptionConfigurationHash: strings.Repeat("a", 64), ClinicalIngestionWorkerTenants: uuid.NewString()}
	c.ClinicalTranscriptRetentionPolicy = "explicit_deletion"
	c.ClinicalJobMetadataRetentionPolicy = "retain_provenance"
	if err := c.validateIngestion(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){func(v *Config) { v.ClinicalArtifactKey = "secret-invalid" }, func(v *Config) { v.ClinicalArtifactRoot = "relative" }, func(v *Config) { v.ClinicalAudioRetentionHours = 0 }, func(v *Config) { v.ClinicalIngestionTimeoutSeconds = 2000 }, func(v *Config) { v.ClinicalIngestionWorkerTenants = "patient name" }, func(v *Config) { v.ClinicalTranscriptionConfigurationHash = "unknown" }} {
		bad := c
		mutate(&bad)
		if err := bad.validateIngestion(); err == nil {
			t.Fatal("unsafe config accepted")
		} else if strings.Contains(err.Error(), "secret-invalid") {
			t.Fatal("secret leaked")
		}
	}
}
func TestIngestionRetentionRequiresExplicitPolicy(t *testing.T) {
	t.Setenv("CLINICAL_AUDIO_RETENTION_HOURS", "")
	t.Setenv("CLINICAL_TRANSCRIPT_RETENTION_POLICY", "")
	t.Setenv("CLINICAL_JOB_METADATA_RETENTION_POLICY", "")
	c := Load()
	if c.ClinicalAudioRetentionHours != 0 || c.ClinicalTranscriptRetentionPolicy != "" || c.ClinicalJobMetadataRetentionPolicy != "" {
		t.Fatal("unapproved retention default")
	}
	c.ClinicalIngestionEnabled = true
	c.DatabaseURL = "synthetic-db"
	c.ClinicalArtifactRoot = t.TempDir()
	c.ClinicalArtifactKeyID = "test"
	c.ClinicalArtifactKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	c.ClinicalTranscriptionConfigurationHash = strings.Repeat("a", 64)
	if c.validateIngestion() == nil {
		t.Fatal("unconfigured retention permitted")
	}
	c.ClinicalAudioRetentionHours = 300 // Synthetic capability check, not a legal duration.
	if c.validateIngestion() == nil {
		t.Fatal("implicit transcript/job retention permitted")
	}
	c.ClinicalTranscriptRetentionPolicy = "explicit_deletion"
	c.ClinicalJobMetadataRetentionPolicy = "retain_provenance"
	if err := c.validateIngestion(); err != nil {
		t.Fatal(err)
	}
}
