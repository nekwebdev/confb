package plan_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
	"github.com/nekwebdev/confb/internal/plan"
)

// helper to write a file with content, creating parent dirs
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPlanTarget_SortingOptionalDedupe(t *testing.T) {
	tmp := t.TempDir()

	// inputs
	srcDir := filepath.Join(tmp, "src")
	mustWrite(t, filepath.Join(srcDir, "00-one.kdl"), "section1 { k 1 }")
	mustWrite(t, filepath.Join(srcDir, "10-two.kdl"), "section2 { k 2 }")
	mustWrite(t, filepath.Join(srcDir, "20-three.kdl"), "section3 { k 3 }")

	confb := `
version: 1
targets:
  - name: test
    format: auto
    output: out/test.conf
    dedupe: by_path
    sources:
      - path: src/*.kdl
        sort: lex
      - path: src/10-*.kdl
        sort: lex
      - path: missing/*.kdl
        optional: true
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, confb)

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	rt, err := plan.PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	wantFiles := []string{
		filepath.Join(srcDir, "00-one.kdl"),
		filepath.Join(srcDir, "10-two.kdl"),
		filepath.Join(srcDir, "20-three.kdl"),
	}
	if !reflect.DeepEqual(rt.Files, wantFiles) {
		t.Fatalf("Files mismatch.\n got: %#v\nwant: %#v", rt.Files, wantFiles)
	}

	wantDeduped := []string{filepath.Join(srcDir, "10-two.kdl")}
	if !reflect.DeepEqual(rt.Deduped, wantDeduped) {
		t.Fatalf("Deduped mismatch.\n got: %#v\nwant: %#v", rt.Deduped, wantDeduped)
	}

	if rt.Output != "out/test.conf" {
		t.Fatalf("unexpected output: got %q want %q", rt.Output, "out/test.conf")
	}
}

func TestPlanTarget_OptionalVsRequired(t *testing.T) {
	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	mustWrite(t, filepath.Join(srcDir, "base.kdl"), "section { k 1 }")

	confb := `
version: 1
targets:
  - name: ok_optional
    format: auto
    output: out/a.conf
    sources:
      - path: src/base.kdl
      - path: missing/*.kdl
        optional: true
  - name: fail_required
    format: auto
    output: out/b.conf
    sources:
      - path: src/base.kdl
      - path: missing/*.kdl
        optional: false
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, confb)

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// ok_optional should resolve fine
	if _, err := plan.PlanTarget(cfg, cfg.Targets[0], ""); err != nil {
		t.Fatalf("ok_optional PlanTarget unexpected error: %v", err)
	}

	// fail_required should error due to missing glob
	if _, err := plan.PlanTarget(cfg, cfg.Targets[1], ""); err == nil {
		t.Fatalf("fail_required PlanTarget expected error for missing required glob, got nil")
	}
}

func TestPlanTarget_DedupeNone_AllowsDuplicates(t *testing.T) {
	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src")
	f := filepath.Join(srcDir, "dup.kdl")
	mustWrite(t, f, "section { k 1 }")

	confb := `
version: 1
targets:
  - name: dup_ok
    format: auto
    output: out/dup.conf
    dedupe: none
    sources:
      - path: src/dup.kdl
      - path: src/*.kdl
        sort: lex
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, confb)

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	rt, err := plan.PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	want := []string{f, f}
	if !reflect.DeepEqual(rt.Files, want) {
		t.Fatalf("Files mismatch.\n got: %#v\nwant: %#v", rt.Files, want)
	}
	if len(rt.Deduped) != 0 {
		t.Fatalf("unexpected deduped entries: %#v", rt.Deduped)
	}
}

func TestPlanTarget_TildeAndRelativeResolution(t *testing.T) {
	tmp := t.TempDir()

	// fake HOME under tmp so ~ expands predictably
	fakeHome := filepath.Join(tmp, "home")
	if err := os.MkdirAll(fakeHome, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	homeFile := filepath.Join(fakeHome, "niri.kdl")
	mustWrite(t, homeFile, "section { k 42 }")

	srcDir := filepath.Join(tmp, "src")
	relFile := filepath.Join(srcDir, "rel.kdl")
	mustWrite(t, relFile, "section { k 7 }")

	confb := `
version: 1
targets:
  - name: paths
    format: auto
    output: out/paths.conf
    sources:
      - path: ~/niri.kdl
      - path: src/rel.kdl
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, confb)

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	rt, err := plan.PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	want := []string{homeFile, relFile}
	if !reflect.DeepEqual(rt.Files, want) {
		t.Fatalf("Files mismatch.\n got: %#v\nwant: %#v", rt.Files, want)
	}
}
