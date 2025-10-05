package blend

import (
	"os"
	"testing"
)

// shared helper: write a file or fail the test
func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// shared helper: normalize []any → []int64 for numeric TOML/YAML arrays
func toInt64Slice(v any) ([]int64, bool) {
	switch s := v.(type) {
	case []any:
		out := make([]int64, len(s))
		for i, x := range s {
			switch n := x.(type) {
			case int64:
				out[i] = n
			case int:
				out[i] = int64(n)
			case float64:
				out[i] = int64(n)
			default:
				return nil, false
			}
		}
		return out, true
	default:
		return nil, false
	}
}
