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

	errCh := make(chan error, 1)
	go func() {
		// Give CI a little extra debounce margin
		errCh <- Run(cfg, Options{
			LogLevel:   LogQuiet,
			Debounce:   80 * time.Millisecond,
			ConfigPath: cfgPath,
		})
	}()

	// Fail fast if daemon exits immediately with an error
	select {
	case err := <-errCh:
		t.Fatalf("daemon exited early: %v", err)
	default:
	}

	// Wait up to 15s for initial output to exist and be non-empty
	waitFor(t, 15*time.Second, func() bool {
		fi, err := os.Stat(out)
		return err == nil && fi.Size() > 0
	})

	// Now modify a source and assert strict final content after rebuild (up to 15s)
	writeFileT(t, src2, "WORLD!")
	waitFor(t, 15*time.Second, func() bool {
		b, err := os.ReadFile(out)
		return err == nil && string(b) == "hello\nWORLD!\n"
	})

	// on_change marker should exist by now (up to 10s)
	waitFor(t, 10*time.Second, func() bool {
		b, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(b)) == "done"
	})

	// stop daemon
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon returned error on shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
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
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func quoteYAML(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
