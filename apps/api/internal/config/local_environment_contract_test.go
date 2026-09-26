package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalPostgresPortIsConsistent(t *testing.T) {
	repoRoot := repositoryRoot(t)
	compose, err := os.ReadFile(filepath.Join(repoRoot, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), `"5433:5432"`) {
		t.Fatal("docker-compose.yml must publish local PostgreSQL on host port 5433")
	}

	forbidden := []string{
		"127.0.0.1:5432",
		"localhost:5432",
		"publish=5432",
		"Port 5432",
	}
	err = filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".github", "node_modules", ".tmp", ".local", "output", "PROGRESS":
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if name == "local_environment_contract_test.go" {
			return nil
		}
		if name != "Makefile" && name != ".env.example" && name != "docker-compose.yml" && filepath.Ext(name) != ".md" && filepath.Ext(name) != ".go" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, value := range forbidden {
			if strings.Contains(string(raw), value) {
				t.Errorf("%s contains obsolete local PostgreSQL reference %q", filepath.ToSlash(strings.TrimPrefix(path, repoRoot+string(filepath.Separator))), value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
}
