package blend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"github.com/nekwebdev/confb/internal/config"
)

// BlendStructured reads all files, parses them as YAML/JSON/TOML, merges per rules,
// then returns the serialized result in the same format.
func BlendStructured(format string, rules *config.MergeRules, files []string) (string, error) {
	if rules == nil {
		return "", fmt.Errorf("merge rules required")
	}
	f := strings.ToLower(format)

	var acc any = nil
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %q: %w", path, err)
		}
		if len(strings.TrimSpace(string(b))) == 0 {
			continue
		}

		var doc any
		switch f {
		case "yaml":
			if err := yaml.Unmarshal(b, &doc); err != nil {
			 return "", fmt.Errorf("parse YAML %q: %w", path, err)
			}
		case "json":
			if err := json.Unmarshal(b, &doc); err != nil {
			 return "", fmt.Errorf("parse JSON %q: %w", path, err)
			}
		case "toml":
			if err := toml.Unmarshal(b, &doc); err != nil {
				return "", fmt.Errorf("parse TOML %q: %w", path, err)
			}
			// go-toml returns map[string]any / []any compatible with our merger
		default:
			return "", fmt.Errorf("unsupported format for BlendStructured: %s", format)
		}

		acc = mergeAny(acc, doc, rules)
	}

	// default empty doc
	if acc == nil {
		acc = map[string]any{}
	}

	switch f {
	case "yaml":
		out, err := yaml.Marshal(acc)
		if err != nil { return "", fmt.Errorf("marshal YAML: %w", err) }
		s := string(out)
		if !strings.HasSuffix(s, "\n") { s += "\n" }
		return s, nil
	case "json":
		out, err := json.MarshalIndent(acc, "", "  ")
		if err != nil { return "", fmt.Errorf("marshal JSON: %w", err) }
		s := string(out)
		if !strings.HasSuffix(s, "\n") { s += "\n" }
		return s, nil
	case "toml":
		out, err := toml.Marshal(acc)
		if err != nil { return "", fmt.Errorf("marshal TOML: %w", err) }
		s := string(out)
		if !strings.HasSuffix(s, "\n") { s += "\n" }
		return s, nil
	default:
		return "", fmt.Errorf("unsupported format")
	}
}

// --- merging primitives (unchanged) ---

func mergeAny(base, next any, rules *config.MergeRules) any {
	if base == nil { return clone(next) }
	if next == nil { return base }

	switch b := base.(type) {
	case map[string]any:
		nmap, ok := toStringMap(next)
		if !ok { return clone(next) } // type mismatch: later wins
		if strings.EqualFold(rules.Maps, "replace") {
			return clone(nmap)
		}
		out := make(map[string]any, len(b)+len(nmap))
		for k, v := range b { out[k] = clone(v) }
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
		if !ok { return clone(next) }
		switch strings.ToLower(rules.Arrays) {
		case "append":
			return append(cloneSlice(b), cloneSlice(narr)...)
		case "unique_append":
			return uniqueAppend(cloneSlice(b), cloneSlice(narr))
		default:
			return clone(narr) // replace
		}

	default:
		return clone(next)
	}
}

func toStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any: // possible from yaml
		out := make(map[string]any, len(m))
		for k, v := range m {
			ks, ok := k.(string)
			if !ok { return nil, false }
			out[ks] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func toAnySlice(v any) ([]any, bool) {
	if s, ok := v.([]any); ok { return s, true }
	return nil, false
}

func clone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v2 := range t { out[k] = clone(v2) }
		return out
	case []any:
		return cloneSlice(t)
	default:
		return t
	}
}

func cloneSlice(s []any) []any {
	out := make([]any, len(s))
	for i := range s { out[i] = clone(s[i]) }
	return out
}

func uniqueAppend(a, b []any) []any {
	out := make([]any, 0, len(a)+len(b))
	seen := map[string]struct{}{}

	key := func(x any) (string, bool) {
		switch v := x.(type) {
		case string:
			return "s:" + v, true
		case bool:
			if v {
				return "b:1", true
			}
			return "b:0", true
		case nil:
			return "n:", true

		// numeric (TOML often yields int64; JSON/YAML often yield float64)
		case int:
			return fmt.Sprintf("i:%d", v), true
		case int8:
			return fmt.Sprintf("i:%d", v), true
		case int16:
			return fmt.Sprintf("i:%d", v), true
		case int32:
			return fmt.Sprintf("i:%d", v), true
		case int64:
			return fmt.Sprintf("i:%d", v), true
		case uint:
			return fmt.Sprintf("u:%d", v), true
		case uint8:
			return fmt.Sprintf("u:%d", v), true
		case uint16:
			return fmt.Sprintf("u:%d", v), true
		case uint32:
			return fmt.Sprintf("u:%d", v), true
		case uint64:
			return fmt.Sprintf("u:%d", v), true
		case float32:
			return fmt.Sprintf("f:%g", float64(v)), true
		case float64:
			return fmt.Sprintf("f:%g", v), true
		default:
			// composite/unknown → don’t attempt to dedup (preserve order)
			return "", false
		}
	}

	for _, x := range a {
		if k, ok := key(x); ok {
			if _, exists := seen[k]; exists {
				continue
			}
			seen[k] = struct{}{}
		}
		out = append(out, clone(x))
	}
	for _, x := range b {
		if k, ok := key(x); ok {
			if _, exists := seen[k]; exists {
				continue
			}
			seen[k] = struct{}{}
		}
		out = append(out, clone(x))
	}
	return out
}

func guessFormatByExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	default:
		return ""
	}
}
