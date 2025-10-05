package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nekwebdev/confb/internal/config"
)

func writeFileT(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func TestRun_RawConcat_RebuildAndOnChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals differ on Windows; skip daemon E2E")
	}

	td := t.TempDir()
	src1 := filepath.Join(td, "src", "a.txt")
	src2 := filepath.Join(td, "src", "b.txt")
	out := filepath.Join(td, "out.txt")
	marker := filepath.Join(td, "marker.txt")

	writeFileT(t, src1, "hello\r\n")
	writeFileT(t, src2, "world") // no trailing newline initially

	cfgPath := filepath.Join(td, "confb.yaml")
	writeFileT(t, cfgPath, `
version: 1
targets:
  - name: raw
    format: raw
    output: `+quoteYAML(out)+`
    sources:
      - path: `+quoteYAML(src1)+`
      - path: `+quoteYAML(src2)+`
    on_change: |
      /bin/sh -lc 'echo done > `+marker+`'
`)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// run daemon in background
	done := make(chan struct{})
	go func() {
		_ = Run(cfg, Options{LogLevel: LogQuiet, Debounce: 30 * time.Millisecond})
		close(done)
	}()

	// wait for initial write
	waitFor(t, 2*time.Second, func() bool {
		b, err := os.ReadFile(out)
		return err == nil && string(b) == "hello\nworld\n"
	})

	// modify a source to trigger rebuild
	writeFileT(t, src2, "WORLD!") // content change
	waitFor(t, 2*time.Second, func() bool {
		b, err := os.ReadFile(out)
		return err == nil && string(b) == "hello\nWORLD!\n"
	})

	// on_change marker should exist by now
	waitFor(t, 2*time.Second, func() bool {
		b, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(b)) == "done"
	})

	// stop daemon by sending SIGINT to this process (daemon listens globally)
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not exit after SIGINT")
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func quoteYAML(s string) string {
	// naive single-quote wrapper (escape any single quote by doubling it)
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
