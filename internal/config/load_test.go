package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper
func writeFileT(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func TestLoad_Valid_YAML_WithMergeRules(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "confb.yaml")

	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: web
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

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("targets len = %d, want 1", len(cfg.Targets))
	}
	tg := cfg.Targets[0]
	if strings.ToLower(tg.Format) != "yaml" {
		t.Fatalf("format = %s, want yaml", tg.Format)
	}
	if tg.Merge == nil || tg.Merge.Rules == nil {
		t.Fatalf("merge.rules missing")
	}
	if strings.ToLower(tg.Merge.Rules.Maps) != "deep" {
		t.Fatalf("maps = %s, want deep", tg.Merge.Rules.Maps)
	}
	if strings.ToLower(tg.Merge.Rules.Arrays) != "unique_append" {
		t.Fatalf("arrays = %s, want unique_append", tg.Merge.Rules.Arrays)
	}
}

func TestLoad_Valid_KDL_WithSectionKeys_List(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "confb.yaml")

	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: niri
    format: kdl
    output: ./config.kdl
    sources:
      - path: ./colors.kdl
      - path: ./src/*.kdl
    merge:
      rules:
        keys: last_wins
        section_keys: ["layout", "theme"]
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tg := cfg.Targets[0]
	if tg.Merge == nil || tg.Merge.Rules == nil {
		t.Fatalf("merge.rules missing")
	}
	if strings.ToLower(tg.Merge.Rules.KDLKeys) != "last_wins" {
		t.Fatalf("kdl keys = %s, want last_wins", tg.Merge.Rules.KDLKeys)
	}
	if len(tg.Merge.Rules.KDLSectionKeys) != 2 {
		t.Fatalf("section_keys len = %d, want 2", len(tg.Merge.Rules.KDLSectionKeys))
	}
}

func TestLoad_Valid_INI_LastWins_Defaulting(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "confb.yaml")

	// Note: no repeated_keys given; loader should default to last_wins for INI.
	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: sys
    format: ini
    output: ./sys.ini
    sources:
      - path: ./base.ini
      - path: ./over.ini
    merge: {}
`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Targets[0].Merge.Rules
	if r == nil || strings.ToLower(r.INIRepeatedKeys) != "last_wins" {
		t.Fatalf("INI repeated_keys default = %v, want last_wins", r)
	}
}

func TestLoad_Errors_MergeWithAutoOrRaw(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "confb.yaml")

	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: bad1
    format: auto
    output: ./x
    sources:
      - path: ./a
    merge: {}
  - name: bad2
    format: raw
    output: ./y
    sources:
      - path: ./b
    merge:
      rules:
        maps: deep
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "merge is not supported when format is") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_Errors_ForeignFieldsRejected(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "confb.yaml")

	// Put yaml target but add kdl-only field -> should error.
	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: bad
    format: yaml
    output: ./out.yaml
    sources:
      - path: ./a.yaml
    merge:
      rules:
        maps: deep
        arrays: append
        section_keys: ["layout"]  # invalid for yaml
`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "not applicable to yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}
