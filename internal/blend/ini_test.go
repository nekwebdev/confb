package blend

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

func TestINI_LastWins_Default(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.ini")
	over := filepath.Join(td, "overlay.ini")

	writeFileT(t, base, `
[layout]
name=base
color=blue
rule=one
`)
	writeFileT(t, over, `
[layout]
color=red
rule=two
`)

	out, err := BlendINI(&config.MergeRules{INIRepeatedKeys: "last_wins"}, []string{base, over})
	if err != nil {
		t.Fatalf("BlendINI error: %v", err)
	}

	if strings.Count(out, "color=") != 1 || !strings.Contains(out, "color=red") {
		t.Fatalf("expected single color=red line, got:\n%s", out)
	}
	if strings.Count(out, "rule=") != 1 || !strings.Contains(out, "rule=two") {
		t.Fatalf("expected single rule=two line, got:\n%s", out)
	}
	if !strings.Contains(out, "[layout]") || !strings.Contains(out, "name=base") {
		t.Fatalf("expected layout section with name=base, got:\n%s", out)
	}
}

func TestINI_Append_RepeatedKeys(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.ini")
	over := filepath.Join(td, "overlay.ini")

	writeFileT(t, base, `
[layout]
name=base
color=blue
rule=one
`)
	writeFileT(t, over, `
[layout]
color=red
rule=two
`)

	out, err := BlendINI(&config.MergeRules{INIRepeatedKeys: "append"}, []string{base, over})
	if err != nil {
		t.Fatalf("BlendINI error: %v", err)
	}

	if strings.Count(out, "color=") != 2 || !strings.Contains(out, "color=blue") || !strings.Contains(out, "color=red") {
		t.Fatalf("expected both color lines, got:\n%s", out)
	}
	if strings.Count(out, "rule=") != 2 || !strings.Contains(out, "rule=one") || !strings.Contains(out, "rule=two") {
		t.Fatalf("expected both rule lines, got:\n%s", out)
	}
	if !strings.Contains(out, "name=base") {
		t.Fatalf("expected name=base to be present, got:\n%s", out)
	}
}
