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
	var quiet bool
	var verbose bool
	var debounceMS int
	var color bool

	cmd := &cobra.Command{
		Use:   "run",
  	Short: "Run the daemon: watch files and rebuild on change",
  	Long: `Run starts a long-lived watcher:
  	- debounced rebuilds
  	- SIGHUP reload of the main config
  	- per-target on_change hooks after writes

	Use --quiet or --verbose to control logs.`,
  	Example: `  confb run -c ~/.config/confb/confb.yaml --verbose
  	# reload config live
  	pkill -HUP confb`,	
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
			}
			if verbose {
				level = daemon.LogVerbose
			}

			absCfg := cfgPath
			if !filepath.IsAbs(absCfg) {
				if abs, err := filepath.Abs(absCfg); err == nil {
					absCfg = abs
				}
			}

			opts := daemon.Options{
				LogLevel:   level,
				Debounce:   msToDuration(debounceMS),
				ConfigPath: absCfg,
				Color:      color,
			}

			return daemon.Run(cfg, opts)
		},
	}

	cmd.Flags().BoolVar(&quiet, "quiet", false, "reduce log output")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "increase log output (debug)")
	cmd.Flags().IntVar(&debounceMS, "debounce-ms", 200, "debounce interval for rebuilds (milliseconds)")
	cmd.Flags().BoolVar(&color, "color", false, "enable ANSI color for log level tags")

	return cmd
}

func msToDuration(ms int) time.Duration {
	if ms <= 0 {
		return 200 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}
