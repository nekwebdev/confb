package config

import "fmt"

// Versioned config file. we currently only accept version: 1
type Config struct {
	Version int      `yaml:"version"`
	Targets []Target `yaml:"targets"`
	// baseDir is set by the loader (directory of the confb.yaml)
	baseDir string `yaml:"-"`
}

// a single build target (one output file)
type Target struct {
	Name     string   `yaml:"name"`
	Format   string   `yaml:"format"`  // auto|yaml|toml|ini|json|raw
	Output   string   `yaml:"output"`  // path (may include ~)
	Sources  []Source `yaml:"sources"` // ordered
	Dedupe   string   `yaml:"dedupe"`  // by_path|none (default by_path)
	Newline  string   `yaml:"newline"` // "\n" only in mvp
	Encoding string   `yaml:"encoding"`// utf8 only in mvp

	// reserved for future merge options – parsed but not used yet
	Merge *MergeSpec `yaml:"merge,omitempty"`
}

// a source entry (file path or glob), with options
type Source struct {
	Path     string `yaml:"path"`               // required; can be a glob
	Optional bool   `yaml:"optional,omitempty"` // if true, missing glob is not fatal
	Sort     string `yaml:"sort,omitempty"`     // lex|none (default lex)
}

type MergeSpec struct {
	Profile  string `yaml:"profile,omitempty"`
	Strategy string `yaml:"strategy,omitempty"` // deep|append|replace (future)
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

