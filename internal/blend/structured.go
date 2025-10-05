package blend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nekwebdev/confb/internal/config"
)

// BlendStructured reads all files, parses them as YAML or JSON, merges them
// using the per-target rules, then returns the serialized result (YAML or JSON).
func BlendStructured(format string, rules *config.MergeRules, files []string) (string, error) {
	if rules == nil {
		return "", fmt.Errorf("merge rules required")
	}

	var acc any = nil
	for _, path := range files {
		// read file
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %q: %w", path, err)
		}
		// skip empty files (treat as null)
		if len(strings.TrimSpace(string(b))) == 0 {
			continue
		}

		var doc any
		switch strings.ToLower(format) {
		case "yaml":
			if err := yaml.Unmarshal(b, &doc); err != nil {
				return "", fmt.Errorf("parse YAML %q: %w", path, err)
			}
		case "json":
			if err := json.Unmarshal(b, &doc); err != nil {
				return "", fmt.Errorf("parse JSON %q: %w", path, err)
			}
		default:
			return "", fmt.Errorf("unsupported format for BlendStructured: %s", format)
		}

		acc = mergeAny(acc, doc, rules)
	}

	// if everything was empty -> produce an empty doc of the "most likely" type
	if acc == nil {
		acc = map[string]any{}
	}

	switch strings.ToLower(format) {
	case "yaml":
		out, err := yaml.Marshal(acc)
		if err != nil {
			return "", fmt.Errorf("marshal YAML: %w", err)
		}
		s := string(out)
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		return s, nil
	case "json":
		out, err := json.MarshalIndent(acc, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal JSON: %w", err)
		}
		s := string(out)
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		return s, nil
	default:
		return "", fmt.Errorf("unsupported format")
	}
}

// --- merging primitives ---

func mergeAny(base, next any, rules *config.MergeRules) any {
	if base == nil {
		return clone(next)
	}
	if next == nil {
		return base
	}

	switch b := base.(type) {
	case map[string]any:
		nmap, ok := toStringMap(next)
		if !ok {
			// type mismatch -> later wins
			return clone(next)
		}
		if strings.EqualFold(rules.Maps, "replace") {
			return clone(nmap)
		}
		// deep merge
		out := make(map[string]any, len(b)+len(nmap))
		for k, v := range b {
			out[k] = clone(v)
		}
		for k, v2 := range nmap {
			if v1, exists := out[k]; exists {
				out[k] = mergeAny(v1, v2, rules)
			} else {
				out[k] = clone(v2)
			}
		}
		return out

	case []any:
		narr, ok := toAnySlice(next)
		if !ok {
			// type mismatch -> later wins
			return clone(next)
		}
		switch strings.ToLower(rules.Arrays) {
		case "append":
			return append(cloneSlice(b), cloneSlice(narr)...)
		case "unique_append":
			return uniqueAppend(cloneSlice(b), cloneSlice(narr))
		default: // "replace" or unknown -> be strict
			return clone(narr)
		}

	default:
		// scalars or mismatched composite -> later wins
		return clone(next)
	}
}

func toStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		// YAML can decode into map[interface{}]any; normalize keys to strings when possible
		out := make(map[string]any, len(m))
		for k, v := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[ks] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func toAnySlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	default:
		return nil, false
	}
}

func clone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v2 := range t {
			out[k] = clone(v2)
		}
		return out
	case []any:
		return cloneSlice(t)
	default:
		return t
	}
}

func cloneSlice(s []any) []any {
	out := make([]any, len(s))
	for i := range s {
		out[i] = clone(s[i])
	}
	return out
}

func uniqueAppend(a, b []any) []any {
	out := make([]any, 0, len(a)+len(b))
	seen := map[string]struct{}{}
	key := func(x any) (string, bool) {
		// dedup only for simple scalars
		switch v := x.(type) {
		case string:
			return "s:" + v, true
		case float64:
			return fmt.Sprintf("f:%v", v), true
		case bool:
			if v {
				return "b:1", true
			}
			return "b:0", true
		case nil:
			return "n:", true
		default:
			return "", false
		}
	}

	for _, x := range a {
		k, ok := key(x)
		if ok {
			if _, exists := seen[k]; exists {
				continue
			}
			seen[k] = struct{}{}
		}
		out = append(out, clone(x))
	}
	for _, x := range b {
		k, ok := key(x)
		if ok {
			if _, exists := seen[k]; exists {
				continue
			}
			seen[k] = struct{}{}
		}
		out = append(out, clone(x))
	}
	return out
}

// Utility to guess content type by extension (not used yet, but handy if needed).
func guessFormatByExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	default:
		return ""
	}
}
