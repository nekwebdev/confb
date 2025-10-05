package exec

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper
func writeFileT(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func TestBuildAndWrite_NormalizesAndWritesAtomically(t *testing.T) {
	td := t.TempDir()

	// Inputs with mixed newlines and no trailing newline
	f1 := filepath.Join(td, "a.kdl")
	f2 := filepath.Join(td, "b.kdl")
	writeFileT(t, f1, "key 1\r\nkey2 2\r\n") // CRLF, already ends with newline
	writeFileT(t, f2, "last 3\r")            // lone CR, no newline at end

	out := filepath.Join(td, "out.kdl")

	if err := BuildAndWrite(out, []string{f1, f2}); err != nil {
		t.Fatalf("BuildAndWrite: %v", err)
	}

	// Read and assert normalized content
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	got := string(b)
	// Expect LF newlines and exactly one trailing newline; also a newline inserted between files if needed
	want := "key 1\nkey2 2\nlast 3\n"
	if got != want {
		t.Fatalf("content:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestSHA256OfFiles_MatchesBuildContent(t *testing.T) {
	td := t.TempDir()

	f1 := filepath.Join(td, "a.txt")
	f2 := filepath.Join(td, "b.txt")
	writeFileT(t, f1, "hello\r\n")
	writeFileT(t, f2, "world") // no newline

	sum, err := SHA256OfFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("SHA256OfFiles: %v", err)
	}

	// Recreate expected content the same way BuildAndWrite would
	expected := "hello\nworld\n"
	h := sha256.Sum256([]byte(expected))
	want := hex.EncodeToString(h[:])

	if !strings.EqualFold(sum, want) {
		t.Fatalf("sha mismatch: got %s want %s", sum, want)
	}
}
