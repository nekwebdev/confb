package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/nekwebdev/confb/internal/config"
	"github.com/nekwebdev/confb/internal/daemon"
)

func newRunCmd() *cobra.Command {
	var trace bool
	var debounceMS int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run confb as a daemon: watch sources and rebuild targets on change",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Root().Flags().GetString("config")
			chdir, _ := cmd.Root().Flags().GetString("chdir")

			if chdir != "" {
				if err := os.Chdir(chdir); err != nil {
					return fmt.Errorf("failed to chdir to %q: %w", chdir, err)
				}
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if trace {
				base, err := cfg.BaseDir()
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "confb(run): baseDir = %s\n", base)
				absCfg, _ := filepath.Abs(cfgPath)
				fmt.Fprintf(os.Stderr, "confb(run): config = %s\n", absCfg)
			}

			opts := daemon.Options{
				Trace:    trace,
				Debounce: time.Duration(debounceMS) * time.Millisecond,
			}
			return daemon.Run(cfg, opts)
		},
	}

	cmd.Flags().BoolVar(&trace, "trace", false, "print watch set and rebuild decisions")
	cmd.Flags().IntVar(&debounceMS, "debounce-ms", 200, "debounce window in milliseconds")

	return cmd
}
