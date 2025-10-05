package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

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
		lastSum  string
		watchSet map[string]struct{}
	}
	states := make([]*tstate, 0, len(cfg.Targets))

	// initial plan + initial write (normalized output)
	for i := range cfg.Targets {
		t := cfg.Targets[i]
		rt, err := plan.PlanTarget(cfg, t, "")
		if err != nil {
			return err
		}
		sum, err := executor.SHA256OfFiles(rt.Files)
		if err != nil {
			return err
		}

		logf(LogNormal, "confb(run): building %q...\n", rt.Name)
		if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
			return err
		}
		logf(LogNormal, "confb(run): wrote %s\n", rt.Output)

		ws, err := computeWatchDirs(cfg, t)
		if err != nil {
			return err
		}
		if opts.LogLevel >= LogVerbose {
			logf(LogVerbose, "confb(run): watch %q dirs:\n", rt.Name)
			for d := range ws {
				logf(LogVerbose, "  - %s\n", d)
			}
		}

		states = append(states, &tstate{
			target:   t,
			lastSum:  sum,
			watchSet: ws,
		})
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// gather all directories
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

	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	var mu sync.Mutex
	pending := make([]bool, len(states))
	timers := make([]*time.Timer, len(states))

	// dir → target indices
	dirToTargets := map[string][]int{}
	for i, st := range states {
		for d := range st.watchSet {
			dirToTargets[d] = append(dirToTargets[d], i)
		}
	}

	flush := func(idx int) {
		st := states[idx]
		rt, err := plan.PlanTarget(cfg, st.target, "")
		if err != nil {
			logf(LogNormal, "confb(run): plan error %q: %v\n", st.target.Name, err)
			return
		}
		sum, err := executor.SHA256OfFiles(rt.Files)
		if err != nil {
			logf(LogNormal, "confb(run): checksum error %q: %v\n", st.target.Name, err)
			return
		}
		if sum == st.lastSum {
			logf(LogVerbose, "confb(run): %q unchanged (sha=%s)\n", st.target.Name, sum)
			return
		}
		logf(LogNormal, "confb(run): %q changed, rebuilding...\n", st.target.Name)
		if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
			logf(LogNormal, "confb(run): write error %q: %v\n", st.target.Name, err)
			return
		}
		st.lastSum = sum
		logf(LogNormal, "confb(run): wrote %s\n", rt.Output)
	}

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
					pending[idx] = true
					i := idx
					timers[i] = time.AfterFunc(opts.Debounce, func() {
						mu.Lock()
						pending[i] = false
						mu.Unlock()
						flush(i)
					})
					mu.Unlock()
				}
			}
		}
	}()

	s := <-sigc
	logf(LogNormal, "confb(run): received %v, exiting\n", s)
	cancel()
	return nil
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
