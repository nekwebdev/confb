package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

// write helper
func writeFileT(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func TestValidate_OK(t *testing.T) {
	td := t.TempDir()
	cfg := filepath.Join(td, "confb.yaml")

	writeFileT(t, cfg, `
version: 1
targets:
  - name: y
    format: yaml
    output: ./out.yaml
    sources:
      - path: ./a.yaml
      - path: ./b.yaml
    merge:
      rules:
        maps: deep
        arrays: unique_append
`)

	root := NewRootCmdForTest()
	root.SetArgs([]string{"validate", "-c", cfg})
	if err := root.Execute(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestBuild_DryRun_OK(t *testing.T) {
	td := t.TempDir()
	cfg := filepath.Join(td, "confb.yaml")
	base := filepath.Join(td, "a.yaml")
	over := filepath.Join(td, "b.yaml")

	writeFileT(t, base, "a: 1\n")
	writeFileT(t, over, "a: 2\n")
	writeFileT(t, cfg, `
version: 1
targets:
  - name: y
    format: yaml
    output: ./out.yaml
    sources:
      - path: ./a.yaml
      - path: ./b.yaml
    merge:
      rules:
        maps: deep
        arrays: replace
`)

	// sanity: loader parses it
	if _, err := config.Load(cfg); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	root := NewRootCmdForTest()
	root.SetArgs([]string{"build", "-c", cfg, "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("build --dry-run failed: %v", err)
	}
}
