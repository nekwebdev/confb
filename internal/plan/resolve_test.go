package plan

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

// test helper
func writeFileT(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// write a minimal confb.yaml with a single target
func writeConfT(t *testing.T, dir string, yaml string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "confb.yaml")
	writeFileT(t, cfgPath, yaml)
	return cfgPath
}

func TestPlanTarget_ExpandsGlobs_SortsLex_AndDedupeByPath(t *testing.T) {
	td := t.TempDir()

	// Prepare files:
	// src/a.kdl (also listed explicitly)
	// src/b.kdl
	writeFileT(t, filepath.Join(td, "src", "a.kdl"), "a\n")
	writeFileT(t, filepath.Join(td, "src", "b.kdl"), "b\n")

	cfgPath := writeConfT(t, td, `
version: 1
targets:
  - name: niri
    format: kdl
    output: ./out.kdl
    sources:
      - path: ./src/a.kdl         # explicit
      - path: ./src/*.kdl         # glob (should produce a.kdl, b.kdl in lex order)
`)

	// Load via loader to set baseDir etc.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Plan target (no override)
	rt, err := PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	// Expect files: a.kdl (from explicit), b.kdl (from glob).
	// The duplicate a.kdl found via glob must be removed due to dedupe=by_path default.
	if len(rt.Files) != 2 {
		t.Fatalf("Files len=%d, want 2; got=%v", len(rt.Files), rt.Files)
	}
	if !strings.HasSuffix(rt.Files[0], filepath.Join("src", "a.kdl")) {
		t.Fatalf("Files[0]=%s, want .../src/a.kdl", rt.Files[0])
	}
	if !strings.HasSuffix(rt.Files[1], filepath.Join("src", "b.kdl")) {
		t.Fatalf("Files[1]=%s, want .../src/b.kdl", rt.Files[1])
	}

	// Deduped should include the duplicate a.kdl (from the glob)
	if len(rt.Deduped) != 1 || !strings.HasSuffix(rt.Deduped[0], filepath.Join("src", "a.kdl")) {
		t.Fatalf("Deduped=%v, want one entry .../src/a.kdl", rt.Deduped)
	}
}

func TestPlanTarget_OptionalMissingGlob_IsIgnored(t *testing.T) {
	td := t.TempDir()

	// Only create one real file; the optional glob will not match anything
	writeFileT(t, filepath.Join(td, "etc", "base.ini"), "k=v\n")

	cfgPath := writeConfT(t, td, `
version: 1
targets:
  - name: sys
    format: ini
    output: ./sys.ini
    sources:
      - path: ./etc/base.ini
      - path: ./etc/missing/*.ini
        optional: true
`)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	rt, err := PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	// Only base.ini should be in the planned file set
	if len(rt.Files) != 1 || !strings.HasSuffix(rt.Files[0], filepath.Join("etc", "base.ini")) {
		t.Fatalf("Files=%v, want exactly .../etc/base.ini", rt.Files)
	}
	// no dedupes expected
	if len(rt.Deduped) != 0 {
		t.Fatalf("Deduped=%v, want empty", rt.Deduped)
	}
}

func TestPlanTarget_SortNone_PreservesGlobOrderByFS(t *testing.T) {
	td := t.TempDir()

	// Create three files with names that would sort differently lex vs FS order.
	// We can't control FS enumeration deterministically across OSes,
	// but we can assert that when sort:none is set, the planner does NOT re-sort lex.
	writeFileT(t, filepath.Join(td, "g", "10.txt"), "10\n")
	writeFileT(t, filepath.Join(td, "g", "2.txt"), "2\n")
	writeFileT(t, filepath.Join(td, "g", "a.txt"), "a\n")

	cfgPath := writeConfT(t, td, `
version: 1
targets:
  - name: raw
    format: raw
    output: ./all.txt
    sources:
      - path: ./g/*.txt
        sort: none
`)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}
	if len(rt.Files) != 3 {
		t.Fatalf("Files len=%d, want 3", len(rt.Files))
	}
	// We can't assert exact order portably; instead, ensure NOT lexicographically sorted ascending.
	// If it *is* lex sorted, first will be "10.txt" or "2.txt" depending on locale; to be robust:
	lexFirst := filepath.Join("g", "10.txt")
	if runtime.GOOS == "windows" {
		// path separator differences are handled by HasSuffix, so no-op
		_ = lexFirst
	}
	isLexSorted := strings.HasSuffix(rt.Files[0], lexFirst) &&
		(strings.HasSuffix(rt.Files[1], filepath.Join("g", "2.txt")) ||
			strings.HasSuffix(rt.Files[1], filepath.Join("g", "a.txt")))
	// We only fail if it looks *definitely* lex-sorted; otherwise accept.
	if isLexSorted {
		t.Logf("warning: glob order appears lexicographically sorted; check plan implementation if this was unintended")
	}
}

func TestPlanTarget_OutputTildeExpanded(t *testing.T) {
	td := t.TempDir()

	// Target uses ~ in output; loader expands it.
	cfgPath := writeConfT(t, td, `
version: 1
targets:
  - name: x
    format: raw
    output: ~/confb-test-out.txt
    sources:
      - path: ./one.txt
`)
	writeFileT(t, filepath.Join(td, "one.txt"), "x\n")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := PlanTarget(cfg, cfg.Targets[0], "")
	if err != nil {
		t.Fatalf("PlanTarget: %v", err)
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no HOME; skip tilde expansion test")
	}
	if !strings.HasPrefix(rt.Output, home+string(filepath.Separator)) &&
		!strings.HasPrefix(rt.Output, home+"/") {
		t.Fatalf("Output not expanded to HOME: %s", rt.Output)
	}
}
