package config

import "fmt"

// Versioned config file. We currently only accept version: 1
type Config struct {
	Version int      `yaml:"version"`
	Targets []Target `yaml:"targets"`
	// baseDir is set by the loader (directory of the confb.yaml)
	baseDir string `yaml:"-"`
}

// A single build target (one output file)
type Target struct {
	Name     string     `yaml:"name"`
	Format   string     `yaml:"format"`   // auto|yaml|toml|ini|json|raw|kdl
	Output   string     `yaml:"output"`   // path (may include ~)
	Sources  []Source   `yaml:"sources"`  // ordered
	Dedupe   string     `yaml:"dedupe"`   // by_path|none (default by_path)
	Newline  string     `yaml:"newline"`  // "\n" only in MVP
	Encoding string     `yaml:"encoding"` // utf8 only in MVP
	Merge    *MergeSpec `yaml:"merge,omitempty"` // optional; enables format-aware merging later
}

// A source entry (file path or glob), with options
type Source struct {
	Path     string `yaml:"path"`               // required; can be a glob
	Optional bool   `yaml:"optional,omitempty"` // if true, missing glob is not fatal
	Sort     string `yaml:"sort,omitempty"`     // lex|none (default lex)
}

// MergeSpec declares how to merge fragments for this target.
// - Profile optionally refers to a named preset (not resolved yet; just parsed).
// - Rules is an inline override (validated here).
type MergeSpec struct {
	Profile string      `yaml:"profile,omitempty"`
	Rules   *MergeRules `yaml:"rules,omitempty"`
}

// MergeRules is format-specific. Only the fields relevant to the chosen format
// should be set. Others must be omitted; the loader will error if they are used
// with an incompatible format.
//
// For yaml/toml/json:
//   - Maps:   "deep" (default) | "replace"
//   - Arrays: "replace" (default) | "append" | "unique_append"
//
// For kdl:
//   - KDLKeys:        "last_wins" (default) | "first_wins" | "append"
//   - KDLSectionKeys: optional list of identifiers to merge; if empty → merge all matching identifiers.
//
// For ini:
//   - INIRepeatedKeys: "last_wins" (default) | "append"
type MergeRules struct {
	// Structured formats
	Maps   string `yaml:"maps,omitempty"`   // deep|replace
	Arrays string `yaml:"arrays,omitempty"` // replace|append|unique_append

	// KDL
	KDLKeys        string   `yaml:"keys,omitempty"`          // last_wins|first_wins|append
	KDLSectionKeys []string `yaml:"section_keys,omitempty"`  // optional list; if empty -> merge all identifiers

	// INI
	INIRepeatedKeys string `yaml:"repeated_keys,omitempty"` // last_wins|append
}

// ValidationError aggregates multiple field issues into one error.
type ValidationError struct {
	Issues []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("configuration invalid:\n  - %s",
		func() string {
			s := ""
			for i, iss := range v.Issues {
				if i > 0 {
					s += "\n  - "
				}
				s += iss
			}
			return s
		}())
}

func (v *ValidationError) add(format string, a ...any) {
	v.Issues = append(v.Issues, fmt.Sprintf(format, a...))
}

func (v *ValidationError) ok() bool { return len(v.Issues) == 0 }
