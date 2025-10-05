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

type Options struct {
	Trace    bool
	Debounce time.Duration
}

// Run watches all source directories and rebuilds targets when content changes.
// It exits cleanly on SIGINT/SIGTERM.
func Run(cfg *config.Config, opts Options) error {
	if opts.Debounce <= 0 {
		opts.Debounce = 200 * time.Millisecond
	}

	// per-target state
	type tstate struct {
		target   config.Target
		lastSum  string
		watchSet map[string]struct{}
	}
	states := make([]*tstate, 0, len(cfg.Targets))

	// initial plan + initial write (normalized output) + baseline checksum
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
		if opts.Trace {
			fmt.Fprintf(os.Stderr, "confb(run): initial %q sha=%s\n", rt.Name, sum)
		}
		if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
			return err
		}
		if opts.Trace {
			fmt.Fprintf(os.Stderr, "confb(run): wrote %s\n", rt.Output)
		}

		ws, err := computeWatchDirs(cfg, t)
		if err != nil {
			return err
		}
		if opts.Trace {
			fmt.Fprintf(os.Stderr, "confb(run): watch %q dirs:\n", rt.Name)
			for d := range ws {
				fmt.Fprintf(os.Stderr, "  - %s\n", d)
			}
		}

		states = append(states, &tstate{
			target:   t,
			lastSum:  sum,
			watchSet: ws,
		})
	}

	// one watcher, many dirs
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	// add unique dirs
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

	// signal handling
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	// debounce bookkeeping
	var mu sync.Mutex
	pending := make([]bool, len(states))
	timers := make([]*time.Timer, len(states))

	// map dir -> target indices
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
			fmt.Fprintf(os.Stderr, "confb(run): plan error %q: %v\n", st.target.Name, err)
			return
		}
		sum, err := executor.SHA256OfFiles(rt.Files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "confb(run): checksum error %q: %v\n", st.target.Name, err)
			return
		}
		if sum == st.lastSum {
			if opts.Trace {
				fmt.Fprintf(os.Stderr, "confb(run): %q unchanged (sha=%s)\n", st.target.Name, sum)
			}
			return
		}
		if opts.Trace {
			fmt.Fprintf(os.Stderr, "confb(run): %q changed (old=%s new=%s)\n", st.target.Name, st.lastSum, sum)
		}
		if err := executor.BuildAndWrite(rt.Output, rt.Files); err != nil {
			fmt.Fprintf(os.Stderr, "confb(run): write error %q: %v\n", st.target.Name, err)
			return
		}
		st.lastSum = sum
		if opts.Trace {
			fmt.Fprintf(os.Stderr, "confb(run): wrote %s\n", rt.Output)
		}
	}

	// event loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-w.Errors:
				fmt.Fprintf(os.Stderr, "confb(run): watcher error: %v\n", err)
			case ev := <-w.Events:
				evDir := filepath.Dir(ev.Name)
				indices := dirToTargets[evDir]
				if opts.Trace {
					fmt.Fprintf(os.Stderr, "confb(run): fs %s %s -> %d target(s)\n", ev.Op.String(), ev.Name, len(indices))
				}
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

	// block until signal
	s := <-sigc
	if opts.Trace {
		fmt.Fprintf(os.Stderr, "confb(run): signal %v, exiting\n", s)
	}
	cancel()
	return nil
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
