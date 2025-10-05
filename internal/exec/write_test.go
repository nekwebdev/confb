package exec_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	execpkg "github.com/nekwebdev/confb/internal/exec"
)

// helper to write a file with content, creating parent dirs
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// normalize helper mirrored for expected strings
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// ensure a single trailing '\n'
func singleTrailingLF(s string) string {
	for strings.HasSuffix(s, "\n\n") {
		s = s[:len(s)-1]
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func TestBuildAndWrite_NormalizesNewlines_And_InsertsBoundaryLF(t *testing.T) {
	tmp := t.TempDir()

	// f1 has CRLF and no trailing newline; f2 has classic CR and ends with CR
	f1 := filepath.Join(tmp, "in", "a", "one.kdl")
	f2 := filepath.Join(tmp, "in", "b", "two.kdl")
	mustWrite(t, f1, "alpha\r\nbeta")  // no trailing newline
	mustWrite(t, f2, "gamma\rdelta\r") // ends with CR -> becomes LF on normalize

	out := filepath.Join(tmp, "out", "merged.kdl")

	if err := execpkg.BuildAndWrite(out, []string{f1, f2}); err != nil {
		t.Fatalf("BuildAndWrite: %v", err)
	}

	gotBytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(gotBytes)

	// expected: normalize both; add boundary LF if f1 lacked it; ensure single trailing LF
	expF1 := normalizeNewlines("alpha\r\nbeta")
	expF2 := normalizeNewlines("gamma\rdelta\r")
	var expected string
	if !strings.HasSuffix(expF1, "\n") {
		expected = expF1 + "\n" + expF2
	} else {
		expected = expF1 + expF2
	}
	expected = singleTrailingLF(expected)

	if got != expected {
		t.Fatalf("output mismatch.\n--- got ---\n%q\n--- want ---\n%q", got, expected)
	}

	// no leftover temp files in output dir
	entries, err := os.ReadDir(filepath.Dir(out))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".confb-") {
			t.Fatalf("leftover temp file found: %s", e.Name())
		}
	}
}

func TestBuildAndWrite_SingleFile_AddsSingleTrailingLF(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "in", "single.kdl")
	mustWrite(t, f, "line1") // no trailing newline in source

	out := filepath.Join(tmp, "out", "single.out")

	if err := execpkg.BuildAndWrite(out, []string{f}); err != nil {
		t.Fatalf("BuildAndWrite: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "line1\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestSHA256OfFiles_MatchesNormalizedContent(t *testing.T) {
	tmp := t.TempDir()

	// three fragments with mixed newlines and missing trailing LFs on some
	f1 := filepath.Join(tmp, "a.kdl")
	f2 := filepath.Join(tmp, "b.kdl")
	f3 := filepath.Join(tmp, "c.kdl")
	mustWrite(t, f1, "A\r\nB")    // no final newline
	mustWrite(t, f2, "C\rD\r\n")  // ends with CRLF
	mustWrite(t, f3, "E")         // no final newline

	sum, err := execpkg.SHA256OfFiles([]string{f1, f2, f3})
	if err != nil {
		t.Fatalf("SHA256OfFiles: %v", err)
	}

	// expected normalized content exactly as writer does it
	exp := normalizeNewlines("A\r\nB")
	if !strings.HasSuffix(exp, "\n") {
		exp += "\n"
	}
	exp2 := normalizeNewlines("C\rD\r\n")
	exp += exp2
	if !strings.HasSuffix(exp, "\n") {
		exp += "\n"
	}
	exp3 := normalizeNewlines("E")
	// add boundary LF if previous chunk didn't end with one
	if !strings.HasSuffix(exp, "\n") {
		exp += "\n"
	}
	exp += exp3
	// final single trailing LF
	exp = singleTrailingLF(exp)

	h := sha256.New()
	h.Write([]byte(exp))
	want := hex.EncodeToString(h.Sum(nil))

	if sum != want {
		t.Fatalf("checksum mismatch.\n got: %s\nwant: %s\nexp-content: %q", sum, want, exp)
	}
}
