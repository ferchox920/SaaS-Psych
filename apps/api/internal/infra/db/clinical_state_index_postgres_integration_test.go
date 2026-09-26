package db

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLongitudinalStateFilteredReadIndexesPostgresIntegration(t *testing.T) {
	if os.Getenv("RUN_PG_INTEGRATION") != "1" {
		t.Skip("set RUN_PG_INTEGRATION=1")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, postgresIntegrationDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	checks := []struct {
		name      string
		fragments []string
	}{
		{"clinical_events_approved_recent", []string{"clinical_events", "tenant_id", "client_id", "observed_at DESC", "approval_status = 'approved'"}},
		{"clinical_processes_approved_state", []string{"clinical_processes", "tenant_id", "client_id", "CASE", "updated_at DESC", "approval_status = 'approved'"}},
		{"clinical_hypotheses_approved_unassigned_state", []string{"clinical_hypotheses", "tenant_id", "client_id", "updated_at DESC", "approval_status = 'approved'", "process_id IS NULL"}},
		{"clinical_diffs_open_state", []string{"clinical_diffs", "tenant_id", "client_id", "created_at DESC", "status = ANY"}},
	}
	for _, check := range checks {
		var definition string
		err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT indexdef FROM pg_indexes WHERE schemaname=current_schema() AND indexname=$1),'')`, check.name).Scan(&definition)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range check.fragments {
			if !strings.Contains(definition, fragment) {
				t.Errorf("index %s must contain %q, definition=%q", check.name, fragment, definition)
			}
		}
	}
}
