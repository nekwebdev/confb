package blend

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

func TestKDL_MergeByNameAndHead_LastWins(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.kdl")
	over := filepath.Join(td, "overlay.kdl")

	writeFileT(t, base, `
output "DP-2" {
  mode "5120x1440@120"
  scale 1
}
output "DP-1" {
  mode "2560x1440@60"
  scale 1
}
`)
	writeFileT(t, over, `
output "DP-2" {
  mode "5120x1440@239.761"
  transform "normal"
}
`)

	rules := &config.MergeRules{
		KDLKeys:       "last_wins",
		KDLSectionKeys: []string{"output"},
	}
	out, err := BlendKDL(rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendKDL error: %v", err)
	}

	// We expect two output blocks (DP-1 untouched; DP-2 merged), props sorted by key.
	wantDP1 := strings.TrimSpace(`
output "DP-1" {
  mode "2560x1440@60"
  scale 1
}
`)
	wantDP2 := strings.TrimSpace(`
output "DP-2" {
  mode "5120x1440@239.761"
  scale 1
  transform "normal"
}
`)
	// Order by section name is lexicographic; both are "output" so instance order is encounter order (DP-2 from base first then DP-1).
	// BUT since both are same name "output", the root renders both "output" instances in the order they were appended:
	// base had DP-2 then DP-1; we merged DP-2 in place. So expect DP-1 to appear after DP-2.
	if !strings.Contains(out, wantDP1) || !strings.Contains(out, wantDP2) {
		t.Fatalf("merged output missing expected blocks:\n--- got ---\n%s\n--- want contains ---\n%s\n\n%s\n", out, wantDP2, wantDP1)
	}
	// Ensure only one DP-2 block and it has the last_wins value for mode
	if strings.Count(out, `output "DP-2" {`) != 1 {
		t.Fatalf("expected exactly one DP-2 block, got:\n%s", out)
	}
	if !strings.Contains(out, `mode "5120x1440@239.761"`) {
		t.Fatalf("DP-2 should have last_wins mode, got:\n%s", out)
	}
}

func TestKDL_SectionKeys_Gating_NonMergedSectionsRemainSeparate(t *testing.T) {
	td := t.TempDir()
	a := filepath.Join(td, "a.kdl")
	b := filepath.Join(td, "b.kdl")

	writeFileT(t, a, `
bindings {
  up "k"
}
layout {
  keymap "us"
}
`)
	writeFileT(t, b, `
bindings {
  down "j"
}
layout {
  keymap "fr"
}
`)

	rules := &config.MergeRules{
		KDLKeys:        "last_wins",
		KDLSectionKeys: []string{"layout"}, // only layout merges; bindings stays as separate instances
	}
	out, err := BlendKDL(rules, []string{a, b})
	if err != nil {
		t.Fatalf("BlendKDL error: %v", err)
	}

	// Expect: one merged layout with keymap "fr" (last_wins), and TWO bindings blocks (up and down) as separate blocks.
	if strings.Count(out, "bindings {") != 2 {
		t.Fatalf("expected two bindings blocks, got:\n%s", out)
	}
	if strings.Count(out, "layout {") != 1 {
		t.Fatalf("expected one merged layout block, got:\n%s", out)
	}
	if !strings.Contains(out, `keymap "fr"`) {
		t.Fatalf("layout should last_wins to fr, got:\n%s", out)
	}
	// Ensure both bindings props exist (across two blocks).
	if !strings.Contains(out, `up "k"`) || !strings.Contains(out, `down "j"`) {
		t.Fatalf("expected both bindings values present, got:\n%s", out)
	}
}

func TestKDL_KeysMode_FirstWins_And_Append(t *testing.T) {
	td := t.TempDir()
	a := filepath.Join(td, "a.kdl")
	b := filepath.Join(td, "b.kdl")
	c := filepath.Join(td, "c.kdl")

	writeFileT(t, a, `
theme {
  color "dark"
  accent "blue"
}
`)
	writeFileT(t, b, `
theme {
  color "light"
  accent "cyan"
}
`)
	writeFileT(t, c, `
theme {
  accent "magenta"
}
`)

	// first_wins: first value wins for duplicate keys (color should remain "dark").
	rulesFirst := &config.MergeRules{
		KDLKeys:        "first_wins",
		KDLSectionKeys: []string{"theme"},
	}
	outFirst, err := BlendKDL(rulesFirst, []string{a, b, c})
	if err != nil {
		t.Fatalf("BlendKDL first_wins error: %v", err)
	}
	if !strings.Contains(outFirst, `color "dark"`) {
		t.Fatalf("first_wins should keep initial color=dark, got:\n%s", outFirst)
	}
	// accent first_wins keeps "blue" despite later files
	if !strings.Contains(outFirst, `accent "blue"`) {
		t.Fatalf("first_wins should keep initial accent=blue, got:\n%s", outFirst)
	}
	// append mode: all values preserved for the same key, in encounter order
	rulesAppend := &config.MergeRules{
		KDLKeys:        "append",
		KDLSectionKeys: []string{"theme"},
	}
	outAppend, err := BlendKDL(rulesAppend, []string{a, b, c})
	if err != nil {
		t.Fatalf("BlendKDL append error: %v", err)
	}
	// We expect three accent lines: blue, cyan, magenta (in that order due to encounter order)
	if strings.Count(outAppend, `accent "blue"`) != 1 ||
		strings.Count(outAppend, `accent "cyan"`) != 1 ||
		strings.Count(outAppend, `accent "magenta"`) != 1 {
		t.Fatalf("append should include all accents once, got:\n%s", outAppend)
	}
	// color append should include both dark and light
	if strings.Count(outAppend, `color "dark"`) != 1 ||
		strings.Count(outAppend, `color "light"`) != 1 {
		t.Fatalf("append should include both colors, got:\n%s", outAppend)
	}
}

func TestKDL_NestedMerge_InMergedSection(t *testing.T) {
	td := t.TempDir()
	a := filepath.Join(td, "a.kdl")
	b := filepath.Join(td, "b.kdl")

	writeFileT(t, a, `
layout {
  keymap "us"
  gaps {
    size 8
  }
}
`)
	writeFileT(t, b, `
layout {
  keymap "fr"
  gaps {
    inner 2
  }
}
`)

	rules := &config.MergeRules{
		KDLKeys:        "last_wins",
		KDLSectionKeys: []string{"layout", "gaps"},
	}
	out, err := BlendKDL(rules, []string{a, b})
	if err != nil {
		t.Fatalf("BlendKDL error: %v", err)
	}

	// Expect: single layout, keymap "fr"; nested gaps merged with both keys (size from a, inner from b).
	if strings.Count(out, "layout {") != 1 {
		t.Fatalf("expected one layout block, got:\n%s", out)
	}
	if !strings.Contains(out, `keymap "fr"`) {
		t.Fatalf("layout.keymap should be fr (last_wins), got:\n%s", out)
	}
	if strings.Count(out, "gaps {") != 1 {
		t.Fatalf("expected one gaps block, got:\n%s", out)
	}
	// props sorted lexicographically; we just check presence
	if !strings.Contains(out, `size 8`) || !strings.Contains(out, `inner 2`) {
		t.Fatalf("expected gaps to have size 8 and inner 2, got:\n%s", out)
	}
}
