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
	// if path is relative, make it absolute from current working dir
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

	// remember the directory of the YAML to resolve relative source paths later
	cfg.baseDir = filepath.Dir(abs)

	// apply defaults + light normalization
	normalize(&cfg)

	// run strict validation
	if verr := validate(&cfg); !verr.ok() {
		return nil, verr
	}
	return &cfg, nil
}

// normalize applies simple defaults and expands ~ in output paths.
// keep it tiny; format-aware stuff happens later.
func normalize(cfg *Config) {
	for i := range cfg.Targets {
		t := &cfg.Targets[i]

		// defaults
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

		// default sort for sources
		for j := range t.Sources {
			if t.Sources[j].Sort == "" {
				t.Sources[j].Sort = "lex"
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

		// name checks
		if strings.TrimSpace(t.Name) == "" {
			verr.add("targets[%d].name is required and must be non-empty", idx)
		} else {
			if _, dup := seenNames[t.Name]; dup {
				verr.add("duplicate target name %q", t.Name)
			}
			seenNames[t.Name] = struct{}{}
		}

		// format enum
		if !inSet(t.Format, "auto", "yaml", "toml", "ini", "json", "raw") {
			verr.add("%s: format must be one of auto|yaml|toml|ini|json|raw (got %q)", loc("format"), t.Format)
		}

		// output required
		if strings.TrimSpace(t.Output) == "" {
			verr.add("%s: output is required", loc("output"))
		}

		// dedupe enum
		if !inSet(t.Dedupe, "by_path", "none") {
			verr.add("%s: dedupe must be by_path|none (got %q)", loc("dedupe"), t.Dedupe)
		}
		// newline only "\n" in MVP
		if t.Newline != "\n" {
			verr.add("%s: newline must be \\n in MVP (got %q)", loc("newline"), t.Newline)
		}
		// encoding only utf8 in MVP
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
			if !inSet(s.Sort, "lex", "none") {
				verr.add("%s: sources[%d].sort must be lex|none (got %q)", loc("sources"), j, s.Sort)
			}
		}
	}

	return verr
}

// expandTilde replaces a leading "~" with the user's home directory.
// if HOME is unknown we leave the string as-is (and let later steps fail clearly).
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

