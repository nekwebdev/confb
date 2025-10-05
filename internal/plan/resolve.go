package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nekwebdev/confb/internal/config"
)

// ResolvedTarget is the concrete build plan for one target.
type ResolvedTarget struct {
	Name    string
	Output  string   // final output path (already tilde-expanded in config)
	Files   []string // absolute paths to read, in order
	Deduped []string // absolute paths dropped due to by_path dedupe
}

// PlanTarget resolves globs, expands ~, applies sort + optional + dedupe rules.
func PlanTarget(cfg *config.Config, t config.Target, outputOverride string) (*ResolvedTarget, error) {
	baseDir, err := cfg.BaseDir()
	if err != nil {
		return nil, err
	}

	out := t.Output
	if outputOverride != "" {
		out = outputOverride
	}

	var files []string
	var deduped []string
	seen := map[string]struct{}{}

	for i, src := range t.Sources {
		// expand ~ and make path absolute (relative to confb.yaml dir)
		p := expandTilde(src.Path)
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}

		var matches []string
		hasGlob := strings.ContainsAny(p, "*?[")
		if hasGlob {
			m, err := filepath.Glob(p)
			if err != nil {
				return nil, fmt.Errorf("%s: sources[%d] invalid glob %q: %w", t.Name, i, src.Path, err)
			}
			matches = append(matches, m...)

			// explicit deterministic sort for lex (do NOT rely on OS glob order)
			if !strings.EqualFold(src.Sort, "none") {
				sort.Strings(matches)
			}

			if len(matches) == 0 && !src.Optional {
				return nil, fmt.Errorf("%s: sources[%d] pattern %q matched no files", t.Name, i, src.Path)
			}
		} else {
			// single file
			st, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) && src.Optional {
					continue
				}
				return nil, fmt.Errorf("%s: sources[%d] file %q: %w", t.Name, i, src.Path, err)
			}
			if st.IsDir() {
				return nil, fmt.Errorf("%s: sources[%d] %q is a directory (use a glob like %q/*)", t.Name, i, src.Path, src.Path)
			}
			matches = []string{p}
		}

		// apply dedupe policy (by absolute path), keep first occurrence
		for _, m := range matches {
			abs, err := filepath.Abs(m)
			if err != nil {
				return nil, fmt.Errorf("%s: resolve %q: %w", t.Name, m, err)
			}
			if strings.EqualFold(t.Dedupe, "by_path") {
				if _, ok := seen[abs]; ok {
					deduped = append(deduped, abs)
					continue
				}
				seen[abs] = struct{}{}
			}
			files = append(files, abs)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%s: resolved file list is empty", t.Name)
	}

	return &ResolvedTarget{
		Name:    t.Name,
		Output:  out,
		Files:   files,
		Deduped: deduped,
	}, nil
}

// local copy; avoids exporting from config package
func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}
