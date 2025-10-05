package blend

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nekwebdev/confb/internal/config"
)

// BlendINI merges INI-like files (sections with key=value lines).
// - Sections merge by name.
// - Keys: last_wins (default) or append (keeps all repeated key lines in order).
// - Comments starting with ';' or '#' are ignored.
// - Blank lines ignored.
// - Lines outside any section are treated as section "" (global).
func BlendINI(rules *config.MergeRules, files []string) (string, error) {
	mode := strings.ToLower(rules.INIRepeatedKeys)
	if mode == "" { mode = "last_wins" }

	type sec map[string][]string // key -> list of values (for append mode)
	acc := map[string]sec{}      // section name -> keys map
	seenSec := []string{}        // to render sections in stable order

	ensure := func(name string) sec {
		if s, ok := acc[name]; ok { return s }
		acc[name] = sec{}
		seenSec = append(seenSec, name)
		return acc[name]
	}

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil { return "", fmt.Errorf("read %q: %w", path, err) }
		sc := bufio.NewScanner(f)
		sect := ensure("") // global by default

		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" { continue }
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
				continue
			}
			// section header?
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				name := strings.TrimSpace(line[1 : len(line)-1])
				sect = ensure(name)
				continue
			}
			// key=value (first '=' splits)
			i := strings.IndexRune(line, '=')
			if i <= 0 {
				// ignore malformed lines (could also error)
				continue
			}
			key := strings.TrimSpace(line[:i])
			val := strings.TrimSpace(line[i+1:])
			if key == "" { continue }

			switch mode {
			case "append":
				sect[key] = append(sect[key], val)
			default: // last_wins
				sect[key] = []string{val}
			}
		}
		_ = f.Close()
	}

	// render
	var b strings.Builder
	for _, name := range seenSec {
		sect := acc[name]
		if name != "" {
			b.WriteString("[")
			b.WriteString(name)
			b.WriteString("]\n")
		}
		// deterministic key order: lexicographic
		keys := make([]string, 0, len(sect))
		for k := range sect { keys = append(keys, k) }
		sortStrings(keys)
		for _, k := range keys {
			for _, v := range sect[k] {
				b.WriteString(k)
				b.WriteString("=")
				b.WriteString(v)
				b.WriteString("\n")
			}
		}
		if !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
	}
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	return b.String(), nil
}

// tiny local sorter to avoid importing sort in this file
func sortStrings(a []string) {
	for i := 0; i < len(a)-1; i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}
