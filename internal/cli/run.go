package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nekwebdev/confb/internal/config"
	"github.com/nekwebdev/confb/internal/daemon"
)

func newRunCmd() *cobra.Command {
	var quiet bool
	var verbose bool
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

			level := daemon.LogNormal
			if quiet {
				level = daemon.LogQuiet
			} else if verbose {
				level = daemon.LogVerbose
			}

			opts := daemon.Options{
				LogLevel: level,
				Debounce: time.Duration(debounceMS) * time.Millisecond,
			}
			return daemon.Run(cfg, opts)
		},
	}

	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-error logs")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "show detailed trace logs")
	cmd.Flags().IntVar(&debounceMS, "debounce-ms", 200, "debounce window in milliseconds")

	return cmd
}
