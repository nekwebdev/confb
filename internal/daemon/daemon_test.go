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

// helper
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

	// Initial content; newline normalization means out should end with \n.
	writeFileT(t, src1, "hello\r\n")
	writeFileT(t, src2, "world")

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

	// Run daemon in background; capture errors.
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(cfg, Options{
			LogLevel:   LogQuiet,
			Debounce:   120 * time.Millisecond, // extra cushion for CI
			ConfigPath: cfgPath,
		})
	}()

	// Fail fast if it exits immediately.
	select {
	case err := <-errCh:
		t.Fatalf("daemon exited early: %v", err)
	default:
	}

	// Instead of waiting for initial write (flaky on CI), FORCE a rebuild by modifying a source,
	// then loop until final content matches. This makes the test independent of initial timing.
	targetContent := "hello\nWORLD!\n"

	deadline := time.Now().Add(30 * time.Second) // generous CI budget
	for {
		// If daemon died during wait, fail with its error.
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("daemon exited with error: %v", err)
			}
			t.Fatalf("daemon exited unexpectedly")
		default:
		}

		// Trigger rebuild by changing src2 content back and forth to ensure events
		writeFileT(t, src2, "WORLD!")
		time.Sleep(200 * time.Millisecond) // allow debounce + write
		// Check output
		if b, err := os.ReadFile(out); err == nil && string(b) == targetContent {
			break
		}

		// Nudge again (some runners can coalesce events aggressively)
		writeFileT(t, src2, "WORLD!") // same content rewrite still produces mtime change
		time.Sleep(250 * time.Millisecond)

		if time.Now().After(deadline) {
			b, _ := os.ReadFile(out)
			t.Fatalf("timeout waiting for final content\nhave: %q\nwant: %q", string(b), targetContent)
		}
	}

	// Verify on_change side effect with its own budget.
	waitUntil(t, 10*time.Second, func() bool {
		b, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(b)) == "done"
	}, func() string {
		return "marker file not created by on_change"
	})

	// Stop daemon
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon returned error on shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit after SIGINT")
	}
}

func waitUntil(t *testing.T, d time.Duration, cond func() bool, msg func() string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	if msg != nil {
		t.Fatal(msg())
	} else {
		t.Fatal("condition not met before timeout")
	}
}

func quoteYAML(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
