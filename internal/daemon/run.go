package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/nekwebdev/confb/internal/blend"
	"github.com/nekwebdev/confb/internal/config"
	executor "github.com/nekwebdev/confb/internal/exec"
	"github.com/nekwebdev/confb/internal/plan"
)

// log levels
type LogLevel int

const (
	LogQuiet LogLevel = iota
	LogNormal
	LogVerbose
)

type Options struct {
	LogLevel LogLevel
	Debounce time.Duration
}

func Run(cfg *config.Config, opts Options) error {
	if opts.Debounce <= 0 {
		opts.Debounce = 200 * time.Millisecond
	}

	// logging helper with timestamp
	logf := func(level LogLevel, format string, args ...any) {
		if opts.LogLevel >= level {
			ts := time.Now().Format("2006-01-02 15:04:05")
			msg := fmt.Sprintf(format, args...)
			fmt.Fprintf(os.Stderr, "[%s] %s", ts, msg)
		}
	}

	type tstate struct {
		target   config.Target
		lastSum  string              // SHA256 hex of *final output content*
		watchSet map[string]struct{} // dirs to watch
	}
	states := make([]*tstate, 0, len(cfg.Targets))

	// initial plan + initial (conditional) write
	for i := range cfg.Targets {
		t := cfg.Targets[i]

		rt, err := plan.PlanTarget(cfg, t, "")
		if err != nil {
			return err
		}

		// build merged (or concatenated) content + checksum, and write if needed
		content, checksum, merged, err := buildContentAndChecksum(t, rt.Files)
		if err != nil {
			return fmt.Errorf("initial build %q: %w", t.Name, err)
		}

		// Write output if we have merged content (we control the bytes),
		// otherwise for concat path we use BuildAndWrite to construct content identically.
		if merged {
			if err := executor.WriteAtomic(rt.Output, content); err != nil {
				return err
			}
			logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
		} else {
			// concat path: BuildAndWrite internally re-generates exactly what we hashed
			if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
				return err
			}
			logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
		}

		// compute and record watch set
		ws, err := computeWatchDirs(cfg, t)
		if err != nil {
			return err
		}
		if opts.LogLevel >= LogVerbose {
			logf(LogVerbose, "confb(run): watch %q dirs:\n", t.Name)
			for d := range ws {
				logf(LogVerbose, "  - %s\n", d)
			}
		}

		states = append(states, &tstate{
			target:   t,
			lastSum:  checksum,
			watchSet: ws,
		})
	}

	// watcher covering all source dirs
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	global := map[string]struct{}{}
	for _, st := range states {
		for d := range st.watchSet {
			global[d] = struct{}{}
		}
	}
	for d := range global {
		_ = os.MkdirAll(d, 0o755)
		if err := w.Add(d); err != nil {
			return fmt.Errorf("watch add %q: %w", d, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// signals
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	// debounce machinery
	var mu sync.Mutex
	timers := make([]*time.Timer, len(states))

	// dir -> target indices
	dirToTargets := map[string][]int{}
	for i, st := range states {
		for d := range st.watchSet {
			dirToTargets[d] = append(dirToTargets[d], i)
		}
	}

	// rebuild one target if output content changed
	flush := func(idx int) {
		st := states[idx]
		t := st.target

		rt, err := plan.PlanTarget(cfg, t, "")
		if err != nil {
			logf(LogNormal, "confb(run): plan error %q: %v\n", t.Name, err)
			return
		}

		content, checksum, merged, err := buildContentAndChecksum(t, rt.Files)
		if err != nil {
			logf(LogNormal, "confb(run): build error %q: %v\n", t.Name, err)
			return
		}

		if checksum == st.lastSum {
			logf(LogVerbose, "confb(run): %q unchanged (sha=%s)\n", t.Name, checksum)
			return
		}

		logf(LogNormal, "confb(run): %q changed, rebuilding...\n", t.Name)
		if merged {
			if err := executor.WriteAtomic(rt.Output, content); err != nil {
				logf(LogNormal, "confb(run): write error %q: %v\n", t.Name, err)
				return
			}
		} else {
			if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
				logf(LogNormal, "confb(run): write error %q: %v\n", t.Name, err)
				return
			}
		}
		st.lastSum = checksum
		logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
	}

	// event loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-w.Errors:
				logf(LogNormal, "confb(run): watcher error: %v\n", err)
			case ev := <-w.Events:
				evDir := filepath.Dir(ev.Name)
				indices := dirToTargets[evDir]
				logf(LogVerbose, "confb(run): fs %s %s -> %d target(s)\n",
					ev.Op.String(), ev.Name, len(indices))
				for _, idx := range indices {
					mu.Lock()
					if timers[idx] != nil {
						timers[idx].Stop()
					}
					i := idx
					timers[i] = time.AfterFunc(opts.Debounce, func() {
						mu.Lock()
						mu.Unlock()
						flush(i)
					})
					mu.Unlock()
				}
			}
		}
	}()

	// wait for signal
	s := <-sigc
	logf(LogNormal, "confb(run): received %v, exiting\n", s)
	cancel()
	return nil
}

// buildContentAndChecksum builds the final output content (for merged formats),
// or computes the normalized concatenation checksum (for concat path).
// Returns (content, checksumHex, merged, error).
func buildContentAndChecksum(t config.Target, files []string) (string, string, bool, error) {
	format := strings.ToLower(t.Format)

	// Merge path?
	if t.Merge != nil && (format == "yaml" || format == "json" || format == "toml" || format == "kdl" || format == "ini") {
		var (
			content string
			err     error
		)
		switch format {
		case "yaml", "json", "toml":
			content, err = blend.BlendStructured(format, t.Merge.Rules, files)
		case "kdl":
			content, err = blend.BlendKDL(t.Merge.Rules, files)
		case "ini":
			content, err = blend.BlendINI(t.Merge.Rules, files)
		}
		if err != nil {
			return "", "", false, err
		}
		sum := sha256Hex(content)
		return content, sum, true, nil
	}

	// Concat path (no merge rules for this format/target)
	sum, err := executor.SHA256OfFiles(files)
	if err != nil {
		return "", "", false, err
	}
	return "", sum, false, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// computeWatchDirs: watch the dir of each explicit file and the dir part of each glob.
func computeWatchDirs(cfg *config.Config, t config.Target) (map[string]struct{}, error) {
	baseDir, err := cfg.BaseDir()
	if err != nil {
		return nil, err
	}
	out := map[string]struct{}{}
	for _, s := range t.Sources {
		p := expandTilde(s.Path)
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		out[filepath.Dir(p)] = struct{}{}
	}
	return out, nil
}

func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}
