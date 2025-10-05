package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads confb.yaml from disk, sets baseDir, normalizes, validates.
func Load(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.baseDir = filepath.Dir(abs)

	normalize(&cfg)

	if verr := validate(&cfg); !verr.ok() {
		return nil, verr
	}
	return &cfg, nil
}

// normalize applies simple defaults and expands ~ in output paths.
// Keep it minimal; format-aware behavior happens later.
func normalize(cfg *Config) {
	for i := range cfg.Targets {
		t := &cfg.Targets[i]

		// general defaults
		if t.Format == "" {
			t.Format = "auto"
		}
		if t.Dedupe == "" {
			t.Dedupe = "by_path"
		}
		if t.Newline == "" {
			t.Newline = "\n"
		}
		if t.Encoding == "" {
			t.Encoding = "utf8"
		}
		// expand ~ in output
		t.Output = expandTilde(t.Output)

		// default sort per source
		for j := range t.Sources {
			if t.Sources[j].Sort == "" {
				t.Sources[j].Sort = "lex"
			}
		}

		// Merge: only apply format defaults if user provided a merge block.
		if t.Merge != nil {
			if t.Merge.Rules == nil {
				t.Merge.Rules = &MergeRules{}
			}
			switch strings.ToLower(t.Format) {
			case "yaml", "toml", "json":
				if t.Merge.Rules.Maps == "" {
					t.Merge.Rules.Maps = "deep"
				}
				if t.Merge.Rules.Arrays == "" {
					t.Merge.Rules.Arrays = "replace"
				}
			case "kdl":
				if t.Merge.Rules.KDLKeys == "" {
					t.Merge.Rules.KDLKeys = "last_wins"
				}
				// sanitize section_keys: trim, drop empties, dedupe
				if len(t.Merge.Rules.KDLSectionKeys) > 0 {
					t.Merge.Rules.KDLSectionKeys = uniqueNonEmptyTrimmed(t.Merge.Rules.KDLSectionKeys)
				}
			case "ini":
				if t.Merge.Rules.INIRepeatedKeys == "" {
					t.Merge.Rules.INIRepeatedKeys = "last_wins"
				}
			case "raw", "auto":
				// no defaults; validation will reject merge under raw/auto
			}
		}
	}
}

// validate checks semantic rules and accumulates all issues before failing.
func validate(cfg *Config) *ValidationError {
	verr := &ValidationError{}

	if cfg.Version != 1 {
		verr.add("version must be 1 (got %d)", cfg.Version)
	}
	if len(cfg.Targets) == 0 {
		verr.add("targets must not be empty")
	}

	seenNames := map[string]struct{}{}
	for idx, t := range cfg.Targets {
		loc := func(field string) string { return field + " (target " + t.Name + ")" }

		// name
		if strings.TrimSpace(t.Name) == "" {
			verr.add("targets[%d].name is required and must be non-empty", idx)
		} else {
			if _, dup := seenNames[t.Name]; dup {
				verr.add("duplicate target name %q", t.Name)
			}
			seenNames[t.Name] = struct{}{}
		}

		// format enum
		if !inSet(strings.ToLower(t.Format), "auto", "yaml", "toml", "ini", "json", "raw", "kdl") {
			verr.add("%s: format must be one of auto|yaml|toml|ini|json|raw|kdl (got %q)", loc("format"), t.Format)
		}

		// output required
		if strings.TrimSpace(t.Output) == "" {
			verr.add("%s: output is required", loc("output"))
		}

		// dedupe enum
		if !inSet(strings.ToLower(t.Dedupe), "by_path", "none") {
			verr.add("%s: dedupe must be by_path|none (got %q)", loc("dedupe"), t.Dedupe)
		}

		// newline only "\n"
		if t.Newline != "\n" {
			verr.add("%s: newline must be \\n in MVP (got %q)", loc("newline"), t.Newline)
		}
		// encoding only utf8
		if strings.ToLower(t.Encoding) != "utf8" {
			verr.add("%s: encoding must be utf8 in MVP (got %q)", loc("encoding"), t.Encoding)
		}

		// sources
		if len(t.Sources) == 0 {
			verr.add("%s: sources must not be empty", loc("sources"))
		}
		for j, s := range t.Sources {
			if strings.TrimSpace(s.Path) == "" {
				verr.add("%s: sources[%d].path is required", loc("sources"), j)
			}
			if !inSet(strings.ToLower(s.Sort), "lex", "none") {
				verr.add("%s: sources[%d].sort must be lex|none (got %q)", loc("sources"), j, s.Sort)
			}
		}

		// Merge validation
		if t.Merge != nil {
			f := strings.ToLower(t.Format)
			r := t.Merge.Rules

			// raw/auto: merging not supported (must choose explicit format)
			if f == "raw" || f == "auto" {
				verr.add("%s: merge is not supported when format is %q; choose a concrete format", loc("merge"), f)
				continue
			}

			// ensure Rules exists after normalize()
			if r == nil {
				verr.add("%s: rules must be present when merge is declared", loc("merge.rules"))
				continue
			}

			switch f {
			case "yaml", "toml", "json":
				// enums
				if !inSet(strings.ToLower(r.Maps), "deep", "replace") {
					verr.add("%s: rules.maps must be deep|replace (got %q)", loc("merge.rules.maps"), r.Maps)
				}
				if !inSet(strings.ToLower(r.Arrays), "replace", "append", "unique_append") {
					verr.add("%s: rules.arrays must be replace|append|unique_append (got %q)", loc("merge.rules.arrays"), r.Arrays)
				}
				// forbid foreign fields
				if r.KDLKeys != "" || len(r.KDLSectionKeys) > 0 || r.INIRepeatedKeys != "" {
					verr.add("%s: rules contains fields not applicable to %s (kdl/ini fields must be omitted)", loc("merge.rules"), f)
				}

			case "kdl":
				if r.KDLKeys == "" {
					r.KDLKeys = "last_wins"
				}
				if !inSet(strings.ToLower(r.KDLKeys), "last_wins", "first_wins", "append") {
					verr.add("%s: rules.keys must be last_wins|first_wins|append (got %q)", loc("merge.rules.keys"), r.KDLKeys)
				}
				// validate section_keys content (no empty/whitespace entries)
				for _, sk := range r.KDLSectionKeys {
					if strings.TrimSpace(sk) == "" {
						verr.add("%s: rules.section_keys must not contain empty strings", loc("merge.rules.section_keys"))
						break
					}
				}
				// forbid foreign fields
				if r.Maps != "" || r.Arrays != "" || r.INIRepeatedKeys != "" {
					verr.add("%s: rules contains fields not applicable to kdl (maps/arrays/ini fields must be omitted)", loc("merge.rules"))
				}

			case "ini":
				if r.INIRepeatedKeys == "" {
					r.INIRepeatedKeys = "last_wins"
				}
				if !inSet(strings.ToLower(r.INIRepeatedKeys), "last_wins", "append") {
					verr.add("%s: rules.repeated_keys must be last_wins|append (got %q)", loc("merge.rules.repeated_keys"), r.INIRepeatedKeys)
				}
				// forbid foreign fields
				if r.Maps != "" || r.Arrays != "" || r.KDLKeys != "" || len(r.KDLSectionKeys) > 0 {
					verr.add("%s: rules contains fields not applicable to ini (yaml/toml/kdl fields must be omitted)", loc("merge.rules"))
				}
			}
		}
	}

	return verr
}

// expandTilde replaces a leading "~" with the user's home directory.
// If HOME is unknown we leave the string as-is.
func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}

// helper: simple membership test
func inSet(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}

// helper: trim/dedupe string list, remove empties
func uniqueNonEmptyTrimmed(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// BaseDir exposes the directory of the loaded confb.yaml for later path resolution.
func (c *Config) BaseDir() (string, error) {
	if c.baseDir == "" {
		return "", errors.New("config baseDir not set")
	}
	// ensure it exists; this is almost always true, but cheap to check
	if st, err := os.Stat(c.baseDir); err != nil || !st.IsDir() {
		return "", fs.ErrNotExist
	}
	return c.baseDir, nil
}
