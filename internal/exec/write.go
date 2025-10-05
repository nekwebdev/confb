package exec

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// BuildAndWrite concatenates files -> normalized string -> atomic write.
// (Used when no merge is requested.)
func BuildAndWrite(outputPath string, files []string) error {
	content, err := readAndNormalize(files)
	if err != nil {
		return err
	}
	return WriteAtomic(outputPath, content)
}

// WriteAtomic writes content to outputPath atomically (same-dir temp + fsync + rename).
func WriteAtomic(outputPath string, content string) error {
	// ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(outputPath), err)
	}
	// atomic write: same-dir temp + fsync + rename
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".confb-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	// buffered writer
	w := bufio.NewWriter(tmp)
	if _, err := w.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("flush temp: %w", err)
	}

	// fsync temp file
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}

	// rename over final
	if err := os.Rename(tmpName, outputPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename %q -> %q: %w", tmpName, outputPath, err)
	}

	// best-effort fsync the directory
	if dir, err := os.Open(filepath.Dir(outputPath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	return nil
}

// SHA256OfFiles returns a hex sha256 of the normalized concatenation.
// used only for --trace-checksums; same path as BuildAndWrite but without writing.
func SHA256OfFiles(files []string) (string, error) {
	content, err := readAndNormalize(files)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = io.WriteString(h, content)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readAndNormalize streams all files, converts CRLF/CR to LF, validates UTF-8,
// ensures a single trailing newline, and inserts a newline between files if needed.
func readAndNormalize(files []string) (string, error) {
	var b stringsBuilder

	for idx, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open %q: %w", path, err)
		}

		r := bufio.NewReader(f)
		for {
			chunk, err := r.ReadString('\n')
			if len(chunk) > 0 {
				chunk = normalizeNewlines(chunk)
				if !utf8.ValidString(chunk) {
					_ = f.Close()
					return "", fmt.Errorf("%q: not valid UTF-8 (MVP requires utf8)", path)
				}
				b.WriteString(chunk)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = f.Close()
				return "", fmt.Errorf("read %q: %w", path, err)
			}
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("close %q: %w", path, err)
		}

		// ensure a newline boundary between files if the previous didn't end with one
		if idx < len(files)-1 && !b.endsWithNewline() {
			b.WriteByte('\n')
		}
	}

	// guarantee exactly one trailing \n
	for b.endsWithTwoNewlines() {
		b.TrimLastByte()
	}
	if !b.endsWithNewline() {
		b.WriteByte('\n')
	}

	return b.String(), nil
}

func normalizeNewlines(s string) string {
	// first convert CRLF to LF, then any lone CR to LF
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// small string builder wrapper with a couple helpers
type stringsBuilder struct {
	sb strings.Builder
}

func (b *stringsBuilder) WriteString(s string) { _, _ = b.sb.WriteString(s) }
func (b *stringsBuilder) WriteByte(c byte)     { _ = b.sb.WriteByte(c) }
func (b *stringsBuilder) String() string       { return b.sb.String() }

func (b *stringsBuilder) endsWithNewline() bool {
	s := b.sb.String()
	return len(s) > 0 && s[len(s)-1] == '\n'
}
func (b *stringsBuilder) endsWithTwoNewlines() bool {
	s := b.sb.String()
	return len(s) > 1 && s[len(s)-1] == '\n' && s[len(s)-2] == '\n'
}
func (b *stringsBuilder) TrimLastByte() {
	s := b.sb.String()
	if len(s) == 0 {
		return
	}
	var nb strings.Builder
	nb.Grow(len(s) - 1)
	nb.WriteString(s[:len(s)-1])
	b.sb = nb
}
