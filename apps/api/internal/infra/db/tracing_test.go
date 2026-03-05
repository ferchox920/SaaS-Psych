package db

import "testing"

func TestParseDatabaseName(t *testing.T) {
	name := parseDatabaseName("postgres://user:pass@localhost:5432/sessionflow?sslmode=disable")
	if name != "sessionflow" {
		t.Fatalf("expected sessionflow, got %q", name)
	}
}

func TestSanitizeStatement(t *testing.T) {
	got := sanitizeStatement("SELECT  *\nFROM users\tWHERE id = $1")
	if got != "SELECT * FROM users WHERE id = $1" {
		t.Fatalf("unexpected sanitized statement: %q", got)
	}
}
