package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
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
	LogLevel   LogLevel
	Debounce   time.Duration
	ConfigPath string // ABS or relative; used for SIGHUP reload
}

type tstate struct {
	target   config.Target
	lastSum  string              // SHA256 hex of *final output content*
	watchSet map[string]struct{} // dirs to watch
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

	// ---- helper closures ----

	buildStates := func(c *config.Config) ([]*tstate, error) {
		states := make([]*tstate, 0, len(c.Targets))
		for i := range c.Targets {
			t := c.Targets[i]

			rt, err := plan.PlanTarget(c, t, "")
			if err != nil {
				return nil, err
			}

			content, checksum, merged, err := buildContentAndChecksum(t, rt.Files)
			if err != nil {
				return nil, fmt.Errorf("initial build %q: %w", t.Name, err)
			}

			if merged {
				if err := executor.WriteAtomic(rt.Output, content); err != nil {
					return nil, err
				}
				logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
			} else {
				if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
					return nil, err
				}
				logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
			}

			if strings.TrimSpace(t.OnChange) != "" {
				runOnChange(t, rt.Output, logf, opts.LogLevel)
			}

			ws, err := computeWatchDirs(c, t)
			if err != nil {
				return nil, err
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
		return states, nil
	}

	buildWatcher := func(states []*tstate) (*fsnotify.Watcher, map[string][]int, error) {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, nil, err
		}
		// dir -> indices
		dirToTargets := map[string][]int{}
		global := map[string]struct{}{}
		for i, st := range states {
			for d := range st.watchSet {
				global[d] = struct{}{}
				dirToTargets[d] = append(dirToTargets[d], i)
			}
		}
		for d := range global {
			_ = os.MkdirAll(d, 0o755)
			if err := w.Add(d); err != nil {
				_ = w.Close()
				return nil, nil, fmt.Errorf("watch add %q: %w", d, err)
			}
		}
		return w, dirToTargets, nil
	}

	reloadConfig := func() (*config.Config, error) {
		if strings.TrimSpace(opts.ConfigPath) == "" {
			return nil, fmt.Errorf("SIGHUP reload requested but Options.ConfigPath is empty")
		}
		cfgPath := opts.ConfigPath
		// No chdir juggling here; Run should be invoked after CLI chdir is applied.
		logf(LogNormal, "confb(run): reloading config from %s\n", cfgPath)
		newCfg, err := config.Load(cfgPath)
		if err != nil {
			return nil, err
		}
		return newCfg, nil
	}

	// ---- initial build & watcher ----
	states, err := buildStates(cfg)
	if err != nil {
		return err
	}
	w, dirToTargets, err := buildWatcher(states)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// signals: INT/TERM for exit; HUP for reload
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// debounce machinery
	var mu sync.Mutex
	timers := make([]*time.Timer, len(states))

	// helpers tied to current states
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

		if strings.TrimSpace(t.OnChange) != "" {
			runOnChange(t, rt.Output, logf, opts.LogLevel)
		}
	}

	// event loop
	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-w.Errors:
			logf(LogNormal, "confb(run): watcher error: %v\n", err)

		case ev := <-w.Events:
			evDir := filepath.Dir(ev.Name)
			indices := dirToTargets[evDir]
			logf(LogVerbose, "confb(run): fs %s %s -> %d target(s)\n",
				ev.Op.String(), ev.Name, len(indices))
			for _, idx := range indices {
				mu.Lock()
				if idx >= len(timers) {
					mu.Unlock()
					continue
				}
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

		case s := <-sigc:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				logf(LogNormal, "confb(run): received %v, exiting\n", s)
				cancel()
				return nil

			case syscall.SIGHUP:
				// Reload: stop timers, rebuild config, states, watcher & routing
				logf(LogNormal, "confb(run): received SIGHUP, reloading configuration\n")

				// Guard: stop all timers
				mu.Lock()
				for i := range timers {
					if timers[i] != nil {
						timers[i].Stop()
						timers[i] = nil
					}
				}
				mu.Unlock()

				newCfg, err := reloadConfig()
				if err != nil {
					logf(LogNormal, "confb(run): reload error: %v (keeping old config)\n", err)
					continue
				}

				newStates, err := buildStates(newCfg)
				if err != nil {
					logf(LogNormal, "confb(run): reload build error: %v (keeping old config)\n", err)
					continue
				}

				newWatcher, newDirToTargets, err := buildWatcher(newStates)
				if err != nil {
					logf(LogNormal, "confb(run): reload watcher error: %v (keeping old config)\n", err)
					continue
				}

				// Swap in new state atomically
				_ = w.Close()
				w = newWatcher
				dirToTargets = newDirToTargets
				states = newStates
				cfg = newCfg
				timers = make([]*time.Timer, len(states))

				logf(LogNormal, "confb(run): reload complete (%d targets)\n", len(states))
			}
		}
	}
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

// --- on_change hook ---

func runOnChange(t config.Target, outputPath string, logf func(LogLevel, string, ...any), level LogLevel) {
	cmdTmpl := strings.TrimSpace(t.OnChange)
	if cmdTmpl == "" {
		return
	}
	// template vars
	cmdStr := cmdTmpl
	cmdStr = strings.ReplaceAll(cmdStr, "{target}", t.Name)
	cmdStr = strings.ReplaceAll(cmdStr, "{output}", outputPath)
	cmdStr = strings.ReplaceAll(cmdStr, "{timestamp}", time.Now().Format(time.RFC3339))

	// best-effort timeout to avoid wedging the daemon
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	logf(LogNormal, "confb(run): running on_change: %s\n", cmdStr)
	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
	c.Env = append(os.Environ(),
		"CONFB_TARGET="+t.Name,
		"CONFB_OUTPUT="+outputPath,
		"CONFB_TIMESTAMP="+time.Now().Format(time.RFC3339),
	)
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		logf(LogNormal, "confb(run): on_change error: %v\n", err)
	}
}
